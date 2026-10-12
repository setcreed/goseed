# 项目进度

## 当前状态

文档基线、D1、D2、D3、D4（HTTP、纯 gRPC、dual、Gateway）与 D5 项目生成器已完成。四种协议均支持四种存储、可选 Redis 与 Casbin，共 64 种组合；MySQL、PostgreSQL、Redis 已完成真实服务验证，Gateway 生产容器验收通过。根命令已支持当前目录生成、能力选择、结构化版本化元数据和 dry-run；业务模块与 CRUD 生成属于 D6。HTTP、dual 与 Gateway 初始项目现在同时生成 `example.yaml` OpenAPI 源及规范化汇总索引；生成器可校验项目状态，并为已登记源变化安全刷新汇总索引与摘要。首次基线提交为 `01c8d6c`，本地分支为 `main`，未配置远端。主模块为 `github.com/setcreed/goseed`，工具链固定为 Go 1.26.4。

纯 gRPC 已按 [协议契约](docs/template-grpc.md) 交付共享业务层、安全链、健康服务、TLS、示例客户端与公开 `pkg/api` 协议类型；dual 按 [双协议契约](docs/template-dual.md) 共享同一 Biz 与依赖，提供独立监听覆盖、启动失败回收和并发排空。Gateway 按 [Gateway 契约](docs/template-gateway.md) 将 HTTP 请求转发到真实 RPC 服务，复用认证、授权和业务层，并先排空 HTTP 再关闭 RPC。`Options.Transport` 支持 `http`（默认）、`grpc`、`dual` 与 `gateway`。Redis 与 Casbin 默认关闭，分别通过内部 Render 的 `Options.Redis`、`Options.Authz` 选择。缓存按 TTL 最终一致；Casbin 默认拒绝，SQL 策略显式迁移与事务导入，无数据库策略只读，离线更新后重启实例。项目生成命令以 [命令契约](docs/cli.md) 为准；D6 模块命令已可生成行为骨架并更新结构化清单、四协议登记和 OpenAPI 汇总，资源 CRUD 生成仍待实现。HTTP/dual 路由及四种协议的 RPC Service/权限、Gateway HTTP 映射已接入登记索引；OpenAPI 合并、源摘要校验和索引安全刷新已实现。需求以 [产品需求](docs/requirements.md) 为准，实施顺序见 [实施方案](docs/implementation.md) ，默认模板契约见 [默认 HTTP 模板](docs/template-http.md) ，变体契约见 [可选模板](docs/template-options.md) 。

## 进行中

D6 进行中：`gen module` 通过一次性创建的模块工厂显式构造 Biz/Handler，SQL Store 接收应用数据库，dual 两侧共享同一 Biz；固定启动入口传递强类型共享依赖，新增模块保留已有手工组装代码。RPC 权限绑定实际模块名，重复 FullMethod 拒绝启动。生成前校验启动入口、完整登记投影和既有工厂，计划过期或写入失败时拒绝/回滚。资源字段 YAML 的严格版本 1 解析器已实现。

D6 剩余任务按 [详细施工清单](docs/implementation.md#d6-剩余施工清单) 执行。D6-R01 已定稿 [资源接入方案](docs/resource-generation.md) ，明确 ResourcePlan、纯权限描述、嵌入 SQL 版本推导、资源 schema 核验、Redis 最小依赖检查和多模块协议/文档复现。下一项为 D6-R02，只交付内部只读目标规划、输入快照与预览，不提前开放公共资源命令；R02 至 R10 尚未完成。

## 待开始

| 检查点 | 交付 | 依赖 |
|---|---|---|
| D7 | 首版覆盖核验和使用文档 | D6 |

所有检查点均需完成对应验收后才能标记完成。

## 待核验与待确认

- 跟踪：go-redis 的纯取消 issue 仍开放；当前明确短命令预算、关闭命令与拨号重试，受控连接及真实服务取消已通过，但不承诺瞬时中断或全部取消竞争已消除。官方证据见 [依赖核验](docs/dependencies.md) 。
- 待核验：pgx 缓冲响应与取消竞争的人工延迟 Write 场景；基础运行中 deadline 与随后池复用已通过，未声明解决上游报告。官方证据与版本边界见 [依赖核验](docs/dependencies.md) 。
- 待确认：goseed 自身许可证；仅在公开发布前必须确定。主模块已确定为 `github.com/setcreed/goseed`，Go 工具链固定为 1.26.4。
- 待实现：将严格资源 spec 解析接入 `gen resource` dry-run 计划，生成四协议 CRUD、显式迁移草稿并完成主体隔离验证。当前 `gen module` 生成的是无业务操作的结构骨架；Gateway HTTP 映射及 query/body 输入规则继续由模块登记描述驱动。
- 资源接入边界：现有 Casbin 策略校验只接受内置示例命名空间，迁移期望版本由模板固定，协议脚本仅处理内置模块，Gateway 复现会用单模块输出覆盖汇总；分别在 D6-R03、R04、R07/R08 解决，不能仅新增 CRUD/SQL 文件。缺少兼容入口的旧模板项目明确拒绝生成，不自动改写手工 Bootstrap；具体接口与兼容规则在 R01 定稿。

## 最近完成

- 2026-10-12 08:53 D6-R01 接入决策：定稿资源计划 API、各协议输出路径和文件归属、CRUD 输入/响应、纯模块权限描述与启动/离线共用目录、迁移期望版本及只读 schema、无自动缓存的 Redis 检查、多模块协议和 Gateway 聚合复现；保留 Go 1.26.4 与现有依赖。同步架构、命令、施工清单和 AGENTS 入口，当前方案未实现为运行能力，下一项为 R02。

- 2026-10-12 08:43 D6 详细施工清单：在实施方案中拆分 D6-R01 至 R10，记录目标、前置依赖、修改范围、文件归属、失败路径、验收与独立提交边界；明确权限目录、迁移版本/schema 和多模块协议复现的前置问题，以及 48 种资源合法组合、16 种无数据库拒绝组合和 64 种原项目组合回归。同步 AGENTS 入口与需求中的 Swagger 更新权限，清理 D6 同时列为进行中/待开始的重复状态。资源开发未启动，下一项为 R01。

- 2026-10-12 08:30 D6 模块依赖组装：静态 Bootstrap 通过强类型共享依赖调用模块工厂，工厂文件只生成一次并由开发者维护；SQL Store 复用应用数据库，dual 两侧共享 Biz，构造错误传播并回收共享资源。生成索引汇总手工 HTTP/RPC/Gateway 描述，RPC 授权绑定所属模块，重复 FullMethod 拒绝启动。生成前拒绝旧入口、改写登记索引和过期工厂输入，不增加依赖、不改写手工 Bootstrap。

- 2026-10-12 07:37 D6 资源字段规范解析：新增严格版本 1 YAML 解析器，校验 UTF-8、单文档、重复/未知键、字段数量与命名、保留名、类型、required、max_length 和默认值；拒绝空值、别名、锚点、合并键及自定义标签。作为 `gen resource` 计划的输入基础，不单独写入业务项目。

- 2026-10-12 07:27 D6 `gen module`：新增当前项目模块生成命令，按四种协议渲染登记索引并生成 Biz/Handler 骨架；SQL 选项生成仓储/Store 扩展点，无数据库不生成 Store。合并模块 OpenAPI 源，dry-run 显示新增、更新和冲突；多文件写入前重验输入与目标，更新失败回滚登记索引和本次新增文件。资源 CRUD 命令仍待实现。

- 2026-10-12 07:01 D6 模块清单摘要校验：四种协议模板登记索引新增模块清单摘要；项目加载校验清单与登记摘要一致，手工改变摘要或清单时拒绝继续。

- 2026-10-12 06:58 D6 结构化模块元数据：新项目 `goseed.yaml` 使用带 `name/type` 的登记项，资源项校验 `specs/` 相对路径及字段规范版本；读取旧版字符串条目时兼容解释为普通模块。

- 2026-10-12 06:49 D6 OpenAPI 安全刷新计划：新增 `PrepareOpenAPIRefresh`，准备阶段不落盘，执行前重新验证项目清单、源文件、登记摘要和目标原始字节；Swagger 与登记摘要采用同目录临时文件原子替换，多文件中后续写入失败时回滚先前替换。本能力当前是生成器内部 API，尚未接入业务生成 CLI。

- 2026-10-12 06:39 D6 项目状态校验：HTTP、dual 与 Gateway 新项目新增内置 `example.yaml` OpenAPI 源，Gateway 协议复现同步更新该源；新增 `generator.LoadProject`，严格校验 `goseed.yaml`、模块源登记和现有索引摘要，计算当前源的规范化索引并标出待更新状态，拒绝未登记 YAML 源及纯 gRPC 项目的 HTTP OpenAPI 文件。四种协议状态校验和关键篡改路径定向测试通过；

- 2026-10-12 00:02 D6 OpenAPI 合并器：新增生成器专用 YAML 依赖和多源 Swagger 2.0 合并器，按模块名稳定合并 paths、组件与 tags，拒绝重复键、别名、merge key、自定义标签、重复操作及 operationId，输出规范 JSON 与 SHA-256。初始模板渲染和 Gateway Proto 复现均规范化 Swagger，并把摘要写入登记索引；补齐 dual 专用 HTTP/RPC 合并登记模板。

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

## 最近验证

- 2026-10-12 08:53 D6-R01 文档：`make contract-check`、`git diff --check` 通过；对照现有模块计划/组装、SQL/Redis/Casbin 模板和协议脚本，核对接口、分页/ID、迁移命名、固定工具版本与 YAML 依赖已有声明。仅形成技术决策，未运行 Go/真实服务测试，未创建临时产物。

- 2026-10-12 08:43 D6 施工清单文档：`make contract-check` 通过（69 个本地链接、331 个交付文件），`git diff --check` 通过；人工对照现有源码、命令/架构契约和 Makefile，核对任务依赖、索引归属、组合数及真实服务验收入口。仅修改四份文档，未开发资源命令、未重跑 Go 或真实服务测试；本次未创建任务临时目录、缓存或编译产物。

- 2026-10-12 08:30 D6 模块组装：16 种协议/存储组合通过生成项目验收，SQLite 与无数据库 8 种组合执行完整 readonly race 测试、vet、lint；MySQL/PostgreSQL 8 种组合执行 readonly 编译、指针组装测试、vet、lint，本轮未重跑真实外部数据库。Redis + Casbin 的四协议/两存储共 8 种代表组合通过 readonly 编译、vet、lint，未新增真实 Redis 验收。覆盖共享 Biz、真实 SQLite 指针与池生命周期、工厂构造失败、手工组装保留、RPC 模块授权、重复 FullMethod 拒绝；RPC 权限修正后重跑受影响 12 种协议/存储组合通过，重复 RPC 登记启动失败后池回收也通过三协议 SQLite race 回归。定向包测试、全仓 vet、修改包 lint（0 issues）、契约检查（65 个本地链接、331 个交付文件）、格式及 diff 检查通过。首次夹具生产 TLS 配置和注入变量 lint 失败已修正并重跑；本次隔离项目已清理，保留原有共享缓存；完整旧模板运行矩阵未重跑。

- 2026-10-12 07:37 D6 资源规范解析：`go test ./internal/generator ./internal/cli ./internal/templates` 与对应 `go vet` 通过；覆盖默认值/字段顺序、无效 UTF-8、多文档、重复及未知键、字段名/类型/保留名、空值、别名/锚点/合并键/自定义标签、max_length 范围和字段数上限。复用项目已有 `.tmp` Go 缓存。

- 2026-10-12 07:31 D6 `gen module`：生成 HTTP、gRPC、dual、Gateway 四种协议的 SQLite 与无数据库代表项目，均执行 `go test -mod=readonly ./...` 通过；覆盖 dry-run 无写入、已有文件冲突、过期计划拒绝、更新失败后回滚和 Go 关键词模块名。定向生成器/模板/CLI 测试、`go vet ./...`、核心包 lint（0 issues）、`make contract-check`（68 个本地链接、326 个交付文件）、`make fmt-check` 与 `git diff --check` 通过。首次测试路径误用缓存触发网络下载失败，修正到 `.tmp` 缓存后测试通过；沙箱拒绝回环监听时只运行编译检查，获准回环验证后八个代表项目完整测试通过。清理本次 `.tmp/d6-module-smoke-20261012`、`.tmp/d6-module-smoke-final` 和误建根目录缓存。

- 2026-10-12 07:01 模块清单摘要校验：`go test ./internal/generator ./internal/cli ./internal/templates`、`go vet ./...`、核心包 `golangci-lint`（0 issues）通过；覆盖四协议项目加载、篡改摘要拒绝和篡改模块清单拒绝。验证复用项目已有 `.tmp` Go 缓存。

- 2026-10-12 06:58 结构化模块元数据：`go test ./internal/generator ./internal/cli ./internal/templates`、`make contract-check`（68 个本地链接、324 个交付文件）、`make fmt-check` 与 `git diff --check` 通过；覆盖结构化模块/资源项、旧版字符串兼容、未知字段和 spec 路径/版本拒绝。使用项目已有 `.tmp` Go 构建与模块缓存。

- 2026-10-12 06:49 OpenAPI 安全刷新：生成器、OpenAPI、模板和 CLI 包测试通过，覆盖 dry-run 无写入、过期计划拒绝及第二个文件替换失败后回滚；`make vet`、核心包 lint（0 issues）、`make contract-check`（68 个链接、324 个交付文件）、`make fmt-check` 和 `git diff --check` 通过。

- 2026-10-12 06:47 D6 项目状态校验：定向测试、`go vet ./...`、契约检查、格式检查和核心修改包 lint（0 issues）通过；集成包编译通过。Gateway SQLite 与无数据库代表项目均完成 Proto／Swagger／`example.yaml` 复现，SQLite 代表用例通过；默认 HTTP 与 Gateway 无数据库用例在服务 SIGTERM 排空处超时（重跑仍复现），不能算集成通过。全仓及 `tests/integration` lint 因 Go 1.26 linter 遇到 Go 1.27 源码而 panic。清理本轮 171 MiB 任务缓存；完整生成项目矩阵尚未通过。

- 2026-10-12 00:02 D6 OpenAPI/登记回归：核心包测试、`make vet`、`make lint`（0 issues）、`make contract-check`（68 个本地链接、321 个交付文件）、`make fmt-check` 与 `git diff --check` 通过。分别运行默认项目、`TestDualProjects/(sqlite-none|none-none)`、`TestGatewayProjects/(sqlite-none|none-none)`；生成项目 readonly 编译、race/vet/lint、本机服务调用通过，Gateway Proto／Swagger 逐字节复现通过。Gateway 首次复现发现模板规范化与 protoc 原始输出格式不一致，已规范化复现脚本并重跑通过。完整 `make test` 矩阵未完成；首次模板测试断言的编译问题已修正，后续全量矩阵被主动停止，不能视为全矩阵通过。验证后清理本次生成的项目和 Go/lint 构建缓存，保留历史 `.tmp/d2/render-3653844838` 与可复用模块缓存。

- 2026-10-11 23:18 D6 文档/格式收尾：`make contract-check`（68 个本地链接、317 个交付文件）、`make fmt-check`、`git diff --check` 通过；本轮专用缓存与集成测试生成目录已清理。

- 2026-10-11 23:17 D6 RPC 登记：`go test ./internal/templates` 通过；在获准的本机回环环境运行 `go test ./tests/integration -run '^Test(GRPCProjects|GatewayProjects)/(sqlite-none|none-none)$'`，SQLite 与无数据库的 gRPC-only、Gateway 代表项目全部通过 readonly 编译、race/vet/lint、Proto 复现和服务回环调用；清理本次测试写入的 28,811 个 Go 构建缓存文件及 132 MiB 隔离缓存，保留历史共享缓存。

- 2026-10-11 23:12 D6 HTTP/dual：`go test ./internal/templates` 与新增 transport-scope 测试通过；在获准的本机回环环境执行 `go test ./tests/integration -run '^Test(DefaultProject|NoDatabaseProject|DualProjects)$'`，三个生成项目集成套件无跳过通过，覆盖 readonly 编译、race/vet/lint、HTTP/JWT CRUD、无数据库入口及 dual HTTP+gRPC 行为。清理本次 132 MiB 隔离 Go 构建缓存；`.tmp/d2` 仅保留任务前已有目录。23:13 `make contract-check`（68 个本地链接、316 个交付文件）、`make fmt-check` 与 `git diff --check` 通过，隔离缓存目录已确认不存在。

- 2026-10-11 23:03 D6 文档与格式回归：`make contract-check`（68 个链接、315 个交付文件）、`make fmt-check`、`git diff --check` 全部通过；确认本次隔离缓存目录已清除。

- 2026-10-11 23:02 D6 登记接口原型：`make compatibility` 首次受沙箱回环监听限制；获准本机重跑后全套兼容性测试通过，包括新增 `TestGeneratedModuleRegistryContract`、Proto 回环测试和 SQLite 行为测试；MySQL/PostgreSQL 兼容性子用例按该目标设计跳过，真实服务仍由 `make compatibility-db` 验证。清理本次隔离 Go 构建缓存。

- 2026-10-11 22:58 D6 契约文档：`make contract-check` 通过（68 个本地链接、314 个交付文件），`git diff --check` 通过；复核命令示例、字段约束、登记文件所有权和 D6 未实现状态一致。

- 2026-10-11 22:55 AGENTS／ROADMAP 文档：`make contract-check` 通过（66 个本地链接、314 个交付文件），`git diff --check` 通过；核对临时文件收尾约定与文档入口。

- 2026-10-11 22:45 D5：`make check` 与 `make build` 通过；`make check` 覆盖全部 Go 包、模板组合集成测试、vet 与 lint（0 issues）。CLI、生成器、模板定向测试、格式、契约链接和 diff 检查通过；构建出的二进制实际运行默认生成与 PostgreSQL + Redis + Casbin + Gateway dry-run，检查默认元数据、依赖版本和无落盘行为。沙箱曾因禁止回环监听拒绝集成用例，获准在本机重跑后全套通过。

- 2026-10-11 22:07 D4 Gateway：`make check` 的根模块集成测试、`make fmt-check`、`make vet`、`make lint`、协议复现与 `make contract-check`／`git diff --check` 通过；新增的 Gateway 包 race 与 IPv4／IPv6 通配监听目标回归通过。`make template-db` 的 MySQL／PostgreSQL Gateway 项目、`make template-redis` 的 SQLite／无数据库及 Casbin 组合均实际运行通过且无跳过；启用 Docker 的 Gateway TLS 容器实际测试通过。Redis 重启后的旧 gRPC 健康恢复用例曾暴露瞬时观察竞态，改为有界 5 秒轮询后完整 Redis 套件通过；没有改变生产服务重试行为。

- 2026-10-11 19:43 文档同步：`make contract-check` 通过版本契约、55 个本地链接、Python 语法与 260 个交付文件检查，`git diff --check` 通过；`make test GO_PACKAGES=./internal/templates` 通过，验证文档模板可渲染及交付链接一致。对照 CLI 源码、Render 选项、模板路径和 Makefile，核对当前默认值、监听参数与 48 种组合；仅改文档及一份生成文档模板，没有重跑数据库或容器，沿用 17:51 的运行验收证据。
