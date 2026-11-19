# sub2api v0.2.21 合并记录

基线为合入 PR #27 后的 SurplusToken main：
`2853f00466432e2fbb8dc8926859c2173d5efb69`。
合入的上游正式 release 为 [v0.2.21](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.21)，
对应提交 `10ba8f9b4dc2e6023d4a9879ab77b21841babd10`。
本地工作分支为 `merge/upstream-v0.2.21-after-pr27`。

## 兼容规则与冲突处理

- PR #27 的拼车 CPR 周额度、用量汇总、desktop2web SSO 启动页、成员身份门禁及退出/移除后的访问撤销保留。
- 已下线的用户 Chat、账号池、Kasm 和贡献/提现界面及路由保持下线；内部仍有用途的存量字段、计费逻辑和 schema 保留。
- 用量日志同时保留共享报价倍率及新增的 30m/12h 缓存写入 TTL。
  INSERT 的 65 个参数、SQL 占位符、SELECT/Scan 顺序及请求类型测试一并适配。
- 上游图片尺寸分层选号、spillover 和 quarantine fail-open 路径继续传递本地 accountEligibility，
  不因新增选号包装丢失共享账号资格检查。新增三种调度模式、显式尺寸及自动回退的组合回归测试。
- 账号创建保留所有者校验与事务锁，并兼容上游 scoped admin 的事务上下文；
  Grok 账号刷新测试补齐本地共同所有者查询。
- 鉴权缓存同时保留本地共享报价区间和上游 RPM override / privacy 字段，快照版本升至 28，使旧快照失效。
- Gemini 保留本地来源引用、模型声明和计费口径，并合入上游工具参数增量、并行工具图片、
  Vertex 模型接口与思考内容处理。xhigh 按上游规则归一为实际支持的 high，计费测试验证转发请求与收费等级一致。
- 模型广场保留本地性能统计与未归渠道分组，合入分组名称本地化所需的数据字段和处理函数。
- MCP 的内部管理路由适配本地 RegisterUserRoutes 参数，相关测试继续验证权限和路由目录。
- 前端批量编辑把本地 test store:false 与上游图片尺寸档位拆成独立设置区，
  同时保留启用、提交、变更检测和重置；新增一次提交两个设置的回归测试。
- Ent 根据合并后的 schema 重生成，保留 sql/execquery、sql/lock；Wire 根据合并后的 provider 重生成。
  Wire 的 go:generate 明确使用 -mod=mod，避免 tidy 后工具间接依赖缺失阻断生成。
- 锁文件使用 pnpm 9 原生合并并验证冻结安装。
  上游使用的 Go 1.27.3 在最新 govulncheck 中命中标准库漏洞
  [GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859)，因此本次候选补升至 Go 1.27.4，
  同步 go.mod、Dockerfile 与三个 CI 工作流中的版本检查。
- 清理旧功能下线后遗留的四个未使用私有函数，以及只覆盖已删除私有清理函数的两个旧测试。
  DESKTOP2WEB_ADMIN_PASSWORD_FILE 仅取自部署进程环境，允许管理员指定秘密文件路径；
  对该文件读取添加带原因的 G703 注释，不放宽全局检查规则。
- 上游 thinking 预检查的分配测试在全套运行时会计入其他用例后台任务的分配；
  将该测量放在独立测试进程，保持原有小于 16 次的阈值和原始请求体引用断言。

## 数据库迁移

本地原有 316 个 SQL 迁移文件的文件名、内容逐字节保持。
新增上游 242–250 范围的 10 个迁移，两个 249 按完整文件名区分，沿用上游命名。
usage_logs 的 249 索引迁移为非事务 CONCURRENTLY，250 增加缓存 TTL 字段和价卡。
迁移只在测试自动创建的独立 PostgreSQL/Redis 环境验证，不连接生产数据库。

TTL 集成测试覆盖单条、批量、best-effort 三条写入路径，并同时校验共享倍率、
30m/12h token、请求类型和 native_compaction_v2 的数据库读回结果。

## 验证

- Go 1.27.4：`go test -p 2 -tags=unit ./...` 全量单元测试通过。
- Go 1.27.4：`go test -p 2 -tags=integration ./...` 全量集成测试通过；数据库迁移与读写使用测试自动创建的 PostgreSQL/Redis。
- 分配测量最终改用 os.Executable 启动测试进程后，unit / integration 两种构建各单独连续复跑 5 次通过；阈值不变。
- `go build -p 2 -tags=embed ./cmd/server` 通过，生成二进制确认为 Go 1.27.4。
- 后端 golangci-lint v2.14.0：通过，0 issues。
- govulncheck v1.9.0：调用路径中的漏洞数为 0；报告另列 6 个 required module 中未被当前代码调用的漏洞，不等于所有依赖均无公告。
- pnpm 9 冻结锁文件安装、前端 lint、类型检查、国际化完整性检查及生产构建通过。
- 前端全量 396 个测试文件、3,247 项测试通过；新增批量设置组合回归后，单独复跑该测试文件，56 项全部通过。
- Git index 未解决条目为 0，无冲突标记。原有 316 个迁移逐字节一致；上游新增的 250 迁移保留原始文件内容，包括末尾空行，不为格式检查改写迁移校验和。

## 范围

本次只处理源码集成。没有部署、重启生产服务、修改运行配置或迁移生产数据。
没有变更 `/home/ops/ypd/SurplusToken` 工作区。
