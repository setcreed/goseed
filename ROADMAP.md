# 项目进度

## 当前状态

文档基线、D1、D2、D3、D4（HTTP、纯 gRPC、dual、Gateway）与 D5 项目生成器已完成。四种协议均支持四种存储、可选 Redis 与 Casbin，共 64 种组合；MySQL、PostgreSQL、Redis 已完成真实服务验证，Gateway 生产容器验收通过。根命令已支持当前目录生成、能力选择、版本化元数据和 dry-run；业务模块与 CRUD 生成属于 D6。首次基线提交为 `01c8d6c`，本地分支为 `main`，未配置远端。主模块为 `github.com/setcreed/goseed`，工具链固定为 Go 1.26.4。

纯 gRPC 已按 [协议契约](docs/template-grpc.md) 交付共享业务层、安全链、健康服务、TLS、示例客户端与公开 `pkg/api` 协议类型；dual 按 [双协议契约](docs/template-dual.md) 共享同一 Biz 与依赖，提供独立监听覆盖、启动失败回收和并发排空。Gateway 按 [Gateway 契约](docs/template-gateway.md) 将 HTTP 请求转发到真实 RPC 服务，复用认证、授权和业务层，并先排空 HTTP 再关闭 RPC。`Options.Transport` 支持 `http`（默认）、`grpc`、`dual` 与 `gateway`。Redis 与 Casbin 默认关闭，分别通过内部 Render 的 `Options.Redis`、`Options.Authz` 选择。缓存按 TTL 最终一致；Casbin 默认拒绝，SQL 策略显式迁移与事务导入，无数据库策略只读，离线更新后重启实例。项目生成命令以 [命令契约](docs/cli.md) 为准；D6 命令、字段、文件所有权和登记契约已确定，HTTP/dual 路由及四种协议的 RPC Service/权限、Gateway HTTP 映射已接入登记索引。OpenAPI 确定性合并器已完成，项目初始模板会写入规范化 Swagger 和摘要；新增模块的源文件读取及索引更新流程、业务生成命令仍待实现。需求以 [产品需求](docs/requirements.md) 为准，实施顺序见 [实施方案](docs/implementation.md) ，默认模板契约见 [默认 HTTP 模板](docs/template-http.md) ，变体契约见 [可选模板](docs/template-options.md) 。

## 进行中

D6 进行中：HTTP/dual 路由及四种协议的 RPC Service/权限、Gateway HTTP 映射已接入登记索引；OpenAPI 多源合并、规范化摘要已实现，dual 索引同时登记 HTTP 与 RPC。默认 HTTP、dual、Gateway 代表项目集成通过。当前检查点是把模块 YAML 读取、索引一致性/摘要验证和安全写回接入生成计划，再交付 `gen module` 与 `gen resource`。

## 待开始

| 检查点 | 交付 | 依赖 |
|---|---|---|
| D6 | 模块与 CRUD 业务生成器 | D5 |
| D7 | 首版覆盖核验和使用文档 | D6 |

所有检查点均需完成对应验收后才能标记完成。

## 待核验与待确认

- 跟踪：go-redis 的纯取消 issue 仍开放；当前明确短命令预算、关闭命令与拨号重试，受控连接及真实服务取消已通过，但不承诺瞬时中断或全部取消竞争已消除。官方证据见 [依赖核验](docs/dependencies.md) 。
- 待核验：pgx 缓冲响应与取消竞争的人工延迟 Write 场景；基础运行中 deadline 与随后池复用已通过，未声明解决上游报告。官方证据与版本边界见 [依赖核验](docs/dependencies.md) 。
- 待确认：goseed 自身许可证；仅在公开发布前必须确定。主模块已确定为 `github.com/setcreed/goseed`，Go 工具链固定为 1.26.4。
- 待实现：将 OpenAPI 多源合并器接入实际模块生成和登记索引安全更新；实现资源 spec 严格解析、dry-run 生成计划及 `gen module`／`gen resource` 命令。Gateway HTTP 映射及 query/body 输入规则已由模块登记描述驱动。

## 最近完成

- 2026-10-12 00:02 D6 OpenAPI 合并器：新增生成器专用 YAML 依赖和多源 Swagger 2.0 合并器，按模块名稳定合并 paths、组件与 tags，拒绝重复键、别名、merge key、自定义标签、重复操作及 operationId，输出规范 JSON 与 SHA-256。初始模板渲染和 Gateway Proto 复现均规范化 Swagger，并把摘要写入登记索引；补齐 dual 专用 HTTP/RPC 合并登记模板。模块文件发现、摘要校验及后续索引安全写回仍待接入业务生成命令。

- 2026-10-11 23:24 D6 Gateway HTTP 登记接入：Gateway 启动组装从 `modules_gen.go` 读取模块名、路径、query 参数、JSON body 方法和 gRPC-Gateway 注册回调；请求日志路由与 query 校验使用同一登记描述，移除 Handler 对示例路径和字段的绑定。SQLite 与无数据库 Gateway 代表项目通过集成验证；OpenAPI 汇总及 D6 命令仍未实现。

- 2026-10-11 23:17 D6 RPC 登记接入：gRPC-only、dual 与 Gateway 的启动组装改用 `modules_gen.go` 返回的 RPC Service 注册回调和 FullMethod 权限映射；HTTP-only 渲染不带 gRPC 登记依赖。

- 2026-10-11 23:12 D6 HTTP/dual 登记接入：生成的 `modules_gen.go` 为 HTTP 业务模块提供路由、名称、路径和操作权限描述；通用路由按登记列表挂载 Handler，Casbin 权限使用模块描述，HTTP 与 dual 共用该入口；gRPC-only/Gateway 渲染裁剪 HTTP 登记文件。业务 Handler 和 Bootstrap 仍由模板维护，生成器暂未开放 `gen` 命令。

- 2026-10-11 23:02 D6 登记接口原型：新增兼容性测试，编译并验证 HTTP 路由组回调、gRPC Service 注册/FullMethod 权限映射、Gateway ServeMux 注册回调与文档输入的确定顺序；明确这是接口原型，尚未接入四种生成模板。同步命令、架构和实施契约，下一步验证真实模板。

- 2026-10-11 22:58 D6 契约定义：确定 `gen module`／`gen resource` 命令、版本 1 YAML 字段规则、类型与校验语义、保留字段、主体隔离、目标文件范围及 no-overwrite 边界；以 `goseed.yaml` 为模块清单权威源、`modules_gen.go` 为带摘要的确定性注册索引。更新 CLI、需求、架构和实施文档；D6 代码尚未开始。

- 2026-10-11 22:55 验证收尾规范：更新 AGENTS，要求每次验证结束后立即清理本次新建的临时项目、日志和可重建产物/缓存，检查清理结果并尽量复用缓存；明确保留任务前已存在、来源不明及其他任务数据。

- 2026-10-11 22:45 D5 项目生成器：根命令默认在当前目录生成 HTTP／SQLite／JWT 项目，支持项目名、模块路径、数据库、Redis、Casbin、四种协议和 dry-run；固定 JWT 为基础能力。生成前渲染全部文件并检查目录、路径与冲突，独占创建、不覆盖已有文件，写入失败仅回滚本次创建内容；允许保留 `.git` 与系统文件。生成 `goseed.yaml` 记录版本、模块、能力和初始 `example` 登记，dry-run 展示文件、依赖版本和冲突。README 与需求、架构、命令契约同步；下一检查点为 D6。

- 2026-10-11 22:07 D4 Gateway：`Options.Transport: "gateway"` 通过实际 gRPC 服务转发 HTTP／ProtoJSON 请求，共享 Handler、Biz、认证和授权链；交付双监听配置、内部 TLS 验证、健康探测、HTTP 优先排空及 Gateway／Swagger 产物，裁剪重复 Gin 业务 Handler。16 种变体组合与四协议共 64 种组合契约通过；补齐 Gateway 文档、依赖和容器／数据库／Redis 验收。下一检查点为 D5。入口见 [Gateway 契约](docs/template-gateway.md) 。

- 2026-10-11 19:43 文档同步：README 增加当前模板能力表，明确 48 种组合、固定 JWT 与内部 Render／CLI 边界；命令契约区分生成参数和三种已实现协议的服务监听参数。架构目录补齐认证、缓存、数据库、RPC、dual 和示例客户端，标明 D5／D6 规划目录；双协议文档按存储选择说明共享依赖，无数据库文档不再声称包含 Store／连接池。核对 AGENTS 规则仍适用，无需制造改动；下一功能仍为 Gateway，代码行为未变。

- 2026-10-11 17:51 D4 dual：Render 支持 `Options.Transport: "dual"`，HTTP／gRPC Handler 共用同一 Biz、数据库池、JWT、可选 Redis 与 Casbin；交付独立监听覆盖、双端口预绑定、失败清理与并发排空，保留 Swagger、Proto 和示例客户端。新增 16 种组合及真实 SQL／TLS Redis／双端口生产容器验收通过，HTTP／纯 gRPC 回归通过；同步架构、配置、依赖与生成文档。Gateway 为下一功能，CLI 写入仍属 D5。入口见 [双协议模板](docs/template-dual.md) 。

- 2026-10-11 12:55 D4 纯 gRPC：Render 支持协议选择，交付 SQL CRUD／无数据库问候的 Proto、公开 `pkg/api`、Handler、服务与示例客户端；复用 Biz／Store、JWT、Redis 和 Casbin，裁剪 Gin／业务 HTTP／Swagger。提供强类型配置、请求 ID、标准错误与日志脱敏、optional 零值、预算／取消、健康 Check／Watch、生产 TLS、开发认证 Reflection 和有限期退出；固定版本显式 Proto 生成无需普通构建重新生成。32 种 HTTP／gRPC 组合与真实数据库／Redis／生产容器验收完成，同步目录和使用文档。CLI 写入仍属 D5。入口见 [纯 gRPC 模板](docs/template-grpc.md) 。

- 2026-10-11 12:12 D3 Casbin：Render 支持 `Options.Authz`，交付固定 RBAC、默认拒绝、用户／角色命名空间及 JWT 后的业务授权，资源归属仍由 Biz 校验。三种 SQL 复用现有连接，显式迁移策略表、Context 存储与事务全量导入；无数据库提供只读 CSV。策略严格校验，完整快照成功后发布，失败保留旧策略；离线更新需重启实例，不提供公开策略管理 API。八种新增组合与 Redis／数据库／容器验收通过，D3 完成，下一检查点为 D4。入口见 [可选模板](docs/template-options.md#casbin-可选能力) 。

- 2026-10-11 11:40 项目命名文档：补充 `--name`、`--module`、默认值、名称校验及目录行为，明确目录名/项目名/模块路径末段可不同，`--name` 不创建或重命名目录。README 说明内部模板当前支持名称与路径替换，创建命令示例明确属于 D5 契约；AGENTS 已有命令文档入口，无需重复规则。CLI 实现未变化，下一功能仍为 Casbin。入口见 [命令契约](docs/cli.md#项目名模块路径与目录) 。

- 2026-10-11 07:35 D3 Redis：Render 支持 `Options.Redis` 与四种存储选择组合，生成时裁剪客户端、配置、依赖、文档与测试；Biz 使用缓存接口，交付主体/资源摘要 Key、成功读取缓存、坏值回源、TTL 和写后失效。提供可选故障回源、必需启动/就绪/业务阻断、验证 TLS、脱敏日志与 App 统一关闭；固定 go-redis v9.23.0 和 Redis 8.10.2 镜像摘要，交付 `make template-redis`。同步项目与生成产物文档，root CLI 写入仍属 D5；下一功能为 Casbin。契约见 [可选模板](docs/template-options.md) 。

- 2026-10-11 07:05 D3 无数据库：Render 支持 `Database: "none"`，保留 HTTP/JWT 与公共配置、日志、生命周期；裁剪 ORM、SQL 驱动、数据库配置、模型、Store 和迁移。交付 Handler → Biz 认证问候示例、严格 query/文本校验、固定清单、Swagger 2.0 和独立项目文档；Make 开发入口不迁移，Docker 不创建数据目录。root CLI 写入保持 D5 范围，下一功能为 Redis。契约见 [可选模板](docs/template-options.md) 。

- 2026-10-11 06:49 D3 SQL 数据库：内部 Render/Options 支持 SQLite、MySQL、PostgreSQL，生成时裁剪驱动、配置、SQL 方言与固定清单；外部数据库提供独立 Connector、默认验证 TLS、连接池、UTC 与脱敏协议日志。交付真实数据库夹具与 `make template-db`，同步变体文档；统一 JWT 主体上限、控制字符及业务文本 NUL 边界，MySQL 相同值更新按匹配行数处理。D3 的其他能力保持待开始。

- 2026-10-11 01:21 D2 完成：交付完整独立 HTTP/SQLite/JWT 项目模板、固定 go.mod/go.sum、项目文档与规范、配置/迁移/Makefile、Swagger 2.0、固定摘要非 root Dockerfile；普通检查不 tidy。HTTP 标准库协议错误接入统一脱敏日志，补齐进程级启动失败、可信 TLS 与 CORS 验收；交付入口为 README。
- 2026-10-11 01:10 D2 错误与请求边界：Store 错误保留内部 cause 并按稳定业务码判定，错误文本不披露原因；数据库日志关联请求 ID，补齐请求预算及开发配置拒绝覆盖/符号链接回归，证书和私钥默认忽略。
- 2026-10-11 01:00 D2 配置类型修复：在环境变量绑定前校验 YAML 原始类型，拒绝数字 issuer、小数连接数和数字布尔值，避免弱类型解码静默改变配置。
- 2026-10-11 00:58 D2 示例 CRUD：Handler → Biz → Store、Biz 仓储接口、资源主体隔离、唯一冲突、UTC 时间、分页和零值 PATCH；写入拒绝未知/重复/null/额外 JSON/无效 UTF-8/超限正文，所有示例业务路由绑定 JWT。
- 2026-10-11 00:52 D2 JWT：固定 HS256 和原始 Token 语法，强制签名、主体、issuer/audience、exp/iat/nbf 和最大生命周期；中间件传递请求主体，开发 CLI 签发，生产禁用，无公开签发接口。
- 2026-10-11 00:49 D2 SQLite：嵌入 SQL 显式迁移、只读版本与 schema 校验、真实文件安全初始化、连接池和每连接 PRAGMA、无 SQL 参数的查询日志；服务就绪绑定数据库，退出及启动失败关闭连接池。
- 2026-10-11 00:46 D2 HTTP 服务基础：标准库命令入口、开发密钥初始化、Bootstrap DI 组装、健康检查、请求 ID、脱敏错误与访问日志、CORS/预算/TLS/pprof、优雅退出及项目内 Make 检查模板。
- 2026-10-11 00:36 D2 配置与日志：模板提供强类型配置、显式环境绑定、覆盖顺序、未知键及带单位时长校验、生产秘密和 JSON 日志要求；日志带服务、版本和 UTC 时间，渲染项目通过自身测试及 vet。
- 2026-10-11 00:36 模板系统元数据修复：复现 Finder 创建 `.DS_Store` 后渲染失败；改用默认嵌入过滤并支持 `dot.<name>.tmpl` 输出隐藏文件，渲染忽略已知系统元数据。
- 2026-10-11 00:30 D2 模板渲染：嵌入模板返回确定顺序的完整文件计划，校验名称/模块路径，使用独立分隔符、缺失变量检查和 Go 格式化；准备阶段失败返回空计划，CLI 生成写入仍在 D5 接入。

## 最近验证

- 2026-10-12 00:02 D6 OpenAPI/登记回归：核心包测试、`make vet`、`make lint`（0 issues）、`make contract-check`（68 个本地链接、321 个交付文件）、`make fmt-check` 与 `git diff --check` 通过。分别运行默认项目、`TestDualProjects/(sqlite-none|none-none)`、`TestGatewayProjects/(sqlite-none|none-none)`；生成项目 readonly 编译、race/vet/lint、本机服务调用通过，Gateway Proto／Swagger 逐字节复现通过。Gateway 首次复现发现模板规范化与 protoc 原始输出格式不一致，已规范化复现脚本并重跑通过。完整 `make test` 矩阵未完成；首次模板测试断言的编译问题已修正，后续全量矩阵被主动停止，不能视为全矩阵通过。验证后清理本次生成的项目和 Go/lint 构建缓存，保留历史 `.tmp/d2/render-3653844838` 与可复用模块缓存。

- 2026-10-11 23:17 D6 RPC 登记：`go test ./internal/templates` 通过；在获准的本机回环环境运行 `go test ./tests/integration -run '^Test(GRPCProjects|GatewayProjects)/(sqlite-none|none-none)$'`，SQLite 与无数据库的 gRPC-only、Gateway 代表项目全部通过 readonly 编译、race/vet/lint、Proto 复现和服务回环调用；清理本次测试写入的 28,811 个 Go 构建缓存文件及 132 MiB 隔离缓存，保留历史共享缓存。

- 2026-10-11 23:18 D6 文档/格式收尾：`make contract-check`（68 个本地链接、317 个交付文件）、`make fmt-check`、`git diff --check` 通过；本轮专用缓存与集成测试生成目录已清理。

- 2026-10-11 23:12 D6 HTTP/dual：`go test ./internal/templates` 与新增 transport-scope 测试通过；在获准的本机回环环境执行 `go test ./tests/integration -run '^Test(DefaultProject|NoDatabaseProject|DualProjects)$'`，三个生成项目集成套件无跳过通过，覆盖 readonly 编译、race/vet/lint、HTTP/JWT CRUD、无数据库入口及 dual HTTP+gRPC 行为。清理本次 132 MiB 隔离 Go 构建缓存；`.tmp/d2` 仅保留任务前已有目录。23:13 `make contract-check`（68 个本地链接、316 个交付文件）、`make fmt-check` 与 `git diff --check` 通过，隔离缓存目录已确认不存在。

- 2026-10-11 23:02 D6 登记接口原型：`make compatibility` 首次受沙箱回环监听限制；获准本机重跑后全套兼容性测试通过，包括新增 `TestGeneratedModuleRegistryContract`、Proto 回环测试和 SQLite 行为测试；MySQL/PostgreSQL 兼容性子用例按该目标设计跳过，真实服务仍由 `make compatibility-db` 验证。清理本次隔离 Go 构建缓存。
- 2026-10-11 23:03 D6 文档与格式回归：`make contract-check`（68 个链接、315 个交付文件）、`make fmt-check`、`git diff --check` 全部通过；确认本次隔离缓存目录已清除。

- 2026-10-11 22:58 D6 契约文档：`make contract-check` 通过（68 个本地链接、314 个交付文件），`git diff --check` 通过；复核命令示例、字段约束、登记文件所有权和 D6 未实现状态一致。

- 2026-10-11 22:55 AGENTS／ROADMAP 文档：`make contract-check` 通过（66 个本地链接、314 个交付文件），`git diff --check` 通过；核对临时文件收尾约定与文档入口。

- 2026-10-11 22:45 D5：`make check` 与 `make build` 通过；`make check` 覆盖全部 Go 包、模板组合集成测试、vet 与 lint（0 issues）。CLI、生成器、模板定向测试、格式、契约链接和 diff 检查通过；构建出的二进制实际运行默认生成与 PostgreSQL + Redis + Casbin + Gateway dry-run，检查默认元数据、依赖版本和无落盘行为。沙箱曾因禁止回环监听拒绝集成用例，获准在本机重跑后全套通过。

- 2026-10-11 22:07 D4 Gateway：`make check` 的根模块集成测试、`make fmt-check`、`make vet`、`make lint`、协议复现与 `make contract-check`／`git diff --check` 通过；新增的 Gateway 包 race 与 IPv4／IPv6 通配监听目标回归通过。`make template-db` 的 MySQL／PostgreSQL Gateway 项目、`make template-redis` 的 SQLite／无数据库及 Casbin 组合均实际运行通过且无跳过；启用 Docker 的 Gateway TLS 容器实际测试通过。Redis 重启后的旧 gRPC 健康恢复用例曾暴露瞬时观察竞态，改为有界 5 秒轮询后完整 Redis 套件通过；没有改变生产服务重试行为。

- 2026-10-11 19:43 文档同步：`make contract-check` 通过版本契约、55 个本地链接、Python 语法与 260 个交付文件检查，`git diff --check` 通过；`make test GO_PACKAGES=./internal/templates` 通过，验证文档模板可渲染及交付链接一致。对照 CLI 源码、Render 选项、模板路径和 Makefile，核对当前默认值、监听参数与 48 种组合；仅改文档及一份生成文档模板，没有重跑数据库或容器，沿用 17:51 的运行验收证据。

- 2026-10-11 17:51 D4 dual：`make check`、`make race build`、`make template-db`、`make template-redis`、`make template-container` 全部通过，lint 零问题；48 种组合契约与原有协议回归通过。新增 16 种 dual 项目 readonly 编译测试包及 Linux amd64 服务／客户端，代表项目完成 race／vet／lint、跨协议 CRUD／问候、主体／角色隔离、TLS、Proto 逐字节复现与独立客户端导入，清单未变化。真实 MySQL／PostgreSQL 与 TLS Redis 无跳过验证持久化、授权先于缓存、故障回源、必需阻断与重连后两侧恢复；就绪使用独立契约，恢复限 5 秒。双端口占用、证书失败、共享池清理、任一 Serve 异常、并发强制退出及两侧在途请求成功排空通过，补强后两个代表项目回归通过。生产容器验证双端口 TLS、非 root、只读根文件系统、只读证书／策略挂载与重启；相关 Docker 测试标签的容器、镜像、卷和网络核对为空，仅清理本次中断生成项目与临时目录，保留共享缓存。CLI 二进制确认 Go 1.26.4、模块路径及帮助／版本行为。

- 2026-10-11 12:55 D4 纯 gRPC：最终 `make check`、`make race build`、`make template-db`、`make template-redis`、`make template-container` 通过，lint 零问题；原有 HTTP 回归通过。新增 16 种 gRPC 组合 readonly 编译测试包及 Linux amd64 纯 Go 服务／客户端；四个代表组合完成 test/vet/race/lint、实际 RPC／TLS、交付 Proto 脚本逐字节复现及独立 Go 模块导入，清单未变化。真实 MySQL／PostgreSQL 与 TLS Redis 无跳过验证 CRUD、主体／角色隔离、权限撤销先于缓存、持久化、必需 Unavailable／健康 NOT_SERVING、可选回源和恢复；内部缓存预算与调用方 deadline 分开映射，原始 Handler 错误／panic 脱敏，Reflection 提前拒绝的 EOF 竞态回归通过。生产 TLS 容器验证非 root、只读根文件系统、证书／策略只读挂载和重启；本次测试标签下容器、镜像、卷和网络只读核对均为零。仅清理本次临时目录及已记录的中断旧验证目录，保留共享缓存。

- 2026-10-11 12:12 D3 Casbin：最终 `make check`、`make race build`、`make template-db`、`make template-redis`、`make template-container` 通过；16 种组合契约及八种新增模板 readonly 测试/vet/race/lint、Linux amd64 纯 Go 构建通过，清单未变化。真实 SQLite/MySQL/PostgreSQL 验证显式迁移、持久化、撤销、唯一约束、保存失败回滚、连接池 deadline 与随后复用；外部数据库无跳过，并先验证 SQL v1 再升级 Casbin v2。真实 TLS Redis 与只读文件容器验证默认拒绝、主体／角色隔离、授权先于缓存、错误导入保留旧策略、重启撤销及日志脱敏；原有数据库、缓存和容器回归通过。仅清理本次临时目录与专用 Docker 资源，相关容器、测试镜像、卷和网络只读核对为空。

- 2026-10-11 11:40 项目命名文档：`make contract-check` 与 `git diff --check` 通过；人工对照 CLI 当前仅支持帮助/版本、模板 Name/Module 独立校验及名称正则，核对示例参数、默认值、当前目录写入和文档锚点一致。仅修改 README、ROADMAP 与命令契约，不涉及代码或运行配置。

- 2026-10-11 07:35 D3 Redis：`make check`、`make race build`、`make template-redis`、模板契约与 diff 检查通过。八种产物文件/依赖裁剪与清单不变，四种 Redis 组合完成 readonly 测试/vet/race/lint 和 Linux amd64 纯 Go 构建。固定摘要真实 TLS Redis 对 SQLite、无数据库完成无跳过验收：缓存命中、TTL、隔离、坏值、失效、取消/deadline、后续池复用、未知 CA/主机名/错误密码拒绝、可选停机回源、必需启动/就绪/读写 503 阻断、恢复及日志脱敏通过。受控 RESP2 连接覆盖运行中取消；修正 socket deadline 与 Context 定时器竞争、内部缓存预算误映射 504。复现 Docker 随机端口重启会改变，隔离夹具改用随机选择的固定映射并检查一致性。专用 Redis 标签下容器为零，仅清理本次临时凭据与依赖准备产物，保留共享镜像缓存。

- 2026-10-11 07:05 D3 无数据库：最终 `make check`、`make race build`、`make template-container` 与模板契约检查通过；四种产物均通过 readonly 测试/vet/race/lint，清单不变。无数据库依赖树与文件裁剪、本机及 Linux amd64 纯 Go 构建、JWT 两主体、UTF-8/控制字符/长度/未知与重复参数、Context、非法配置、迁移命令无写入、TLS/CORS、JSON 日志脱敏与重启/退出通过。实际无数据库容器验证非 root、只读根文件系统、无卷与 JWT；默认容器迁移/CRUD/持久化回归通过。定位新二进制首次执行约 4.99s、再次非法命令约 9ms，测试先检查无写入帮助，再按原 3s 门槛验证失败退出。专用测试标签下容器、镜像与卷只读检查全部为零。

- 2026-10-11 06:49 D3 SQL 数据库：`make check`、`make race build`、相关集成测试/vet/lint 通过；三个变体的 readonly 清单、直接依赖裁剪、文档与 Swagger 通过，MySQL/PostgreSQL 最新模板完成 Linux amd64 纯 Go 构建。最终 `make template-db` 使用固定摘要 MySQL 8.4.11、PostgreSQL 18.6，所有外部用例无跳过；迁移重放、空库只读检查、实际 schema 不匹配、唯一/大小写/Unicode、零值与相同值更新、跨仓储回滚、运行中 deadline 与随后池复用、启动失败关闭、HTTP 主体隔离、重启持久化及日志脱敏通过。真实 TLS 验证受信 CA 成功、未知 CA/主机名不符/错误密码/生产明文拒绝；定位并修正 MySQL Charset 接入及握手排序规则问题。最终专用容器、卷与网络只读检查均为零。

- 2026-10-11 01:21 D2 最终验收：`make check`、`make race build`、`make template-container` 通过；独立项目全部使用 readonly 且清单未变化，文档链接、Swagger 2.0 本地引用、直接依赖与镜像摘要核对通过。真实进程验证未知配置/秘密缺失/未迁移/TLS 无效/端口占用/生产开发命令拒绝，可信证书链与精确 CORS 通过；复现 TLS 失败绕过 JSON 日志后修复，所有 TLS 日志可解析为 JSON。最终实际容器验证非 root、显式迁移重放、JWT/CRUD 和重启持久化；专用标签只读检查容器、卷及测试镜像均为零。
- 2026-10-11 01:10 D2 错误与请求边界：`make check` 通过；独立项目测试、vet、race、lint 与实际 HTTP 全链路通过。cause 可追溯且 Error 文本脱敏，请求预算返回 504，已有无效配置及符号链接均拒绝覆盖。
- 2026-10-11 01:00 D2 配置类型修复：先复现数字 issuer 被接受，再修正并通过 `make check`；生成项目测试、vet、race、lint 和真实 HTTP 回归通过，文件/环境/显式覆盖行为保持一致。
- 2026-10-11 00:58 D2 示例 CRUD：`make check` 通过，独立项目测试、vet、race、lint 全通过；两个实际服务进程验证认证、CRUD、跨主体读/改/删 404、同主体唯一冲突 409、分页/参数边界、零值更新、非法 JSON 与重启持久化；日志未包含测试 Token、密钥、正文或 SQL 数据。同事务两个仓储回滚、取消与 deadline 传播通过。
- 2026-10-11 00:52 D2 JWT：`make check` 通过，独立渲染项目测试、vet、race、lint 通过；算法/签名/issuer/audience、缺失声明、到期与未来时间、换行和超长输入全部拒绝；重复认证头、主体传播、签发参数和生产禁用回归通过。
- 2026-10-11 00:49 D2 SQLite：`make check` 通过，渲染项目测试、vet、race、lint 全通过；真实文件覆盖特殊字符路径、0600 权限、三连接 PRAGMA、空库检查不建表、迁移重放、取消请求、关闭后持久化及日志脱敏。独立二进制初始化与迁移命令通过；缺 schema、非法 TLS、端口占用及失败后池关闭回归通过。
- 2026-10-11 00:46 D2 HTTP 服务基础：`make check` 通过；渲染独立项目测试、vet、race、lint 全部通过，真实回环测试验证关闭请求排空与预算到期强制取消；开发配置权限及密钥保留、panic/Token/查询日志脱敏通过。
- 2026-10-11 00:36 D2 配置与日志：`make check` 通过；集成测试实际渲染独立模块、补齐固定依赖后执行 readonly 测试与 vet。配置回归覆盖覆盖优先级、环境独立启动、显式文件缺失、未知键、无单位时长、pprof 限制及密钥错误脱敏。
- 2026-10-11 00:36 模板元数据：`make test vet GO_PACKAGES=./internal/templates` 通过；回归覆盖 `.DS_Store`、`Thumbs.db` 不进入文件计划及 `.gitignore` 输出保留。
- 2026-10-11 00:30 D2 模板渲染：`make check` 通过；单元测试覆盖确定性、变量展开、非法名称与模块注入、缺失变量、语法和格式错误，失败不返回部分计划。
