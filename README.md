# goseed

面向中小型 Go 后端业务的项目脚手架与业务代码生成器项目。

## 当前状态

Go 工具链固定为 **1.26.4**，主模块为 **`github.com/setcreed/goseed`**。仓库默认分支为 `main`。

根命令已生成独立业务项目：在空目录执行 `goseed` 默认生成 HTTP + SQLite + JWT 项目；`--name` 和 `--module` 可分别指定项目名与模块路径，数据库、协议、Redis 和 Casbin 可通过参数选择，`--dry-run` 只展示计划并检查冲突。目录名默认作为项目名，写入前拒绝非空业务目录，不覆盖已有文件；生成的 `goseed.yaml` 记录模板版本和能力选择，业务服务不依赖此元数据。`goseed gen module <name>` 已生成行为骨架并更新模块清单、协议登记和 OpenAPI 索引；CRUD 资源命令 `gen resource` 尚未实现。模块骨架不含业务操作，生成后需开发者补齐领域逻辑和接口。完整参数、默认值和限制见 [命令契约](docs/cli.md) 。开发检查点以 [ROADMAP.md](ROADMAP.md) 为准。

内部模板渲染入口为 `internal/templates.Default` 与 `Render(Project, Options)`，返回校验后的文件计划，展开项目名称及模块路径，并格式化 Go 源码。`Project.Name` 与 `Project.Module` 分别指定项目名和 Go 模块路径，二者独立校验，不要求名称与模块路径末段相同。渲染不联网、不直接写入文件。

内部 `generator.PrepareResource` 已提供只读资源目标计划：校验 spec、当前组装入口、协议工具、迁移版本和文件冲突，预览输出目标，并在输入变化后拒绝旧计划。`SchemaStore()` 返回只读表结构核验源码，选择迁移时 `MigrationDraft()` 返回当前数据库方言 SQL 草稿和下一版本；均返回内容副本，不写文件、不连接数据库。该入口尚不渲染 CRUD、不提供 Write，也未接入公共 CLI；接口与接入边界见 [资源接入方案](docs/resource-generation.md) 。

SQL 模板从嵌入迁移推导期望版本，启动只读核对完整已应用集合；显式迁移拒绝未知版本和中间断档。资源 schema 原型检查保留列、类型、非空、单列自增主键、长度与 owner 排序规则；启动组装向模块工厂提供限时 Context。人工 schema 需遵循草稿结构，检查不推断任意 SQL 等价，也不自动修表。MySQL DDL 失败可能保留部分 schema，需审阅后处理。

模块工厂通过 `<name>Permissions()` 纯函数声明协议权限；启用 Casbin 时，启动和离线策略导入共用 `bootstrap.PermissionCatalog()`，显式接受已登记业务资源并默认拒绝未知权限。组装入口已升级到版本 2；旧项目缺入口或纯描述时拒绝继续生成，需要人工升级。使用与边界见 [授权契约](docs/template-options.md#casbin-可选能力) 。

| 模板能力 | 当前支持 | 默认值 |
|---|---|---|
| `Options.Database` | `sqlite`、`mysql`、`postgres`、`none` | `sqlite` |
| `Options.Redis` | 开启或关闭 Redis 缓存 | `false` |
| `Options.Authz` | `none`、`casbin` | `none` |
| `Options.Transport` | `http`、`grpc`、`dual`、`gateway` | `http` |
| 身份认证 | 所有当前模板均包含 JWT，Render 没有认证裁剪选项 | JWT |

四种存储 × Redis 开关 × Casbin 开关 × 四种协议模式共 64 种组合。按选项裁剪源码、驱动、配置、迁移、依赖和文档；根命令支持这四个维度的生成参数，JWT 为固定基础能力。业务项目中的 `apiserver serve` 监听参数见 [命令契约](docs/cli.md#服务开发与运行入口) 。

默认 HTTP 模板包含 Handler → Biz → Store 分层、SQLite、JWT、示例 CRUD、配置与日志、健康检查和优雅退出。提供开发密钥初始化、开发 Token 签发、显式迁移、严格输入校验、资源主体隔离、TLS 与 CORS；没有公开的临时签发接口。服务命令与验收边界见 [默认 HTTP 模板](docs/template-http.md) 。

MySQL、PostgreSQL 变体复用相同业务分层与 JWT，提供强类型连接配置、连接池、显式方言迁移、UTC 时间、默认校验证书的数据库 TLS；生产拒绝关闭数据库 TLS。配置和真实服务验收入口见 [可选模板](docs/template-options.md) 。

无数据库变体保留所选协议、JWT、配置、日志和生命周期，提供 Handler → Biz 的认证问候接口，移除模型、Store、ORM、SQL 驱动、迁移命令与数据库配置。没有持久化或资源 CRUD，详细行为见 [可选模板](docs/template-options.md) 。

Redis 变体提供按主体隔离的示例读取缓存、TTL、写后失效、连接池与生命周期，默认验证 TLS。可选模式在故障时回源，必需模式在启动、就绪和业务入口检查依赖；生产要求密码与验证证书。缓存与数据库之间按 TTL 保证最终一致性，强一致读取应绕过缓存；配置和边界见 [可选模板](docs/template-options.md) 。

Casbin 变体提供默认拒绝的简单 RBAC，业务路由在 JWT 之后、缓存和 Biz 之前授权，资源归属仍由 Biz 校验。用户／角色名称隔离，权限精确匹配资源与操作；默认策略为空，不授予任何用户权限。SQL 复用现有连接并显式迁移策略表，提供 `policy replace --file <csv>` 全量事务导入；无数据库只读加载策略文件，配置 `authz.policy_file`。离线更新后需重启所有实例，运行接口不提供策略管理 API。输入约束、权限和快照边界见 [可选模板](docs/template-options.md#casbin-可选能力) 。

纯 gRPC 变体复用相同 Biz、Store、JWT、Redis 和 Casbin，提供 SQL CRUD／无数据库问候的 Proto、已生成代码与示例客户端。业务方法经过 JWT 和可选授权，保留 optional 零值更新、资源归属、请求 ID、取消与 deadline，错误和日志脱敏。标准 Health Check／Watch 支持依赖故障与恢复，退出先停止就绪再有限期关闭；生产必须 TLS，Reflection 仅开发回环且要求 JWT。移除 Gin、业务 HTTP 配置与 Swagger。协议目录、配置、输入和验证边界见 [纯 gRPC 模板](docs/template-grpc.md) 。

dual 变体同时提供 Gin HTTP 与 gRPC，两种 Handler 共用同一个 Biz、JWT、可选缓存与授权；启用数据库时共用 Store 与连接池。`serve --address` 覆盖 HTTP，`serve --grpc-address` 覆盖 gRPC；先绑定双端口，任一启动失败回收资源，任一服务返回则停止两侧就绪并并发排空，全部退出后关闭共享依赖。保留两种接口契约和生产 TLS 规则，详见 [双协议模板](docs/template-dual.md) 。

Gateway 变体通过真实 RPC 转发 HTTP，共用 gRPC Handler 与安全链，裁剪 Gin 和 HTTP 业务 Handler。交付固定 HTTP 映射、ProtoJSON、已生成 Gateway 与 Swagger、内部 RPC 证书验证、启动失败回收和先 HTTP 后 RPC 的顺序排空；64 位整数输出字符串，成功方法统一返回 200。配置与输入边界见 [Gateway 契约](docs/template-gateway.md) 。

渲染项目包含完整 `go.mod` / `go.sum`、AGENTS/README/ROADMAP、Makefile、配置示例和非 root Dockerfile；HTTP 提供 Swagger 2.0，gRPC 提供 Proto 与固定版本显式生成入口，dual 同时提供两者，Gateway 从 Proto 和固定 HTTP 映射生成 Swagger。按所选数据库提供 SQL 迁移，可以独立构建及运行。

`make test` 实际渲染独立项目，执行 readonly 测试、vet、race、lint 与构建，并通过实际服务进程验证初始化、迁移、JWT/CRUD、资源隔离、非法输入、TLS/CORS、启动失败、日志脱敏和重启持久化。gRPC、dual 与 Gateway 各 16 种组合均编译测试包、交叉构建服务与客户端；SQLite／无数据库的基础组合与 Redis + Casbin 组合执行完整测试、实际 RPC、TLS、Proto 脚本重新生成比对和独立 Go 客户端模块导入验证。dual 额外检查跨协议数据、输入与权限语义、双端口失败回收和同时排空；Gateway 额外检查转发身份、ProtoJSON 输入、内部 TLS、HTTP 预算／取消及先 HTTP 后 RPC 排空。需先 `make tools`、`make protocol-tools`，同时需要本机回环监听。64 种组合的总测试时限为 40 分钟，单条验证命令和启动失败门槛保持不变。

## 构建与检查

需要 Go；首次构建会下载 Go 1.26.4 工具链及固定依赖。以下命令在仓库根目录执行：

```sh
make build
./bin/goseed --help
./bin/goseed --version

# 在空业务目录内执行；可先用 --dry-run 查看输出清单
goseed --name shop --module github.com/acme/shop --dry-run
goseed --db=postgres --redis --authz=casbin --transport=dual

make tools
make protocol-tools
make check
make race

# 显式启动本次专用容器验证，需要 Docker
make template-container
make template-db
make template-redis
```

`make tools` 显式安装固定版本 linter 到 `.tools/bin/`。构建与检查使用项目 `.tmp/` 缓存，不安装全局工具，不自动改写已交付源码或依赖清单。`template-container` 实际构建默认与无数据库业务镜像，使用随机回环端口；默认模板用全新数据卷验证生产禁用开发签发、显式迁移、CRUD 与重启持久化，无数据库模板验证非 root、只读根文件系统、无数据卷、JWT 与重启；Casbin 文件策略镜像另验证默认拒绝、只读策略挂载和角色冒充拒绝。gRPC／dual／Gateway 无数据库 + Casbin 镜像验证生产 TLS、非 root、只读根文件系统、证书／策略只读挂载、健康调用、权限与重启；dual／Gateway 同时验证两个公开端口，Gateway 验证内部 RPC 的证书信任。结束清理本次容器、卷及测试镜像，普通检查不启动 Docker。

普通检查同时实际渲染 MySQL、PostgreSQL 项目，执行 readonly 测试、vet、race、lint 与构建；没有专用服务时明确跳过其中的外部数据库用例。`template-db` 使用固定摘要的真实 MySQL 8.4.11、PostgreSQL 18.6，全新测试证书/数据卷、随机回环端口和隔离数据库；外部用例禁止跳过，依次验证 SQL 基线、Casbin v1 → v2 迁移升级，以及 gRPC／dual／Gateway + Casbin 实际调用。追加迁移用例验证可选 Casbin 与两个资源表的顺序升级、schema 不兼容、owner 排序、Unicode 长度、32 字段宽表、失败迁移及重启；SQLite 对应用例由 `TestMigrationVersionProjects` 执行。覆盖 CRUD、权限、事务与策略回滚、取消／连接复用、启动失败、日志脱敏、数据库 TLS 信任／主机名及重启持久化。结束仅清理本次资源，需要 Docker、Compose 与 OpenSSL。

普通检查也实际渲染无数据库项目，核对文件与依赖树裁剪、固定清单不变，执行 readonly 测试、vet、race、lint、本机构建与 Linux amd64 纯 Go 构建；真实服务进程覆盖初始化、迁移命令拒绝、JWT、输入边界、TLS/CORS、日志脱敏、重启和优雅退出。

普通检查验证四种存储与 Redis 的组合、缓存命中/主体隔离/失效、受控连接取消和短超时；实际 Redis 用例在缺少专用凭据时明确跳过。`template-redis` 使用固定摘要的 Redis 8.10.2、专用密码与 TLS 证书、随机回环端口，验证 SQLite 与无数据库缓存、TTL、TLS 信任、错误密码、必需／可选故障行为及停机后恢复，禁止跳过真实用例。需要 Docker 与 OpenSSL，退出只清理本次容器与临时凭据，保留依赖镜像缓存；不清空共享 Redis 数据。

普通检查覆盖 Casbin 的八种 HTTP 开启组合及原有关闭组合，验证 readonly 清单、Linux 纯 Go 构建、策略输入、并发快照、取消、默认拒绝、授权与归属隔离、错误导入保留旧策略及撤销。`template-redis` 同时验证 HTTP／gRPC／dual／Gateway 的 Casbin 组合，权限撤销并重启后，Redis 中仍存在的缓存不能绕过授权；gRPC 验证可选回源、必需模式 Unavailable、健康 NOT_SERVING、恢复与日志脱敏，dual／Gateway 验证两侧共同执行故障与授权规则。

`goseed` 生成命令只写入当前目录，不执行 Git、依赖安装、迁移、容器或服务。目标目录可保留 `.git`、`.DS_Store` 和 `Thumbs.db`；其他已有内容均作为冲突拒绝生成。dry-run 不写入文件，输出完整文件计划、依赖版本与冲突说明。

进入已生成项目目录后，可用 `goseed gen module orders --dry-run` 检查新模块计划，再执行 `goseed gen module orders` 写入。命令拒绝覆盖已有业务文件；登记文件更新失败时回滚本次新增文件和此前已更新的索引。模块 Handler 当前不注册虚构业务端点，Store 仅为持久化边界骨架，不代表可运行 CRUD。

模块组装位于首次生成的 `internal/apiserver/bootstrap/module_orders.go`，由开发者维护构造逻辑、HTTP 操作权限、RPC 权限与 Gateway 映射。固定启动入口把共享数据库、日志及所选缓存交给模块工厂；每个模块只构造一次，dual 两种 Handler 共用同一 Biz。新增模块不改写 `providers.go` 或已有工厂；缺少组装入口的旧模板项目在生成前报不兼容，需显式更新入口。入口缺失、登记索引内容被改写或计划准备后组装文件发生变化时，命令拒绝写入。

## 技术兼容性验证

固定版本的业务运行依赖在 `tests/compatibility/` 的独立模块中核验。主模块的直接依赖为 Cobra 和生成期 YAML 解析库，不引入业务运行时框架。

```sh
make compatibility
make compatibility-cross
make compatibility-db

make protocol-tools
make protocol-check
```

兼容性测试需要本机回环监听；`compatibility` 在没有外部数据库配置时明确跳过 MySQL/PostgreSQL。`compatibility-cross` 以纯 Go 方式交叉编译 Linux amd64/arm64 和 Windows amd64 测试程序。检查脚本需要 Python 3.9+；`compatibility-db` 还需要 Docker 与 Compose，会启动本次专用数据库、随机回环端口和全新卷，退出时清理本次容器与卷；不使用已有数据库。

协议测试使用已生成源码，无需 protoc。`protocol-tools` 显式安装固定版本工具到 `.tools/`；`protocol-check` 校验版本并重新生成到临时目录，比对提交范围内的验证产物。依赖选型、许可证、维护证据与测试边界见 [依赖核验](docs/dependencies.md) 。

验证程序覆盖以下范围，验证结果统一记录在 ROADMAP：

| 对象 | 环境与检查范围 |
|---|---|
| 业务模块生成与组装 | 四协议/四存储生成项目；单次构造、共享 Biz/数据库、保留手工工厂、版本/纯声明校验、冲突与失败回滚 |
| 只读资源目标计划 | 48 种 SQL 组合、16 种无数据库拒绝组合；spec/工具/迁移/路径校验、预览确定性、输入快照和副本隔离；不渲染或写入 CRUD |
| 模块权限目录 | 四协议/四存储生成项目；纯目录不构造业务、工厂声明一致、默认拒绝、启动与离线导入共用、目录快照隔离 |
| SQLite | 真实磁盘文件；多连接 PRAGMA、迁移、约束错误、事务回滚、Context、锁等待和关闭连接后读取 |
| MySQL 8.4.11、PostgreSQL 18.6 | 固定镜像摘要的真实数据库服务；显式迁移、Casbin 策略保存与重载、权限判定、删除和保存失败回滚 |
| Casbin 正式模板 | 三种数据库策略与无数据库只读文件；默认拒绝、主体/角色命名空间、权限与资源隔离、事务回滚、撤销、并发快照和请求 Context |
| Casbin 适配原型 | D1 独立兼容性验证，未作为正式模板实现复用 |
| gRPC 正式模板 | 16 种组合裁剪／构建；代表项目的真实调用、安全链、取消、健康 Watch、TLS、退出、真实 SQL／Redis、生产容器与 Proto 复现 |
| dual 正式模板 | 16 种组合编译／构建；代表项目共享业务、跨协议调用与权限、双端口失败清理、并发排空、真实 SQL／Redis、生产双端口 TLS 容器 |
| Gateway 正式模板 | 16 种组合编译／构建；真实 RPC 转发、输入与身份、Proto／Gateway／Swagger 复现、顺序排空、真实 SQL／Redis、双端口 TLS 容器 |
| gRPC、Gateway 兼容性原型 | D1 本机回环实际调用和转发、Swagger 2.0 产物解析、固定工具重新生成比对 |
| Redis 8.10.2 | 专用 TLS 服务；缓存读写/隔离/TTL、请求取消、证书与密码拒绝、故障回源、必需依赖阻断、恢复和关闭 |
| 其他运行依赖 | 配置读取、日志与 DI 组装、JWT 签发与解析 |

纯文档改动可使用 `make contract-check` 检查模块、工具版本、本地链接与文件格式。项目开发及功能完成后的 Git 提交流程见 [项目规范](AGENTS.md) 。

## 文档入口

- [项目规范](AGENTS.md)：目录职责、工程边界、工作流与验证要求。
- [产品需求](docs/requirements.md)：使用场景、功能范围和验收标准。
- [技术架构](docs/architecture.md)：分层、目录、技术选型、配置、日志和服务生命周期。
- [命令契约](docs/cli.md)：默认行为、参数、能力组合和业务生成约束。
- [实施方案](docs/implementation.md)：开发顺序、检查点及验证方式。
- [依赖核验](docs/dependencies.md)：固定版本、官方来源、维护情况与兼容性约束。
- [默认 HTTP 模板](docs/template-http.md)：开发初始化、迁移、JWT、配置、示例业务与运行契约。
- [可选模板](docs/template-options.md)：数据库裁剪、Redis 缓存、Casbin 策略及真实服务验收。
- [纯 gRPC 模板](docs/template-grpc.md)：协议目录、配置、安全链、健康服务、退出与验收。
- [双协议模板](docs/template-dual.md)：共享依赖、独立配置、双端口启动、并发退出与验收。
- [Gateway 模板](docs/template-gateway.md)：真实 HTTP 转 RPC、ProtoJSON、证书验证、顺序退出与验收。
