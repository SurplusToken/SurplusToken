#!/bin/bash
set -e
source ~/.gvm/scripts/gvm 2>/dev/null; gvm use go1.26.3 >/dev/null 2>&1
cd ~/surplusai-merge/backend
export GOPROXY=https://goproxy.cn,direct GOFLAGS=-mod=mod GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=mod
echo "### go version: $(go version)"
echo "### 1) gofmt 手改文件"
gofmt -w internal/service/account.go internal/repository/account_repo.go internal/handler/dto/mappers.go internal/handler/dto/types.go internal/service/settings_view.go internal/service/openai_gateway_service_codex_snapshot_test.go internal/service/ratelimit_service_openai_test.go 2>&1 | head
echo "### 2) go generate ./ent (重生成 ent)"
go generate ./ent 2>&1 | tail -15
echo "### 3) go generate ./cmd/server (wire)"
go generate ./cmd/server 2>&1 | tail -15
echo "### 4) go build ./..."
go build ./... 2>&1 | tail -40
echo "### BUILD_DONE rc=$?"
