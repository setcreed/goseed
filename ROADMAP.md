# 项目进度

## 当前状态

goseed 是独立的 Go 项目与业务代码生成器，主模块为 `github.com/setcreed/goseed`，最低 Go 版本为 1.26.4，日常使用本机兼容工具链，发行归档固定版本，默认分支 main，许可证 MIT。源码仓库已公开至 https://github.com/setcreed/goseed ，origin 使用 HTTPS，main 跟踪 origin/main；首版功能验收状态见阶段快照和最近验证。

初始项目生成与 gen module 可用，支持四协议、四存储及可选 Redis/Casbin，共 64 种初始项目组合。资源阶段 R01 至 R10 已交付，包含公共 `gen resource`、四协议生成、安全写入与失败回滚，以及完整资源组合和真实服务回归。

需求、架构及从零建设的工作顺序见 [施工清单](docs/implementation.md)，新电脑恢复、实现入口与验证方法见 [开发指南](docs/development.md)。本文件是状态唯一来源；从现有进度继续，保留已交付代码与 Git 历史。

## 阶段快照

| 阶段 | 状态 | 交付边界 |
|---|---|---|
| D0 文档与规范 | 已交付 | 独立需求、架构、命令、规范、MIT、公开说明与换机恢复指南 |
| D1 技术核验 | 已交付 | 固定依赖、工具链、兼容原型、工程检查入口 |
| D2 默认模板 | 已交付 | HTTP + SQLite + JWT、示例 CRUD 与工程基础 |
| D3 可选能力 | 已交付 | MySQL/PostgreSQL/无数据库/Redis/Casbin；真实服务验证记录见 Git 历史 |
| D4 多协议 | 已交付 | 既有示例的 gRPC/dual/Gateway、安全链、协议复现与生命周期 |
| D5 项目生成 | 已交付 | 根命令、参数、元数据、dry-run、冲突与恢复保护 |
| D6 模块与资源 | 已交付 | gen module 与资源 R01 至 R10 已验收；48 种资源组合及真实 SQL/Redis/容器验证通过 |
| D7 首版收口 | 已交付 | 首版范围、文档、生成集成、真实服务、race 与交叉构建验收完成 |
| D8 基础使用快速验收 | 已交付 | 干净源码构建＋默认 HTTP/SQLite/JWT 流程＋一个真实 MySQL/纯 gRPC 代表项目 |
| D9 版本与分发 | 已交付 | 版本信息、本地安装/归档/校验、旧项目兼容与人工维护；Linux amd64 容器模拟运行通过，原生硬件未验证 |
| D10 可观测性 | 已交付 | P01 契约、P02 指标、P03 追踪与 P04 组合验收完成；默认关闭并按协议和选项裁剪 |
| D11 单机部署 | 已交付 | P01 部署契约、P02 四协议 systemd/Nginx 模板及真实服务验收、P03 SQLite/MySQL/PostgreSQL 备份恢复、P04 release 回退指引与隔离演练全部完成 |
| D12 至 D16 业务与运行扩展 | 暂缓 | 账号、消息、任务、多实例、Kubernetes 均退出连续排期；有具体业务需求再独立评估 |
| D17 资源声明增强 | 暂缓 | 仅在试用发现常见 CRUD 阻碍后评估最小增强范围 |

## 下一步施工顺序

D0 至 D11 的已有交付保持不变，原 D12 至 D17 全部暂缓。小型业务试用已完成，未发现资源生成、手工扩展、增量登记及基础安全流程的功能阻断。试用暴露的首要开发体验问题是 Makefile 强制切换 Go 工具链：本机 Go 1.27.2 在 `GOTOOLCHAIN=local` 下能构建并通过生成项目验证，固定工具链下载则受网络限制失败。

1. **Go 版本策略已落地**。各模块 `go.mod` 声明最低 Go 1.26.4；Makefile 和项目内验证使用 `GOTOOLCHAIN=local`，本机版本满足最低要求即可。可复现发行归档仍固定 Go 1.26.4；仓库当前没有 CI 工作流。
2. **后续按证据维护生成能力**。仅修复实际试用中复现的阻断或正确性问题；没有具体问题时不新增功能。
3. **候选方向不自动恢复**。账号、消息、任务、多实例、Kubernetes 和资源声明增强均需具体需求及独立实施指令。

当前维护边界见 [施工清单](docs/implementation.md#证据驱动的生成质量维护)，候选方向见 [需求清单](docs/requirements.md#已盘点但暂不实施的方向)。后续仅按改动风险定向验证，不默认重复 D7 全量长测。

## 已知缺口与验证边界

- R06 已验收：Swagger `/v1/<name>` 路径与 Handler 一致，请求/响应 schema 与 DTO 对齐；curl 示例展开 `$TOKEN`、接受临时 `API_URL` 并在生成项目集成测试中实际执行。SQLite Casbin 使用真实 Manager/策略存储验证默认拒绝、允许、权限不足及授权先于 Redis Check。源码与验收入口：`internal/templates/resource_core.go`、`internal/templates/resource/http_integration_test.go.tmpl`。
- R07 已验收：HTTP/RPC/dual 资源工厂共享一次 Biz 构造；两个资源的固定版 Proto 复现、独立 Go 客户端导入、真实 TLS RPC 主体隔离/边界/零值更新，以及 dual 跨协议读写通过。R09 随后开放完整资源写入及登记更新。
- SchemaStore 是只读 schema 原型；CoreFiles 提供完整 Store，不得混用输出路径。ResourcePlan 提供完整 Write 与公共 CLI；R10 的生成项目组合和运行行为已完成验收。
- 模块组装版本 2 表示纯权限/单次组装；R09/R10 已覆盖 Context/schema、旧项目兼容及多模块协议复现。
- R08 已交付 Gateway RPC 资源映射、ProtoJSON 说明及多模块协议/Swagger 复现；R09 已开放 `gen resource` 并完成完整写入回滚；R10 完成全矩阵和外部服务验收。
- D7 全仓 `make race` 已通过，`tests/integration` 用时 2289.398 秒；D6 的完整 `make check`、真实服务矩阵、生成产物集成和 D7 的兼容性/交叉构建检查均通过。
- go-redis 取消、pgx 人工延迟 Write 竞争边界仍按 [依赖记录](docs/dependencies.md) 保留；固定版本不意味着上游问题消失。
- 仓库采用 MIT；第三方依赖及生成代码仍保留适用许可。源码仓库已经公开；本地 D8 验收完成不表示最新提交已推送或正式版本已发布。D8 仅覆盖指定干净源码构建、默认 HTTP 使用流程和一个真实 MySQL/纯 gRPC 代表项目，不等同于无缓存、独立机器或跨平台恢复验收。
- D9 已完成本地安装、可复现归档、许可证与 SHA-256 核验、v1 增量兼容及人工维护说明。macOS arm64、Rosetta 下 macOS amd64、Linux arm64 的包运行和生成检查通过；Linux amd64 已在 Apple Silicon Docker 的 amd64 模拟容器中运行发行包，并验证默认项目健康及认证创建/列表，原生 amd64 硬件未验证；Windows 原生不在支持范围。D10 完成指标/追踪模板及本地 OTLP protobuf 接收器验收；D11 已完成真实 systemd/Nginx 服务级验收；D12 至 D17 均暂缓且未实施。

## 最近完成

- 2026-10-14 23:59 文档导航整合：将 HTTP、数据库/可选能力、gRPC、dual 与 Gateway 五份模板契约合并为 [模板指南](docs/templates.md)，更新 README、开发阅读顺序及引用；清除已完成部署能力被列为后续方向的旧表述。文档检查器现在会跳过工作区已删除的文件，使文档合并可正常验证。

- 2026-10-14 23:40 D9-P04 Linux amd64 运行补充验收：已校验的 Linux amd64 归档在 Apple Silicon Docker 的 Debian amd64 模拟容器中运行，并以该归档生成默认项目；D9 平台运行检查补齐。此结果不代表原生 amd64 硬件验收。

- 2026-10-14 23:17 业务试用路线收口：将已完成的小型业务试用从“下一步”改为已完成，并明确当前没有待启动的功能开发项；后续按实际反馈维护生成质量。原 D12 至 D17 继续暂缓，候选方向须有具体业务问题、最小范围、验收方式和对应实施指令。

- 2026-10-14 23:04 默认业务试用完成：Go 1.27.2 下生成 HTTP/SQLite/JWT 项目，生成 item CRUD 与 SQLite 迁移，再增量添加 checkout/returns 模块；生成项目构建、`go test ./...`、`go vet ./...` 通过。实际请求验证未认证拒绝、认证 CRUD、零值 PATCH、主体隔离及手写 checkout 操作；新增模块后手写 Biz/Handler 字节保持不变。没有发现生成流程的功能阻断。试用发现 Makefile 曾强制下载 Go 1.26.4；本机 Go 1.27.2 验证通过，因此将版本策略复核列为下一项。

- 2026-10-14 22:47 D11 单机部署完成：`make template-deployment` 全部通过，锁定 Debian 13 镜像实际运行 systemd 与 Nginx；专用账号/服务硬化、HTTP/gRPC TLS 代理、SIGTERM 停止与重启健康检查、错误证书拒绝路径通过。修正测试镜像缺少 CA bundle 导致 apt HTTPS 校验失败的问题，改用项目已锁定 Go 镜像的 CA bundle 并显式配置 CAInfo；修正 Debian 镜像启动时预启用 Nginx 导致生成配置未加载，以及 `systemctl is-active` 停止状态的测试断言。结合已通过的四协议 Nginx 语法、SQLite WAL/MySQL/PostgreSQL TLS 恢复和 release 回退演练，D11 全部检查点完成；仅测试容器，未触碰宿主或生产服务。
- 2026-10-14 21:48 D11-P03 备份恢复工具与 P04 回退指引完成：SQLite 生成 WAL 在线 `.backup` 与完整性检查，并拒绝损坏源和覆盖已有目标；MySQL 使用 TLS 校验和独立凭据文件的 `mysqldump --single-transaction`，PostgreSQL 使用 `verify-full` 与 custom dump。真实 SQLite WAL、MySQL 8.4.11、PostgreSQL 18.6 隔离恢复演练均核对回备份前数据且保留原备份；专用数据库卷/网络已清理。P04 增加 Debian 原子符号链接切换/回退步骤和隔离模拟；应用二进制回退不回滚 schema。P02 真实 systemd/Nginx 验收随后通过。

- 2026-10-14 21:04 D11-P01 部署契约完成：确定 `--deployment=single-host` 默认关闭、systemd/Nginx 仅作为 `deploy/` 文件副本；支持 HTTP/gRPC/dual/Gateway 及 SQLite/MySQL/PostgreSQL/无数据库，应用单实例绑定 loopback、双协议分离域名、Nginx TLS 终止/验证 gRPC 上游 TLS、不信任任意 forwarded headers。定稿专用服务账号、`/opt` releases+current、`/etc` 私有配置、`/var/lib` 状态和 `/var/backups` 备份目录；固定 SQLite 在线 `.backup`、MySQL InnoDB `mysqldump --single-transaction`、PostgreSQL custom `pg_dump` 及 schema 不自动回退边界。Docker 29.8.2 Linux/arm64 API 只读核验成功；D11-P02 进入实施。

- 2026-10-14 20:56 D10 可观测性完成：实现 `--metrics`/`--tracing` 独立选择、默认关闭、生成元数据兼容、依赖/源码裁剪；HTTP、gRPC、dual、Gateway 覆盖请求量/状态/延迟/依赖指标及 W3C TraceContext 父子传播，RPC 健康 Watch 也受追踪/指标拦截。Exporter 验证独立 endpoint 与 headers、HTTPS/loopback 限制、无 baggage/请求内容/凭据导出、3s 导出超时及共享 5s 失败清理/关闭预算。四协议指标和追踪项目 readonly 编译通过；本地 OTLP 接收器解码真实 protobuf 并核对 trace parent；HTTP 与 Gateway 双开组合、不可用出口限时关闭及观测包定向 race 通过。指标/追踪/组合集成矩阵用时 95.589s；race 与基准目标用时 9.149s。Apple M1 Pro / macOS arm64 微基准：Metrics `733.3 ns/op, 568 B/op, 5 allocs/op`，Trace `591.8 ns/op, 801 B/op, 8 allocs/op`，仅作环境基线，不代表线上请求端到端开销。根包定向单测、`make contract-check`（94 个链接、380 个交付文件）、`make fmt-check` 与 `git diff --check` 通过；未连接外部生产 Collector。

- 2026-10-14 19:53 D10-P01 观测契约与选型完成：固定 `--metrics`/`--tracing` 独立默认关闭、OpenTelemetry Go v1.47.0、OTLP/HTTP protobuf 主动推送、无入站指标端点、低基数标签、parent-based 0.1 采样、仅传播 TraceContext、远端 HTTPS/loopback 明文、3s 导出超时与 5s 关闭预算；记录 HTTP exporter 的 gRPC 依赖权衡。官方固定版本要求 Go 1.26.0，兼容项目工具链；P02 转为当前任务。

- 2026-10-14 19:37 D9 版本、分发与维护基线完成：生成器显示语义版本/提交/模板/spec；提供显式源码安装、固定 Go 1.26.4 的单平台本地包、依赖许可证、版本 manifest、可复现 SHA-256 与损坏拒绝；v1 旧式 metadata 能增量生成，不兼容版本在写前拒绝；维护流程和人工升级步骤已文档化。macOS arm64/amd64（Rosetta）与 Linux arm64 实际运行/生成通过；Linux amd64 运行保留未验证。D10 转为下一阶段；无 tag、公开 release 或远端推送。

- 2026-10-14 19:30 D9-P03 兼容与维护完成：将 metadata、template、resource spec 版本提升为独立常量；增量 CLI 继续兼容 v1 旧式字符串模块登记与新版映射，并对未知 metadata/template 版本报告受支持值且在计划阶段零文件变更。补充依赖单项升级审查/回归流程和既有项目人工升级步骤。P04 转为当前任务。

- 2026-10-14 19:27 D9-P02 安装与本地打包完成：新增固定 Go 1.26.4 的显式 `make install`、版本/提交/模板/spec 信息及单平台可复现 tar.gz 归档；归档附 Go 构建依赖许可证、JSON 版本清单、二进制摘要与 SHA-256 sidecar，并提供 `make verify-package`。安装和 macOS arm64 归档生成/复现通过；改包校验失败、脏工作区与非法版本均被拒绝。仅制作本地 QA 材料，未发布。

- 2026-10-14 19:21 D9-P01 版本、安装、平台与维护契约定稿：区分生成器语义版本、项目元数据格式、模板兼容版本和资源 spec 格式；本阶段仅生成本地安装/归档材料，明确 SHA-256 只用于损坏检测且不做 tag/release/公开分发。macOS/Linux amd64/arm64 保持候选，只有实际运行通过才列为已验证；更新策略为单依赖变更、复核许可证/维护证据/锁与 manifest，并人工升级既有项目。P02 转为当前任务。

- 2026-10-14 19:12 D8 基础使用快速验收完成：干净导出的已提交源码使用 Go 1.26.4 通过构建及 CLI 帮助/版本检查；默认 HTTP/SQLite 项目通过构建、初始化、迁移和健康检查，并实际完成认证 CRUD；MySQL 8.4.11＋纯 gRPC 项目通过生成构建、独立应用账号迁移、健康状态、未认证拒绝及认证 CRUD。D9 转为下一阶段；未推送或发布。

- 2026-10-14 19:02 按用户决定简化 D8 规划：仅保留 P01 干净源码构建与 P02 默认 HTTP/SQLite/JWT 使用流程，允许复用 Go 缓存；复杂组合、增量与失败路径仅按实际问题补测，版本/分发和开发工具恢复入口核对归 D9。同步需求、施工清单和开发导航，后续计划共 38 个任务；未启动运行验收。


- 2026-10-14 18:31 D8 规划完成：承接原 D7 交付核验与开发指南的恢复路径，定义 P01 至 P04 的依赖、范围、失败路径、验收和清理要求；下一实施入口为 P01，未启动运行验证。同步移除开发指南中 D7 进行中及 race 暂缓的失效描述，后续扩展方向仅列规划门槛。

- 2026-10-14 17:59 D7 首版交付核验完成：需求覆盖、README/开发指南、生成项目集成、构建、静态检查、真实 MySQL/PostgreSQL、Redis、容器及全仓 race 验收通过；兼容性原型、Linux amd64/arm64 与 Windows amd64 交叉编译、协议产物复现也通过。后续阶段尚未定义。

- 2026-10-14 17:13 D7 补充兼容性检查完成：依赖兼容原型与 Linux amd64、Linux arm64、Windows amd64 交叉编译通过；协议产物复现、格式和 vet 通过。兼容原型内需独立数据库凭据的 MySQL/PostgreSQL 子用例按设计跳过，对应真实数据库验收已由 R10 的 `make template-db` 完成。race 仍按用户决定暂缓。

- 2026-10-14 17:08 D7 首版范围与文档核对：对照需求验收项确认默认模板、数据库/无数据库、Redis、Casbin、四协议及 `gen module`/`gen resource` 均有已通过的集成或真实服务证据；同步开发指南至当前实现，核对 README 命令与 CLI 一致。race 按用户决定暂缓，D7 不标记完成。

- 2026-10-14 17:03 D6-R10 与 D6 收口完成：48 种 SQL×协议×Redis×Casbin 资源项目 readonly 编译、16 种无数据库拒绝、原有 64 项目协议/存储组合检查及四协议资源运行回归通过。真实 MySQL 8.4.11、PostgreSQL 18.6、TLS Redis 与 HTTP/gRPC/dual/Gateway/无数据库/Casbin 容器验收均通过，测试资源已清理。完整 `make check` 通过；额外全仓 race 按用户要求停止，未作为通过项记录。D7 转为下一阶段。



## 最近验证

- 2026-10-15 00:06 文档导航整合验证：`make contract-check` 通过（99 个本地链接、393 个实际交付文件），Markdown 本地链接核对与 `git diff --check` 通过；旧模板文件名和已知过期部署/路线表述检索无命中。未运行代码、数据库、Docker 或 race 测试。

- 2026-10-14 23:41 Linux amd64 发行包验收：`make verify-package ARCHIVE=.tmp/dist/goseed_v0.0.0-d9.1_linux_amd64.tar.gz` 通过；发行包在 Debian `trixie-slim` 的 `linux/amd64` 模拟容器中运行 `--version`、`--help` 并生成 52 文件默认项目。生成项目以 Go 1.27.2、`GOOS=linux GOARCH=amd64 CGO_ENABLED=0` readonly 构建；容器中显式初始化 SQLite 迁移、启动服务，health/ready、未认证拒绝（401）和认证创建/列表通过。`make contract-check`（97 个本地链接、397 个交付文件）与 `git diff --check` 通过。测试运行于 Apple Silicon Docker amd64 模拟环境，不代表原生硬件；专用容器和本轮临时目录已清理。

- 2026-10-14 23:17 业务试用状态文档核对：`make contract-check`（97 个本地链接、397 个交付文件）与 `git diff --check` 通过；核对施工清单、需求、CLI、开发指南和 ROADMAP 均将小型业务试用标为已完成，并指向证据驱动维护。未运行代码、数据库、Docker 或 race 测试。

- 2026-10-14 23:14 路线聚焦文档检查：`make contract-check`（96 个本地链接、397 个交付文件）与 `git diff --check` 通过；核对 ROADMAP、施工清单、需求、架构、CLI、开发指南和项目规范指向一致，最近完成/验证记录按时间倒序且不超过 20 条。未运行代码、数据库、Docker 或 race 测试。

- 2026-10-14 23:12 Go 版本策略验证：本机 Go 1.27.2 下 `make build`、`make test GO_PACKAGES='./cmd/goseed ./internal/cli ./internal/generator'`、对应 `make vet`、`make contract-check`（96 个本地链接、397 个交付文件）与 `git diff --check` 均通过；确认仅发行包脚本仍请求固定 Go 1.26.4，未运行打包流程、外部数据库/Redis 或 race。

- 2026-10-14 23:04 默认业务试用验证：本机 Go `1.27.2`、`GOTOOLCHAIN=local`；生成项目 `go test -mod=readonly -timeout=5m ./...`、`go vet -mod=readonly ./...` 通过，SQLite 显式迁移到 schema version 2。回环请求经批准的隔离本机端口运行通过，覆盖 health、未认证拒绝、认证创建/列表/零值更新、跨主体 404 和手写 checkout 成功/非法输入；未使用 Docker 或 race。

- 2026-10-14 22:47 D11-P02 完整运行验收：`make template-deployment` 通过；重新构建锁定 Debian/systemd/Nginx 测试镜像后，隔离 dual 项目服务硬化检查、HTTP TLS 与 gRPC 上游 TLS 健康请求、SIGTERM 停止状态、重启后健康恢复及错误证书失败路径全部通过。`make template-deployment-nginx` 四协议配置/错误证书用例通过；测试专用 Docker 容器自动清理。

- 2026-10-14 19:38 D9-P04 清洁提交最终验收：提交 `a5ab852` 上 `make install VERSION=v0.0.0-d9` 安装到隔离目录并显示正确提交/模板/spec；`make package` 从 clean source 生成 macOS/Linux amd64/arm64 四个包，manifest 均记录相同完整提交及 `source_dirty=false`，`make verify-package` 全部通过，macOS arm64 归档重建摘要一致。该 clean commit 的 macOS arm64 本机、macOS amd64 Rosetta、Linux arm64 固定断网容器包均运行 `--version` 并生成 52 个文件。Linux amd64 仍未运行：本机锁定镜像只有 arm64 内容层，强制 amd64 时 Docker 报缺失 layer；保留未验证。Windows 原生未测试。

- 2026-10-14 19:30 D9-P03 回归：`make test GO_PACKAGES='./internal/generator ./internal/cli'`、`make build`、CLI `--version`、`make fmt-check`、`make contract-check`（94 个本地链接、374 个交付文件）和 `git diff --check` 通过。旧 v1 字符串登记夹具实际执行 `PrepareModule`/`Write`/`LoadProject`；metadata/template v2 两个失败夹具明确提示只支持 v1，并以整棵项目文件快照证明无改动。归档复核显示版本字段与当前源码常量一致；测试归档已清理。

- 2026-10-14 19:27 D9-P02 检查：`make test GO_PACKAGES='./cmd/goseed ./internal/generator'`、`make fmt-check`、`make contract-check`（92 个本地链接、374 个交付文件）、`git diff --check` 通过。用户级写入未执行；`make install` 安装到隔离目录后版本输出正确。macOS arm64 归档两次生成 SHA-256 一致，`make verify-package` 验证二进制与全部许可证摘要通过；追加字节并保留原摘要后明确报 archive SHA-256 mismatch，脏工作区/非法版本均拒绝。临时安装、归档、篡改样本与 Python 缓存已清理。

- 2026-10-14 19:21 D9-P01 文档契约检查：`make contract-check`（92 个本地链接、372 个交付文件）和 `git diff --check` 通过；核对 D9 的本地/外部边界、四类版本独立语义、候选平台证据门槛及维护流程。仅修改施工计划与路线图，未生成或发布二进制。

- 2026-10-14 19:12 D8 P01/P02/P03 验收：从本地已提交 HEAD 导出干净源码后以固定 Go 1.26.4 执行 `make build`，CLI `--help`/`--version` 通过；默认 HTTP/SQLite/JWT 生成项目执行构建、`dev init`、迁移并启动回环服务，健康/就绪、未认证拒绝和认证创建/读取/更新/删除均符合契约；MySQL 8.4.11 专用容器中以独立应用账号执行 gRPC 项目迁移，健康为 SERVING，未认证请求返回 Unauthenticated，认证 CRUD 通过。服务、专用容器、卷、网络和任务临时目录均已清理；Go 共享缓存保留。未运行全矩阵、race 或跨平台恢复测试。

- 2026-10-14 19:04 D8 MySQL/gRPC 规划检查：`make contract-check`（92 个本地链接、372 个交付文件）与 `git diff --check` 通过；只读脚本核对 D8 三项任务、MySQL/gRPC 代表参数、总计划 39 个任务、标题锚点及最近记录上限。仅修改 4 份相关文档，未运行数据库/RPC 验收，未启动 Docker 或创建临时产物。

- 2026-10-14 19:02 D8 简化规划检查：`make contract-check`（92 个本地链接、372 个交付文件）与 `git diff --check` 通过；只读脚本核对 D8 仅 P01/P02、总计划 38 个任务、标题锚点及最近记录上限。仅修改 4 份相关文档，未执行运行验收、长测或 Docker 操作，未创建临时产物。

- 2026-10-14 18:39 后续全局规划检查：`make contract-check`（93 个本地链接、372 个交付文件）与 `git diff --check` 通过；只读脚本核对 10 项需求、10 个阶段、40 个任务、文档标题锚点及最近记录数量上限。工作区仅含 6 份相关文档修改；未执行业务/服务测试，未新增临时项目、缓存或 Docker 资源。

- 2026-10-14 18:31 D8 规划文档检查：`make contract-check`（82 个本地链接、372 个交付文件）与 `git diff --check` 通过；核对阶段依赖、当前状态及开发指南入口一致。仅修改规划与文档，未执行 D8 运行验收或重跑长时间测试，未创建验证项目或 Docker 资源。

- 2026-10-14 17:59 D7 最终验证：`PATH=/Applications/Docker.app/Contents/Resources/bin:$PATH make race` 全仓通过，集成包 `ok .../tests/integration 2289.398s`；此前 `make check`（集成包 2195.054 秒、vet、lint 0 issues）、`make build`、`make template-db`（真实 MySQL 8.4.11/PostgreSQL 18.6）、`make template-redis`（TLS Redis）、`make template-container`、`make compatibility`、`make compatibility-cross`、`make protocol-check`、`make fmt-check`、`make vet`、`make contract-check` 与 `git diff --check` 均通过。兼容原型中 MySQL/PostgreSQL 子用例按设计跳过，真实服务验证由 template-db 覆盖。本轮 race 项目和交叉编译输出由套件/任务清理；Docker 测试标签无残留容器，共享缓存保留。`.tmp/d2/render-3779792103` 时间戳早于本轮测试，作为既有目录保留。

- 2026-10-14 17:13 D7 补充检查：`make compatibility` 与 `make compatibility-cross` 通过，生成的 Linux amd64/arm64、Windows amd64 兼容性测试二进制随后清理；`make protocol-check`（固定 protoc、插件和生成文件匹配）、`make fmt-check`、`make vet` 通过。兼容性原型中的 MySQL/PostgreSQL 子用例按设计跳过，并非真实数据库验证；真实 MySQL/PostgreSQL 已由 `make template-db` 验收。共享 Go 模块缓存保留。全仓 race 暂缓。

- 2026-10-14 17:08 D7 文档与入口验证：`make contract-check`（79 个本地链接、372 个交付文件）、`git diff --check`、`make build` 通过；构建产物实际响应 `goseed --help`、`--version`、`gen module --help` 与 `gen resource --help`，资源参数显示 `--spec`、`--dry-run` 和 `--migration`，与 README 示例一致。README 的项目/模块/资源示例与生成器/生成项目文档核对通过；模板容器 README 明确迁移与策略前置步骤。race 按用户决定暂缓。

- 2026-10-14 17:03 D6-R10 验证：48 种 SQL×协议×Redis×Casbin 资源项目实际经 `PrepareResource`/`Write` 生成并通过 readonly 测试编译；16 种无数据库拒绝及现有 64 项目组合回归通过。真实 MySQL 8.4.11/PostgreSQL 18.6 的 `make template-db`、TLS 密码 Redis 的 `make template-redis`、非 root/只读文件系统等组合的 `make template-container` 全部通过，Docker 测试容器/卷已清理。`make protocol-check`、`make check`（集成包 2195.054 秒、vet、lint 0 issues）、`make build` 通过；工具超时边界增加至 15 秒，定向测试通过。`make race` 按用户要求中断，`tests/integration` 报告 `signal: interrupt`，运行 919.608 秒，不计为通过。中断后仅发现并清理本轮遗留的 `.tmp/d2/render-16425610`；Docker 测试标签无残留容器。
