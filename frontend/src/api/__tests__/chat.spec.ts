import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  chatCompletion,
  fetchModelCatalog,
  parseCodexModelsManifest,
  reasoningEffortsForPlatform,
} from '@/api/chat'

describe('chatCompletion', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  it('uses JWT auth and consumes the persisted chat SSE endpoint', async () => {
    localStorage.setItem('auth_token', 'jwt-token')
    const encoder = new TextEncoder()
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode('data: {"type":"response.output_text.delta","delta":"hello "}\n\n'))
        controller.enqueue(encoder.encode('data: {"type":"response.output_text.delta","delta":"world"}\n\n'))
        controller.enqueue(
          encoder.encode(
            'data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed","usage":{"total_tokens":4},"output":[{"type":"message","content":[{"type":"output_text","text":"hello world","annotations":[{"type":"url_citation","url":"https://openai.com/docs","title":"OpenAI Docs"}]}]}]}}\n\n',
          ),
        )
        controller.close()
      },
    })
    const fetchMock = vi.fn().mockResolvedValue(new Response(body, { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    const deltas: string[] = []

    const result = await chatCompletion(10, 20, 'gpt-test', 'high', 'question', 'client-1', [], (delta) => {
      deltas.push(delta)
    })

    expect(result).toEqual({
      content: 'hello world\n\n### Sources\n- [OpenAI Docs](<https://openai.com/docs>)',
      model: 'gpt-test',
      usage: { total_tokens: 4 },
    })
    expect(deltas).toEqual(['hello ', 'world'])
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/chat/conversations/10/completions')
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer jwt-token')
    expect(JSON.parse(String(init.body))).toMatchObject({
      api_key_id: 20,
      reasoning_effort: 'high',
      client_message_id: 'client-1',
      stream: true,
    })
  })

  it('surfaces the gateway error message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: { message: 'insufficient balance' } }), {
          status: 403,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    )

    await expect(chatCompletion(1, 2, 'gpt-test', '', 'question', 'client-2')).rejects.toThrow(
      'insufficient balance',
    )
  })

  it('surfaces errors delivered inside an SSE stream', async () => {
    const encoder = new TextEncoder()
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(
          encoder.encode(
            'data: {"type":"response.failed","response":{"status":"failed","error":{"message":"Unexpected upstream failure"}}}\n\n',
          ),
        )
        controller.close()
      },
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 200 })))

    await expect(chatCompletion(1, 2, 'gpt-test', '', 'question', 'client-3')).rejects.toThrow(
      'Unexpected upstream failure',
    )
  })

  it('consumes Anthropic Messages SSE and merges citations and usage', async () => {
    const encoder = new TextEncoder()
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(
          encoder.encode(
            'data: {"type":"message_start","message":{"id":"msg_1","model":"claude-test","content":[],"usage":{"input_tokens":5}}}\n\n',
          ),
        )
        controller.enqueue(
          encoder.encode(
            'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"researched answer"}}\n\n',
          ),
        )
        controller.enqueue(
          encoder.encode(
            'data: {"type":"content_block_delta","delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","url":"https://docs.anthropic.com/search","title":"Anthropic Docs"}}}\n\n',
          ),
        )
        controller.enqueue(
          encoder.encode(
            'data: {"type":"message_delta","usage":{"input_tokens":0,"output_tokens":3}}\n\n',
          ),
        )
        controller.close()
      },
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status: 200 })))
    const deltas: string[] = []

    const result = await chatCompletion(1, 2, 'claude-test', 'xhigh', 'question', 'client-4', [], (delta) => {
      deltas.push(delta)
    })

    expect(result).toEqual({
      content: 'researched answer\n\n### Sources\n- [Anthropic Docs](<https://docs.anthropic.com/search>)',
      model: 'claude-test',
      usage: { input_tokens: 5, output_tokens: 3 },
    })
    expect(deltas).toEqual(['researched answer'])
  })

  it('uses provider-native reasoning effort labels', () => {
    expect(reasoningEffortsForPlatform('openai', 'gpt-5.6-sol')).toEqual([
      '',
      'none',
      'minimal',
      'low',
      'medium',
      'high',
      'xhigh',
      'max',
    ])
    expect(reasoningEffortsForPlatform('anthropic', 'claude-opus-4-8')).toEqual([
      '',
      'low',
      'medium',
      'high',
      'xhigh',
      'max',
    ])
    expect(reasoningEffortsForPlatform('gemini', 'gemini-3.5-flash')).toEqual([
      '',
      'minimal',
      'low',
      'medium',
      'high',
    ])
  })

  it('uses the OpenAI subscription manifest in catalog order, including ultra', async () => {
    const manifest = {
      models: [
        {
          slug: 'gpt-5.6-sol',
          display_name: 'GPT-5.6 Sol',
          visibility: 'list',
          supported_reasoning_levels: [
            { effort: 'medium', description: 'Balanced' },
            { effort: 'max', description: 'Extended reasoning' },
            { effort: 'ultra', description: 'Maximum reasoning' },
          ],
        },
        { slug: 'hidden-model', visibility: 'hide', supported_reasoning_levels: ['high'] },
      ],
    }
    expect(parseCodexModelsManifest(manifest)).toEqual([
      {
        id: 'gpt-5.6-sol',
        label: 'GPT-5.6 Sol',
        reasoningOptions: [
          { value: 'medium', label: 'medium', description: 'Balanced' },
          { value: 'max', label: 'max', description: 'Extended reasoning' },
          { value: 'ultra', label: 'ultra', description: 'Maximum reasoning' },
        ],
      },
    ])

    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(manifest), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await expect(fetchModelCatalog('subscription-key', 'openai')).resolves.toEqual(
      parseCodexModelsManifest(manifest),
    )
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]?.[0]).toContain('/v1/models?codex_manifest=1')
  })
})
