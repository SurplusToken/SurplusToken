# SurplusAI 内部 OAuth 算力共享平台计划

## 1. 项目定位

SurplusAI 基于 `Wei-Shaw/sub2api` 的 fork 二次开发，用于内部团队共享 OAuth 订阅账号算力。团队成员通过平台发放的内部 API Key 调用统一网关，平台负责选择可用 OAuth 上游账号、记录用量、控制权限和做审计。

明确不做：

- 不开放外部用户注册。
- 不允许上传或创建上游 API Key 账号。
- 不做订阅售卖、充值、兑换码、优惠码、返利、订单等 SaaS 商业化流程。
- 不向普通用户暴露 OAuth access token / refresh token。

平台内部 API Key 仍保留。这里的 API Key 只用于访问 SurplusAI 网关，不是上游服务商 API Key。

## 2. 当前仓库基线

- 本地仓库：`ypd666/SurplusAI`
- 上游仓库：`Wei-Shaw/sub2api`
- 当前本地 HEAD：`2f70d965bf5b046ad6e9474a77a493bf4fb60801`
- 当前 `origin/main`：`2f70d965bf5b046ad6e9474a77a493bf4fb60801`
- 当前 `upstream/main`：`2f70d965bf5b046ad6e9474a77a493bf4fb60801`

同步策略：

1. 保留 `upstream` remote 指向 `https://github.com/Wei-Shaw/sub2api.git`。
2. 拉取 upstream 时通过本地代理：

```bash
git -c http.proxy=http://127.0.0.1:7890 -c https.proxy=http://127.0.0.1:7890 fetch upstream
```

3. 后续二次开发在 SurplusAI fork 上提交，定期从 upstream 合并安全修复和网关能力更新。

## 3. 产品约束

### 3.1 OAuth-only 上游账号

SurplusAI 只允许进入共享调度池的账号类型为 `oauth`。

必须拒绝：

- `apikey`
- `setup-token`
- `bedrock`
- `service_account`
- `upstream`

前端隐藏入口只是体验优化，真正边界必须在后端：

```text
if upstream_account.type != "oauth":
    reject
```

### 3.2 内部团队使用

默认关闭：

- 公开注册
- 优惠码
- 兑换码入口
- 支付入口
- 订阅售卖入口
- 返利入口
- 订单入口

用户来源建议：

- 管理员手动创建用户
- 企业邮箱白名单
- 后续接入内部 SSO / OIDC

### 3.3 标准模式部署

推荐使用 `RUN_MODE=standard`，保留完整用户、分组、Key、额度、审计、用量统计能力，同时通过配置和代码关闭商业化功能。

不建议把 upstream 的 simple mode 当作最终形态，因为 simple mode 会跳过部分计费/余额/用户侧能力，不利于团队配额和审计。

## 4. 目标架构

```text
Client
  -> SurplusAI Internal API Key Auth
  -> User / Group Permission Check
  -> User Quota / Rate Limit Check
  -> Request Normalizer
  -> OAuth Account Scheduler
  -> Provider Adapter
  -> Upstream OAuth Provider
  -> Response Parser
  -> Usage Recorder
  -> Audit Log
  -> Client
```

## 5. 已完成的改造范围

### 5.1 后端策略边界

已增加 SurplusAI 账号类型策略：

- `Account.IsSurplusAISchedulableType()`
- `isSurplusAIUpstreamAccountTypeAllowed(...)`
- `filterSurplusAISchedulableAccounts(...)`

已在管理服务中拦截：

- 创建非 OAuth 上游账号
- 把 OAuth 账号更新成非 OAuth 类型
- 编辑历史非 OAuth 账号使其继续参与共享
- 批量启用非 OAuth 账号调度
- 单独启用非 OAuth 账号调度

已在网关调度中拦截：

- Anthropic / 多平台网关账号选择
- OpenAI 网关账号选择
- Gemini Messages 兼容网关账号选择
- 可用模型聚合
- 分组容量统计
- scheduler snapshot 读取后的二次过滤

### 5.2 CRS 同步策略

CRS 导入保留 OAuth 账号同步。

非 OAuth 账号统一跳过：

- Claude Console API Key
- OpenAI Responses API Key
- Gemini API Key
- Claude `setup-token`

CRS 预览也只展示 OAuth 账号，避免管理员在同步选择界面看到不可导入账号。

### 5.3 前端入口收敛

账号创建弹窗已调整为 OAuth-only：

- 隐藏 API Key / Bedrock / Vertex service account / upstream 账号类型入口
- Anthropic 强制走 OAuth
- OpenAI 强制走 OAuth
- Gemini 强制走 OAuth
- Antigravity 强制走 OAuth
- 提交前再次强制 `form.type = "oauth"`
- `createAccountAndFinish(...)` 对非 OAuth 类型做最后一道前端拒绝

账号列表类型筛选已收敛为：

- 全部
- OAuth

侧边栏已隐藏内部版不需要的商业化入口：

- 用户订阅
- 购买订阅
- 订单
- 兑换
- 返利
- 管理端订阅
- 管理端兑换码
- 管理端优惠码
- 管理端返利记录
- 管理端订单和套餐

路由守卫已增加硬限制，直接输入相关 URL 也会重定向到 dashboard。

### 5.4 默认配置

新安装实例默认：

- `registration_enabled = false`
- `promo_code_enabled = false`
- `site_name = "SurplusAI"`
- `site_subtitle = "Internal OAuth Account Sharing Platform"`

部署样例已更新：

- `deploy/.env.example`
- `deploy/config.example.yaml`

推荐保留：

- `RUN_MODE=standard`
- PostgreSQL
- Redis
- 管理员首登后手动创建内部用户

## 6. 上线前检查清单

### 6.1 必填环境变量

- `SERVER_MODE=release`
- `RUN_MODE=standard`
- `POSTGRES_PASSWORD=<强密码>`
- `JWT_SECRET=<强随机值>`
- `ENCRYPTION_KEY=<强随机值>`
- `FRONTEND_URL=https://your-domain`
- `API_BASE_URL=https://your-domain`
- 各 provider OAuth client 配置

### 6.2 账号和权限

- 创建管理员账号。
- 关闭公开注册。
- 为内部成员创建用户。
- 为用户分配分组。
- 为用户生成平台内部 API Key。
- 为分组配置模型、并发、RPM/TPM、额度策略。

### 6.3 OAuth 上游账号

- 仅通过 OAuth 添加上游账号。
- 确认账号类型为 `oauth`。
- 确认 token 自动刷新可用。
- 确认账号可调度。
- 确认账号绑定到正确分组。
- 确认历史非 OAuth 账号不会被启用调度。

### 6.4 安全

- 生产环境启用 HTTPS。
- 配置可信反向代理。
- 限制管理后台访问范围。
- 禁止日志输出 token、cookie、上游 API key。
- 数据库备份加密。
- Redis 不暴露公网。

## 7. 验证记录

已通过：

```bash
gofmt -w <changed go files>
go test -tags unit ./internal/...
HTTP_PROXY=http://127.0.0.1:7890 HTTPS_PROXY=http://127.0.0.1:7890 go test -tags unit ./...
corepack pnpm@10.23.0 install --frozen-lockfile
corepack pnpm@10.23.0 run typecheck
corepack pnpm@10.23.0 run build
CGO_ENABLED=0 go build -tags embed -ldflags "-s -w -X main.BuildType=release" -o bin/server-surplusai.exe ./cmd/server
bin/server-surplusai.exe -version
docker build -t surplusai:local \
  --build-arg NODE_IMAGE=m.daocloud.io/docker.io/library/node:24-alpine \
  --build-arg GOLANG_IMAGE=m.daocloud.io/docker.io/library/golang:1.26.3-alpine \
  --build-arg ALPINE_IMAGE=m.daocloud.io/docker.io/library/alpine:3.20 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -f deploy/Dockerfile .
docker run --rm --entrypoint /app/sub2api surplusai:local -version
```

前端生产构建产物输出到：

```text
backend/internal/web/dist
```

Docker 说明：

- `deploy/Dockerfile` 已验证可构建镜像 `surplusai:local`。
- 本机 Docker Hub 直连不可用，构建时通过 build args 使用 DaoCloud 基础镜像源。
- `deploy/Dockerfile` 已将 pnpm 固定为 `10.23.0`，避免 `pnpm@latest` 与现有 lockfile 的 overrides 行为不兼容。
- `deploy/docker-compose.yml` 当前是发布部署配置，只声明 `image:`，没有 `build:`；本地镜像构建应直接执行 `docker build -f deploy/Dockerfile .` 或在 CI 中调用同等命令。

## 8. 后续迭代

优先级 P0：

- 在 CI 中加入 Go test、frontend typecheck、frontend build。
- 加入 OAuth-only 策略的集成测试。
- 加入后台设置项，明确显示“SurplusAI 内部 OAuth-only 模式已启用”。

优先级 P1：

- 贡献者维度统计：账号归属人、贡献额度、被使用次数、成功率。
- 内部成员可见的“贡献账号使用情况”页面。
- 账号健康度和异常自动降级。

优先级 P2：

- 内部 SSO / OIDC 登录。
- 更细粒度的模型授权。
- 分组级成本归因报表。
