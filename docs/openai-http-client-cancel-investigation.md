# OpenAI Responses HTTP client-cancel investigation

Date: 2026-08-11

Base revision: `27ad20834b0a4f1c7096f81c9da98c451812e067`

Related issue: `SurplusToken/SurplusToken#18`

## Scope

This document separates locally reproducible facts from hypotheses about the
August 10 latency increase. It covers native OpenAI `/responses` HTTP forwarding
for a fixed account. It does not claim that account scheduling selected a bad
account.

## Confirmed findings

### The two August 10 commits

`27ad20834` only changes the account-pool frontend and its frontend test. It
cannot change request forwarding, TTFT, HTTP cancellation, or connection-pool
behavior.

`c35b5d7fd` merges upstream v0.1.173. Its directly relevant OpenAI HTTP changes
remove `OpenAI-Beta: responses=experimental` and add
`x-codex-routing-hint`. The same merge adds account scheduling-threshold work,
but that path has a bounded five-second database timeout and cannot by itself
explain a 73-second or 740-second wait.

Neither August 10 commit introduced the cancellation defect below.
`detachUpstreamContext` was already present before the update; its current file
history reaches at least `084d26cbd` from 2026-07-07, which was itself a code
move. The August 10 merge may have exposed the old defect by changing upstream
tail latency, but local static evidence cannot prove which upstream header or
routing behavior triggered the tail.

### The relay detached native Responses requests from client cancellation

Before this patch, native OpenAI HTTP `Forward` called
`detachUpstreamContext(ctx)` immediately before constructing the real upstream
request. That function returns `context.WithoutCancel(ctx)`. Consequently,
closing or canceling a Codex CLI request did not cancel the corresponding
upstream HTTP request.

This behavior conflicts with the existing transport-error policy: the code
already treats `context.Canceled` as "client gone", returns without account
failover, and does not evict the account. The error policy was ready to handle
client cancellation, but the request context prevented the cancellation from
reaching it.

### Logical concurrency and physical connections could diverge

The handler releases the logical concurrency slot with `context.AfterFunc` when
the downstream context is canceled. In account/account-proxy isolation mode,
the HTTP transport independently sets `MaxConnsPerHost` to the account
concurrency.

For a fixed account with concurrency `1`, the old sequence was deterministic:

1. Request A acquired the logical slot and one physical HTTP connection.
2. The client canceled A while the upstream had not returned response headers.
3. The handler released A's logical slot.
4. A's detached upstream request kept the only physical connection occupied.
5. Request B acquired the now-free logical slot but waited inside
   `net/http.Transport` for a physical connection.

This is a connection-pool convoy, not a logical concurrency-slot leak and not
evidence that the scheduler selected another account.

### Default OpenAI watchdogs do not bound the wait

The default values of all three relevant OpenAI limits are zero:

```text
gateway.openai_response_header_timeout = 0
gateway.openai_first_output_timeout_seconds = 0
gateway.openai_high_effort_first_output_timeout_seconds = 0
```

Therefore, the default native OpenAI HTTP path has no relay-side response-header
or first-semantic-output deadline. This patch deliberately does not change those
defaults because the existing first-output timeout can trigger account failover
and duplicate upstream billing. Enabling failover is a separate operator/product
decision, not part of this cancellation fix.

## Deterministic local reproduction

`TestOpenAIForwardClientCancelReleasesAccountHTTPConnection` uses the real
`OpenAIGatewayService.Forward`, a real `net/http.Transport` with
`MaxConnsPerHost: 1`, and a local `httptest.Server` that blocks the first request
before response headers. No external network or OpenAI account is involved.

With the original production code and the final test, the exact command fails
every time because the request context remains live after client cancellation:

```text
$ go test ./internal/service \
    -run '^TestOpenAIForwardClientCancelReleasesAccountHTTPConnection$' \
    -count=1 -v
=== RUN   TestOpenAIForwardClientCancelReleasesAccountHTTPConnection
    openai_http_cancel_integration_test.go:105: Condition never satisfied
--- FAIL: TestOpenAIForwardClientCancelReleasesAccountHTTPConnection (0.51s)
```

## Fix

Native OpenAI `/responses` HTTP requests now inherit the caller context. The
optional first-output header guard remains a child context, so both client
cancellation and its configured deadline cancel the real HTTP request. The
guard no longer carries an unused release callback for a detached context.

The patch does not add retries, account switching, or a new timeout. Image and
other background-completion paths that intentionally use detached contexts are
unchanged.

A long decode remains valid while the downstream connection is alive. For
example, a connected client can still wait five minutes for the first output
when the OpenAI timeout settings remain at their zero defaults. If the client or
an intermediate reverse proxy has already canceled the downstream context,
continuing the upstream request cannot deliver a response; the patch now closes
that request and releases its transport connection.

## Verification

The focused end-to-end test passes ten consecutive runs after the fix:

```text
$ go test ./internal/service \
    -run '^TestOpenAIForwardClientCancelReleasesAccountHTTPConnection$' \
    -count=10
ok github.com/Wei-Shaw/sub2api/internal/service 0.158s
```

Related service timeout/cancellation tests pass:

```text
$ go test ./internal/service \
    -run '^(TestOpenAIForwardClientCancelReleasesAccountHTTPConnection|TestOpenAIForwardFirstOutputTimeoutIncludesResponseHeaderWait|TestOpenAINativeFirstOutput.*|TestOpenAIFirstOutput.*|TestDetachUpstreamContextIgnoresClientCancel)$' \
    -count=1
ok github.com/Wei-Shaw/sub2api/internal/service 8.322s
```

The handler-level client-disconnect/failover tests also pass:

```text
$ go test ./internal/handler \
    -run '^TestOpenAIGatewayHandlerResponses_' \
    -count=1
ok github.com/Wei-Shaw/sub2api/internal/handler 0.050s
```

Finally, the complete service and handler packages pass after updating the old
test that explicitly required the detached behavior:

```text
$ go test ./internal/service -count=1
ok github.com/Wei-Shaw/sub2api/internal/service 101.949s

$ go test ./internal/handler -count=1
ok github.com/Wei-Shaw/sub2api/internal/handler 36.691s
```

## Proven boundary

The local reproduction proves that SurplusToken amplified a canceled/stalled
request into per-account connection-pool blocking, and that the patch removes
that amplification. It does not prove why the first upstream request developed
a long tail on August 10. The routing-hint addition and legacy beta-header
removal remain trigger candidates that require upstream request correlation or
an A/B deployment to distinguish.
