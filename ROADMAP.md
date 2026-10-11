# 项目进度

## 当前状态

文档基线、D1 与 D2 已完成，D3 的 MySQL、PostgreSQL 模板已完成真实服务验证，无数据库变体已完成进程与容器验证，Redis 可选能力已完成四种存储组合检查与真实 TLS 服务验收。首次基线提交为 `01c8d6c`，本地分支为 `main`，未配置远端。主模块为 `github.com/setcreed/goseed`，工具链固定为 Go 1.26.4。三个 SQL 模板与无数据库模板可独立构建，默认 HTTP + SQLite + JWT 及无数据库 HTTP + JWT 的实际进程和容器验证通过。

下一功能为 D3 Casbin。Redis 默认关闭，通过内部 Render 的 `Options.Redis` 选择，缓存按 TTL 最终一致，不用于强一致或权威状态。root CLI 仍仅提供帮助与版本，项目生成写入在 D5 接入。需求以 [产品需求](docs/requirements.md) 为准，实施顺序见 [实施方案](docs/implementation.md) ，默认模板契约见 [默认 HTTP 模板](docs/template-http.md) ，变体契约见 [可选模板](docs/template-options.md) 。

## 待开始

| 检查点 | 交付 | 依赖 |
|---|---|---|
| D3 后续 | Casbin | D2；SQL、无数据库与 Redis 变体已完成 |
| D4 | gRPC、dual、Gateway | D2；相关组合依赖 D3 |
| D5 | 全参数项目生成器、项目命名与 dry-run | D2、D3、D4 |
| D6 | 模块与 CRUD 业务生成器 | D5 |
| D7 | 首版覆盖核验和使用文档 | D6 |

所有检查点均需完成对应验收后才能标记完成。

## 待核验与待确认

- 待核验：D3 的正式 Casbin 策略适配实现、输入约束与请求 Context；D1 原型已在三种数据库通过，不作为业务实现复用。
- 跟踪：go-redis 的纯取消 issue 仍开放；当前明确短命令预算、关闭命令与拨号重试，受控连接及真实服务取消已通过，但不承诺瞬时中断或全部取消竞争已消除。官方证据见 [依赖核验](docs/dependencies.md) 。
- 待核验：pgx 缓冲响应与取消竞争的人工延迟 Write 场景；基础运行中 deadline 与随后池复用已通过，未声明解决上游报告。官方证据与版本边界见 [依赖核验](docs/dependencies.md) 。
- 待确认：goseed 自身许可证；仅在公开发布前必须确定。主模块已确定为 `github.com/setcreed/goseed`，Go 工具链固定为 1.26.4。
- 待细化：D6 的 CRUD 字段声明完整格式与生成登记接口。

## 最近完成

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
- 2026-10-11 00:29 D2 配置与运行契约：确定开发初始化、迁移、Token、配置覆盖、身份与资源归属、CRUD 输入和生命周期验收；默认模板契约提供后续实现入口。
- 2026-10-11 00:16 文档同步：明确目标默认行为与当前 CLI 能力的文档入口；README 补充真实数据库及协议验证程序范围、纯文档检查命令；实施方案改为引用 ROADMAP 状态，补记首次提交并合并重复验证记录。
- 2026-10-11 00:12 Git 工作流：用户授权每个功能完成并通过验证后提交，规则已写入 AGENTS；当前 D0、D1 交付纳入首次基线提交，后续以可独立验收的功能为提交单位。
- 2026-10-11 00:08 D1：确定 Go 1.26.4 与 `github.com/setcreed/goseed`；交付最小 CLI、Make 检查入口、独立兼容性模块、依赖锁及官方维护证据、项目内协议工具、固定摘要的隔离数据库验证。SQLite 驱动选 libtnb，策略存储采用复用现有连接的最小适配方案，文档输出固定 Swagger 2.0。CLI 当前仅支持帮助和版本，未实施业务生成。交付：[README](README.md) 与 [依赖核验](docs/dependencies.md) 。
- 2026-10-10 23:16 D0：初始化 `main` 分支，建立项目规范、需求、架构、命令契约、实施方案和文档导航；默认 SQLite/JWT，三种数据库、Redis、Casbin 与四种协议进入首版范围。交付入口：[README](README.md) 与 [实施方案](docs/implementation.md) 。

## 最近验证

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
- 2026-10-11 00:29 D2 契约：`make contract-check` 通过 22 个本地链接、版本契约及 35 个交付文件格式；对照现有需求与架构，业务生成 CLI 保持 D5 范围。
- 2026-10-11 00:16 文档更新：`make contract-check` 通过模块/工具契约、20 个本地文档链接、Python 语法与 34 个交付文件格式；`git diff --check` 通过。README 验证范围逐项核对测试源码及 D1 记录，变更仅涉及 4 份文档，沿用已有运行证据。
- 2026-10-11 00:13 功能提交工作流与基线：`make check` 通过模块/工具契约、19 个本地文档链接、34 个交付文件、格式、CLI 测试、vet 和 lint；核对提交范围仅包含项目交付文件，`.tmp/`、`.tools/` 和 `bin/` 保持忽略。数据库与协议实现未变化，沿用 D1 已通过的运行证据。
- 2026-10-11 00:08 D1 工程与协议：`make tools`、`make protocol-tools` 成功，仅安装到项目 `.tools/`；linter 实际由 Go 1.26.4 编译。`make check`、`make race build`、`make protocol-check` 与独立模块 lint/race 通过；二进制元数据确认工具链与主模块，CLI 进程退出码及无写入行为通过。`make compatibility-cross` 对运行依赖及生成协议的两个测试包在 Linux amd64/arm64、Windows amd64 完成纯 Go 编译；实际 gRPC/Gateway 回环与 Swagger 2.0 解析通过。
- 2026-10-11 00:08 D1 真实数据库：`make compatibility-db` 使用固定 RepoDigest 的 MySQL 8.4.11、PostgreSQL 18.6，在全新隔离数据库完成显式迁移、缺表失败且不建表、默认拒绝、角色权限、重载、删除与原子保存回滚；SQLite 同时通过，外部用例无跳过。回环 gRPC/Gateway 通过；两个测试项目的容器、卷及网络经 Docker 标签过滤只读检查均为零。日志仅作临时证据，正式复现入口见 README。
- 2026-10-11 00:08 D1 SQLite：`CGO_ENABLED=0` 本机测试通过多连接 PRAGMA、显式 Goose 迁移及重复执行、唯一/外键错误、事务回滚、运行中 deadline、busy timeout 与连接关闭后持久化读取；策略原型通过缺表失败且不建表、默认拒绝、角色权限、重载、删除与保存失败回滚。运行依赖树只包含 libtnb/modernc SQLite，不含第二套驱动。
