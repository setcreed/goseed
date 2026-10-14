# 依赖与工具核验

运行库维护核验日期：2026-10-10；YAML parser 补充核验日期：2026-10-11。版本清单以 [dependencies.lock.json](../dependencies.lock.json) 为准；原始官方仓库响应快照见 [dependency-audit.json](dependency-audit.json) 。固定直接依赖、间接依赖与校验和保存在主模块、兼容性模块及独立业务模板的依赖清单中。

D10 观测依赖于 2026-10-14 通过 OpenTelemetry 官方 Go 文档、GitHub 发布页、固定标签模块清单及仓库 issue 核验，固定 `go.opentelemetry.io/otel` API/SDK 与 OTLP metric HTTP、trace HTTP exporter 为 v1.47.0；生成项目的本地 OTLP 接收器测试使用官方生成的 `go.opentelemetry.io/proto/otlp` v1.11.0 消息类型。上游 Go v1.47.0 于 2026-10-02 发布，许可证 Apache-2.0；固定模块 go.mod 要求 Go 1.26.0，与项目 Go 1.26.4 一致。OTLP protobuf v1.11.0 的官方发布说明为生成代码更新，协议仓库将 OTLP 二进制 protobuf 定义列为稳定；证据见 [Go SDK v1.47.0 release](https://github.com/open-telemetry/opentelemetry-go/releases/tag/v1.47.0)、[固定 API go.mod](https://github.com/open-telemetry/opentelemetry-go/blob/v1.47.0/go.mod)、[trace HTTP exporter go.mod](https://raw.githubusercontent.com/open-telemetry/opentelemetry-go/v1.47.0/exporters/otlp/otlptrace/otlptracehttp/go.mod)、[metric HTTP exporter go.mod](https://raw.githubusercontent.com/open-telemetry/opentelemetry-go/v1.47.0/exporters/otlp/otlpmetric/otlpmetrichttp/go.mod)、[OTLP protobuf Go v1.11.0 release](https://github.com/open-telemetry/opentelemetry-proto-go/releases/tag/v1.11.0)、[OTLP 稳定定义](https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md)、[官方 Go 状态](https://opentelemetry.io/docs/languages/go/) 与 [exporter 文档](https://opentelemetry.io/docs/languages/go/exporters/)。官方 issue 样本显示 HTTP exporter 当前仍带有 gRPC 模块依赖（[#2579](https://github.com/open-telemetry/opentelemetry-go/issues/2579)）；接受其对已启用观测项目的依赖体积影响，观测未启用时整个依赖组不进入生成项目。该选择只用于可选指标/追踪，不引入日志遥测或自动 exporter 环境发现。

依赖维护证据来自 D1 及下列正式接入补充核验，模板运行验证记录以 ROADMAP 为准。主模块只引入 Cobra，默认业务模板只包含 HTTP、SQLite、JWT 与实际需要的配置/日志/DI/ORM/迁移依赖；独立兼容性模块核验的其他能力不自动进入默认项目。

## 核验方法

检查官方仓库的归档状态、许可证、最近 release/tag、默认分支最近 3 个提交与最近更新的 2 个 issue，并查看评论者身份；重要选择另查固定标签源码及相关 issue。所有候选仓库在查询时均未归档。有限样本不能证明所有问题都有人响应；机器人更新、CI 更新和关闭状态也不等于维护者修复。表中提交日期均为 UTC 日期。

## 官方证据快照

| 依赖 | Release/tag | 许可证 | 最近提交 | Issue 样本 |
|---|---|---|---|---|
| Cobra | [v1.10.2](https://github.com/spf13/cobra/releases/tag/v1.10.2)，2025-12-04 | Apache-2.0 | [2026-07-11](https://github.com/spf13/cobra/commit/adbc8813901bba65827259daa8e22ff94ec1f30e) | [#2505](https://github.com/spf13/cobra/issues/2505)、[#2520](https://github.com/spf13/cobra/issues/2520) |
| YAML parser（生成器） | [v3.0.5](https://pkg.go.dev/go.yaml.in/yaml/v3@v3.0.5) | MIT、Apache-2.0 | YAML 组织维护；v3 仅安全修复，常规开发在 v4 | [维护策略](https://github.com/yaml/go-yaml#project-status)；生成器专用，不进入业务项目 |
| Gin | [v1.12.0](https://github.com/gin-gonic/gin/releases/tag/v1.12.0)，2026-02-28 | MIT | [2026-10-10](https://github.com/gin-gonic/gin/commit/0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646) | [#4851](https://github.com/gin-gonic/gin/issues/4851)、[#4877](https://github.com/gin-gonic/gin/issues/4877) |
| Viper | [v1.21.0](https://github.com/spf13/viper/releases/tag/v1.21.0)，2025-09-08 | MIT | [2025-10-15](https://github.com/spf13/viper/commit/528f7416c4b56a4948673984b190bf8713f0c3c4) | [#2146](https://github.com/spf13/viper/issues/2146)、[#2140](https://github.com/spf13/viper/issues/2140) |
| Zap | [v1.28.0](https://github.com/uber-go/zap/releases/tag/v1.28.0)，2026-04-28 | MIT | [2026-09-16](https://github.com/uber-go/zap/commit/4892335e05f14bce8a98a69a577fcf3844a42623) | [#1593](https://github.com/uber-go/zap/issues/1593)、[#1333](https://github.com/uber-go/zap/issues/1333) |
| GORM | [v1.31.2](https://github.com/go-gorm/gorm/releases/tag/v1.31.2)，2026-06-25 | MIT | [2026-09-14](https://github.com/go-gorm/gorm/commit/b3d3bf219f0283f8e2e985bac509cb643170f729) | [#7252](https://github.com/go-gorm/gorm/issues/7252)、[#7875](https://github.com/go-gorm/gorm/issues/7875) |
| SQLite 候选 | [v1.2.2](https://github.com/libtnb/sqlite/releases/tag/v1.2.2)，2026-08-04 | MIT | [2026-09-29](https://github.com/libtnb/sqlite/commit/5f0112e3502219ebbbf63f65927915eb8a7de871) | [#2](https://github.com/libtnb/sqlite/issues/2)、[#24](https://github.com/libtnb/sqlite/issues/24) |
| SQLite 未选 | [v1.11.0](https://github.com/glebarez/sqlite/releases/tag/v1.11.0)，2024-03-14 | MIT | [2026-08-02](https://github.com/glebarez/sqlite/commit/caa6e2bf4e27bc5d7c58a61658589196a406bbf1) | [#157](https://github.com/glebarez/sqlite/issues/157)、[#152](https://github.com/glebarez/sqlite/issues/152) |
| MySQL 驱动 | [tag v1.6.0](https://github.com/go-gorm/mysql/tree/v1.6.0)；无 GitHub Release | MIT | [2026-07-31](https://github.com/go-gorm/mysql/commit/90fd5b365215ad3b7f54453d7328b2bd6980c689) | [#176](https://github.com/go-gorm/mysql/issues/176)、[#175](https://github.com/go-gorm/mysql/issues/175) |
| PostgreSQL 驱动 | [tag v1.6.3](https://github.com/go-gorm/postgres/tree/v1.6.3)；无 GitHub Release | MIT | [2026-09-14](https://github.com/go-gorm/postgres/commit/1a2d67b91586ce34c989a2b2be91f9c37c0cfb86) | [#350](https://github.com/go-gorm/postgres/issues/350)、[#332](https://github.com/go-gorm/postgres/issues/332) |
| do v2 | [v2.1.0](https://github.com/samber/do/releases/tag/v2.1.0)，2026-07-20 | MIT | [2026-09-07](https://github.com/samber/do/commit/9bae325c6606c6e68b6494f2c31ef906028d1331) | [#317](https://github.com/samber/do/issues/317)、[#318](https://github.com/samber/do/issues/318) |
| Goose | [v3.28.0](https://github.com/pressly/goose/releases/tag/v3.28.0)，2026-09-02 | [MIT](https://github.com/pressly/goose/blob/v3.28.0/LICENSE) | [2026-10-01](https://github.com/pressly/goose/commit/0e5c23df4b93cec4594830b21f66c9ae16d77f75) | [#1131](https://github.com/pressly/goose/issues/1131)、[#1126](https://github.com/pressly/goose/issues/1126) |
| JWT | [v5.3.1](https://github.com/golang-jwt/jwt/releases/tag/v5.3.1)，2026-01-28 | MIT | [2026-09-14](https://github.com/golang-jwt/jwt/commit/73c870b18e68b6e654b2b03f485aa3c9fab32cea) | [#542](https://github.com/golang-jwt/jwt/issues/542)、[#539](https://github.com/golang-jwt/jwt/issues/539) |
| Casbin | [v3.11.0](https://github.com/apache/casbin/releases/tag/v3.11.0)，2026-08-20 | Apache-2.0 | [2026-10-09](https://github.com/apache/casbin/commit/1e19095c5287eff82ae0d5cc07502d87017944f2) | [#1072](https://github.com/apache/casbin/issues/1072)、[#1672](https://github.com/apache/casbin/issues/1672) |
| 官方适配器未选 | [v3.41.0](https://github.com/apache/casbin-gorm-adapter/releases/tag/v3.41.0)，2026-01-20 | Apache-2.0 | [2026-01-20](https://github.com/apache/casbin-gorm-adapter/commit/533597758aa2e278a6b68797aa0af1c159681a91) | [#281](https://github.com/apache/casbin-gorm-adapter/issues/281)、[#249](https://github.com/apache/casbin-gorm-adapter/issues/249) |
| Redis 客户端 | [v9.23.0](https://github.com/redis/go-redis/releases/tag/v9.23.0)，2026-10-05 | BSD-2-Clause | [2026-10-08](https://github.com/redis/go-redis/commit/760b98dc0b7d8621f09e1d91d7a6b32fe555bebe) | [#4079](https://github.com/redis/go-redis/issues/4079)、[#4083](https://github.com/redis/go-redis/issues/4083) |
| gRPC | [v1.84.0](https://github.com/grpc/grpc-go/releases/tag/v1.84.0)，2026-09-17 | Apache-2.0 | [2026-10-10](https://github.com/grpc/grpc-go/commit/8c59ee6356bf92d0a5819b0cdbd3118876783028) | [#9371](https://github.com/grpc/grpc-go/issues/9371)、[#9496](https://github.com/grpc/grpc-go/issues/9496) |
| Gateway | [v2.31.0](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v2.31.0)，2026-09-25 | BSD-3-Clause | [2026-10-10](https://github.com/grpc-ecosystem/grpc-gateway/commit/8d8ffa799daeceb4a9cc67e5f187203dc8cc2e56) | [#2304](https://github.com/grpc-ecosystem/grpc-gateway/issues/2304)、[#6677](https://github.com/grpc-ecosystem/grpc-gateway/issues/6677) |
| Protobuf Go | [v1.36.12](https://github.com/protocolbuffers/protobuf-go/releases/tag/v1.36.12)，2026-08-10 | BSD-3-Clause | [2026-10-08](https://github.com/protocolbuffers/protobuf-go/commit/3933e752f9ea536e2cf45bf15b321274293863d3) | 该仓库未返回 issue |
| protoc | [v36.2](https://github.com/protocolbuffers/protobuf/releases/tag/v36.2)，2026-09-17 | [BSD-3-Clause](https://github.com/protocolbuffers/protobuf/blob/v36.2/LICENSE) | [2026-10-10](https://github.com/protocolbuffers/protobuf/commit/db4635af9d0d3ac3c3801f3a59d7c872ed2fda6d) | [#9918](https://github.com/protocolbuffers/protobuf/issues/9918)、[#29651](https://github.com/protocolbuffers/protobuf/issues/29651) |
| golangci-lint | [v2.14.0](https://github.com/golangci/golangci-lint/releases/tag/v2.14.0)，2026-09-24 | GPL-3.0 | [2026-10-09](https://github.com/golangci/golangci-lint/commit/6a4a138631363326f09a91823db60630c38cb41e) | [#6858](https://github.com/golangci/golangci-lint/issues/6858)、[#6856](https://github.com/golangci/golangci-lint/issues/6856) |

## 选型结论与工程约束

- Go 最低版本为 1.26.4，主模块为 `github.com/setcreed/goseed`。`go.mod` 声明最低兼容版本，日常构建入口设置 `GOTOOLCHAIN=local`，本机 Go 版本不低于最低要求即可；[Go 工具链选择规则](https://go.dev/doc/toolchain) 说明 `go` 指令是最低版本要求，`GOTOOLCHAIN` 控制是否切换或下载工具链。仅本地发行归档通过固定 `GOTOOLCHAIN=go1.26.4` 保持构建可复现。
- Cobra 处理命令树、帮助和参数，标准库用于模板、文件和格式化；YAML parser 仅解析业务生成器输入，不进入生成项目运行依赖。主模块不引入业务运行依赖。Cobra 样本有近期人工代码维护，但所查两个 issue 没有维护者响应证据。
- YAML parser 固定 `go.yaml.in/yaml/v3 v3.0.5`，只用于生成器读取 OpenAPI YAML 与后续资源 spec。原 `go-yaml/yaml` 仓库已归档，YAML 组织接管维护；官方说明 v3 保持稳定并接收安全修复，后续功能在 v4。当前 v4 仍是候选版，官方仓库在 2026-09 有 maintainer 发起的 [#429](https://github.com/yaml/go-yaml/pull/429) 与 [#430](https://github.com/yaml/go-yaml/pull/430)，这证明的是 v4 分支活动，不代表 v3 有常规功能维护。当前采用已发布 v3 以保持解析依赖稳定；v4 稳定后重新评估。
- Gin、Zap、Goose、gRPC、Gateway、Protobuf Go 有近期人工源码维护；Zap、Goose、gRPC、protoc、golangci-lint 的样本存在 collaborator/member 回复。详细日期、作者及评论身份可在快照核对。
- Viper 沿用成熟的配置 API，但当前样本以 2025 年 CI 更新及机器人关闭 issue 为主，不能称为近期人工响应活跃。首版只用独立实例与强类型配置，不增加热更新；若后续发现 API 缺陷，再评估替换。do v2 有近期维护，但 [健康检查 issue](https://github.com/samber/do/issues/317) 尚未解决；只用 provider/构造组装，生命周期与健康状态由应用管理。
- SQLite 选择 `github.com/libtnb/sqlite v1.2.2`，底层 modernc；[GORM 官方接入文档](https://gorm.io/docs/connecting_to_the_database.html) 提供这个纯 Go 选项。小型维护团队不能视为大型社区：最近 3 个提交是依赖机器人，但 [单连接死锁问题](https://github.com/libtnb/sqlite/issues/24) 有人工响应并关闭。用隔离驱动接入和实际行为测试控制替换成本。未选 `glebarez/sqlite`，其 [维护者说明](https://github.com/glebarez/sqlite/issues/152) 表示缺少维护时间并征集维护者。
- GORM MySQL/PostgreSQL 使用官方驱动。MySQL 样本没有维护者 issue 回复证据；PostgreSQL 有近期源码修复。GORM [默认事务超时问题](https://github.com/go-gorm/gorm/issues/7875) 仍待处理，模板使用请求 Context 与自行持有 cancel 的超时，不依赖该配置项。
- Casbin 使用同步 Enforcer，不使用 CachedEnforcer；近期缓存判断修复不代表已进入 v3.11.0。策略存储复用现有 GORM 连接，表由 Goose 显式迁移。官方适配器虽然提供关闭自动建表开关，但 [固定版本源码](https://github.com/apache/casbin-gorm-adapter/blob/v3.41.0/adapter.go) 与 [依赖清单](https://github.com/apache/casbin-gorm-adapter/blob/v3.41.0/go.mod) 同时引入多种驱动及旧 SQLite 库，会破坏按参数裁剪的依赖边界。D1 原型仅验证可行性；正式模板提供独立 Context 存储接口、严格输入约束和受控快照替换，详细边界见下方补充核验。
- JWT v5.3.1 的 [签名段 CR/LF issue](https://github.com/golang-jwt/jwt/issues/542) 有维护者讨论；底层 Base64 解码可能接受换行。D2 在解析前校验三段 Base64URL 字符语法，并固定算法和必需 claims，不能只依赖签名校验或 `WithStrictDecoding`。
- Redis 客户端仅由 Redis 选项引入，明确设置读写、连接池和命令预算，禁用隐式重试与实验性流水线；取消约束、正式接入和真实验证边界见下方补充核验。

标准库没有 GORM 的方言适配、Casbin 的权限模型、SQL 迁移版本机制或 Gateway 代码生成能力，因此这些库用于对应能力；不再引入同类框架。Zap 与 Viper 用于已确定的日志和配置方案。

## SQL Connector 补充核验

2026-10-11 通过官方 GitHub GraphQL 核对 `go-sql-driver/mysql` 与 `jackc/pgx` 的归档、许可证、发布、最近三次提交及两个 issue 样本，精简响应保存在 `dependency-audit.json` 的 `sql_connector_supplement`。两仓库均未归档；MySQL 为 MPL-2.0，pgx 为 MIT，与固定模块内许可证一致。

模板固定 `mysql v1.10.0`、`pgx v5.10.0`，沿用 D1 已验证的兼容性基线；它们不是查询时最新版本。最新发布分别为 [MySQL v1.10.1](https://github.com/go-sql-driver/mysql/releases/tag/v1.10.1) 和 [pgx v5.11.0](https://github.com/jackc/pgx/releases/tag/v5.11.0) 。最近人工提交为 MySQL [2026-10-09](https://github.com/go-sql-driver/mysql/commit/cac656f386d90128f33461e27993af1699049fe6) 、pgx [2026-10-05](https://github.com/jackc/pgx/commit/3a84369ef796ce81fef379d9c129a3f693ea8f2f) ；issue 首批评论样本不足以声称近期响应及时。版本升级需要独立变更与对应真实服务回归。

这两项已经是 GORM 方言的底层依赖，直接使用是为了构造独立 SQL 连接池、隔离 TLS 配置、禁止明文回退以及确定 UTC 编解码，不再引入另一套连接库。标准库 `database/sql` 本身不提供 MySQL/PostgreSQL 协议驱动。固定版本在 `dependencies.lock.json` 的 `sql_connectors`；生成清单只包含所选 Connector。

MySQL 默认配置不保证 TLS，见 [TLS 默认值 issue](https://github.com/go-sql-driver/mysql/issues/1728) ；模板显式提供 TLS 配置，不使用全局注册。真实验证发现连接握手不支持 309 号 `utf8mb4_0900_bin`，因此连接仅设置 utf8mb4，唯一约束的二进制语义由迁移中的表列决定。驱动协议日志统一为脱敏 JSON 事件。

pgx 存在未解决的 [缓冲响应与取消竞争报告](https://github.com/jackc/pgx/issues/2673) ，报告涉及 v5.11.0 及 v5.8.0，不能认为升级即可解决。模板使用请求 Context 和无语句缓存执行模式；实际验证覆盖运行中 deadline 及随后复用连接，尚未复现该报告的人工延迟 Write 调度场景。后续驱动维护与上线负载验证须继续核对该问题，基础回归不证明所有取消竞争已消除。

## Casbin 补充核验

2026-10-11 通过官方 GitHub GraphQL 重新核对 `apache/casbin` 的归档、许可证、最新发布、最近三次提交与两个 issue 的评论身份，响应保存在 `dependency-audit.json` 的 `casbin_supplement`。仓库未归档，Apache-2.0，最新 release 仍为 [v3.11.0](https://github.com/apache/casbin/releases/tag/v3.11.0)（2026-08-20）；正式模板沿用此固定版本。最近人工源码维护为 [2026-10-09](https://github.com/apache/casbin/commit/1e19095c5287eff82ae0d5cc07502d87017944f2)；所查 issue 中有 MEMBER 历史回复，但不足以声称近期响应及时。

标准库与 JWT 不提供 RBAC 角色权限模型，Casbin 仅负责精确权限判定；用户认证和资源归属分别由 JWT、Biz 负责。[官方 RBAC 文档](https://casbin.apache.org/docs/rbac/) 指出用户与角色都是字符串，应避免同名；模板强制 `user:`/`role:` 前缀及严格 p/g 校验，不开放模型表达式和角色继承。

固定 [v3.11.0 构造与配置源码](https://github.com/apache/casbin/blob/v3.11.0/enforcer.go) 使用事件 Logger API；模板显式 `SetLogger(nil)`，不沿用旧版 EnableLog/末尾布尔参数。不使用 CachedEnforcer，关闭 g 函数缓存，避免无界主体缓存及依赖未发布的缓存修复。同步 Enforcer 仅用于已经验证的完整快照，策略读取和保存由显式 Context 的存储接口执行，绕过 SDK 隐式加载与 AutoSave；SQL 使用已有 GORM 连接，不增加多驱动官方适配器。

真实验收入口为 `make template-db`、`make template-redis` 和 `make template-container`；单独兼容性模块仍保留 D1 原型用于版本核验，不当作正式实现。版本升级或扩展模型需独立权限、并发与真实存储回归。

## Redis 补充核验

2026-10-11 通过官方 GitHub API 重新核对 `redis/go-redis` 的归档状态、许可证、最近发布、最近三次人工提交和 issue 评论，精简响应保存在 `dependency-audit.json` 的 `redis_supplement`。仓库未归档，许可证 BSD-2-Clause，最新发布仍为 [v9.23.0](https://github.com/redis/go-redis/releases/tag/v9.23.0)（2026-10-05），沿用已固定版本。最近人工代码维护为 [2026-10-08](https://github.com/redis/go-redis/commit/760b98dc0b7d8621f09e1d91d7a6b32fe555bebe)；[#4079](https://github.com/redis/go-redis/issues/4079) 仍开放，评论仅有报告者补充，不能声称维护者已经修复取消问题。

标准库不提供 Redis 协议客户端、连接池、认证和重连能力，因此正式 Redis 变体使用 go-redis，Biz 仅依赖本项目缓存接口。固定 [v9.23.0 配置源码](https://github.com/redis/go-redis/blob/v9.23.0/options.go) 对应 RESP2、禁用重试/实验流水线、开启 Context 超时和明确 socket/池超时。每次命令保留调用者 Context，并限制为默认 200ms 的短预算；纯 cancel 可能等待 socket 预算才返回，不承诺瞬时取消。受控 RESP 连接和真实 TLS Redis 的运行中 cancel/deadline、后续池复用均有验收，覆盖当前配置，不证明上游全部竞争场景已解决。

连接失败只记录脱敏操作和请求 ID，不输出密码、地址、键值或原始 SDK 协议日志。内部缓存命令超时映射为依赖不可用（必需模式 HTTP 503，可选模式回源），调用者取消或 deadline 映射为 HTTP 504；两种预算不能混淆。连接建立与 CA 校验、启动失败、就绪和退出由应用管理，不依赖 DI 自动生命周期。

真实验收固定 [Docker Official Images 的 Redis 清单](https://github.com/docker-library/official-images/blob/master/library/redis) 中的 `8.10.2-alpine`。通过官方 Registry API 核对多架构摘要 `sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0`，并在实际 TLS 容器验收中核对 RepoDigest；版本见锁定清单 `redis_image`，来源记录在 `redis_image_supplement`。`make template-redis` 只使用本次唯一名称/标签、密码和证书，不使用共享实例或执行全库清空。

## 协议工具及产物

`protoc 36.2`、`protoc-gen-go v1.36.12`、`protoc-gen-go-grpc v1.6.2` 和 Gateway/OpenAPI 插件 `v2.31.0` 全部固定。gRPC 插件独立发布，版本不能与 gRPC 运行库混用；其 [固定标签 go.mod](https://github.com/grpc/grpc-go/blob/cmd/protoc-gen-go-grpc/v1.6.2/cmd/protoc-gen-go-grpc/go.mod) 要求 Go 1.25.0。golangci-lint v2.14.0 的 [go.mod](https://github.com/golangci/golangci-lint/blob/v2.14.0/go.mod) 要求 Go 1.26.0。

`protoc` 下载资产和 SHA-256 来自官方 release API，校验后安装到 `.tools/`；Go 插件由指定工具链安装到 `.tools/bin/`。普通构建只读取已生成文件，不要求 protoc，也不安装工具。

首版接口文档统一为 **Swagger 2.0（OpenAPI 2.0）**，不标记为 OpenAPI 3.x。D1 的无注解协议仅用于生成与回环通信验证；业务模板在 D4 使用显式 HTTP 注解确定资源路径。

工具安装入口为 `make tools` 和 `make protocol-tools`；后者目前锁定 macOS/Linux 的 amd64/arm64 资产，Windows 主机的安装入口尚未提供。业务依赖的 Windows amd64 纯 Go 交叉编译是独立验证范围。

## 许可证范围

直接业务运行依赖采用 MIT、Apache-2.0 或 BSD 系列许可，正式生成模板需要携带适用的版权与许可说明。Goose/protoc 的 GitHub 自动许可证识别不明确，已检查固定标签 LICENSE。golangci-lint 的 GPL-3.0 仅限开发工具，不进入业务运行链或生成器二进制；对外分发工具时另行处理义务。goseed 自身采用 [MIT](../LICENSE)，许可文本按 [Open Source Initiative 标准文本](https://opensource.org/license/mit) 保存。本项目许可证不替换第三方依赖许可，公开源码与分发包含的第三方代码仍须保留适用说明。

## 可复现验证入口

## 依赖变更维护流程

每次只升级一个库、工具或镜像。先查官方发布/tag、固定版本源码中的许可证和 Go 要求、近期维护以及相关 issue，再修改对应清单：主模块更新 `go.mod`/`go.sum`，生成模板同时更新 variant manifest，协议工具同步 lock 与下载脚本，容器镜像同步摘要和 Compose。更新 [本文件的依赖证据](#官方证据快照) 与锁文件后，执行受影响 Go 包、生成项目 readonly 构建和协议复现；改动 SQL/Redis/协议/基础镜像时再执行对应真实服务目标。没有受影响的真实服务矩阵不重复运行，但不得用 SQLite、接口替身或交叉编译声称真实兼容。记录验证日期、实际目标与跳过范围到 ROADMAP。

- `make check`：生成器测试、vet、固定版本 lint 与全部项目 Go 文件的格式检查。
- `make race build`：生成器竞争检测与构建。
- `make compatibility`：独立模块的运行依赖、SQLite 和实际 gRPC/Gateway 回环验证；外部数据库缺失时明确跳过。
- `make compatibility-cross`：纯 Go 交叉编译独立模块与生成协议的测试程序，不在宿主机执行跨平台二进制。
- `make compatibility-db`：新建唯一 Compose 项目、回环随机端口和全新测试卷，要求真实 MySQL/PostgreSQL 测试通过，结束后只清理本次资源。
- `make protocol-check`：校验工具版本，在 `.tmp/` 重新生成并比较 Go/gRPC/Gateway/Swagger 文件。
- `make template-redis`：固定摘要的专用 TLS Redis，验证缓存、隔离、TTL、取消、TLS/认证失败及必需／可选模式故障恢复；结束仅清理本次容器与凭据。

交叉编译使用 `CGO_ENABLED=0`，验证测试程序可为 Linux amd64、Linux arm64 和 Windows amd64 构建；本机 darwin/arm64 实际运行。交叉编译不代表已在其他平台运行。完整运行结果与尚未完成项统一记入 [ROADMAP](../ROADMAP.md) 。

数据库测试使用 [MySQL 官方镜像](https://hub.docker.com/_/mysql) 的 8.4.11 和 [PostgreSQL 官方镜像](https://hub.docker.com/_/postgres) 的 18.6；实际拉取并测试后，将 RepoDigest 写入版本清单与 Compose 文件，避免 tag 后续变化影响复现。首版业务模板的多版本数据库支持范围留到 D3 验证，D1 不据此声明支持所有服务器版本。

## 默认模板与容器

默认业务模板保存完整 `go.mod` 与 `go.sum`；集成检查直接 readonly 构建、测试、vet、race 和 lint，并核对运行后清单未变化。模板直接依赖及工具链与根目录锁定清单比对，不在普通检查中自动 tidy。

dual 的 16 种固定清单位于 `internal/templates/variants/dual/manifests/`，合并已有 HTTP 与 gRPC 所需依赖，沿用相同固定版本，没有新增运行库。无数据库、Redis 与 Casbin 按选择裁剪；生成项目同时交付完整校验和，普通检查不得通过 tidy 补齐依赖。

Gateway 的固定清单位于 `internal/templates/variants/gateway/manifests/`，使用已在 D1 核验并固定的 grpc-gateway v2.31.0 与相同版本生成插件，移除 Gin。2026-10-11 复核 [固定版本发布](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v2.31.0) 、[官方外部 HTTP 配置](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/grpc_api_configuration/) 与 [定制接口](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/customizing_your_gateway/) ，并对照项目缓存中的固定版本源码确认插件选项、方法覆盖与表单回退关闭入口、metadata 映射。HTTP 映射使用 Proto 同目录 YAML，无需额外下载 Google API Proto 或引入 Buf；普通构建无生成工具依赖。输入、TLS 和生命周期约束见 [Gateway 契约](template-gateway.md) 。

2026-10-11 通过官方 Registry API 与 `docker manifest inspect` 核对 `golang:1.26.4-alpine` 多架构摘要，固定为 `sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648`。镜像的 [官方固定提交 Dockerfile](https://github.com/docker-library/golang/blob/34bfa3703c98a3f4e7dcc808a5b0a754faecb5ef/1.26/alpine3.24/Dockerfile) 明确 Go 1.26.4、平台下载校验和与 CA 证书；模板使用该构建镜像和纯 Go 二进制，运行阶段为 scratch、UID/GID 10001。

实际容器验证通过 `make template-container` 复现，使用专用标签、全新数据卷与随机回环端口；不推送镜像、不操作现有数据库，测试结束清理本次容器、卷与镜像。镜像摘要升级需重新核验并同时更新锁定清单和模板。
