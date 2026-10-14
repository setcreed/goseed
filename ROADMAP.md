# 项目进度

## 当前状态

goseed 是独立的 Go 项目与业务代码生成器，主模块为 `github.com/setcreed/goseed`，Go 固定 1.26.4，默认分支 main，许可证 MIT。源码仓库已公开至 https://github.com/setcreed/goseed ，origin 使用 HTTPS，main 跟踪 origin/main；首版功能验收状态见阶段快照和最近验证。

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
| D9 版本与分发 | 已交付 | 版本信息、本地安装/归档/校验、旧项目兼容与人工维护；Linux amd64 运行仍未验证 |
| D10 可观测性 | 已交付 | P01 契约、P02 指标、P03 追踪与 P04 组合验收完成；默认关闭并按协议和选项裁剪 |
| D11 单机部署 | 进行中 | P01 契约完成；P02 模板、P03 备份恢复演练、P04 升级故障验收待完成 |
| D12 独立账号模板 | 已规划，未实施 | 注册/登录/重置/刷新/撤销，SQL 与四协议安全链 |
| D13 消息基础 | 已规划，未实施 | 单后端发布消费、重试/重放、失败隔离及事务发布 |
| D14 任务系统 | 已规划，未实施 | Worker、异步/延迟/定时、取消与运行诊断 |
| D15 多实例安全 | 已规划，未实施 | 权限/会话收敛、共享限流与故障契约 |
| D16 Kubernetes | 已规划，未实施 | 独立清单、真实隔离集群、滚动更新与排空 |
| D17 资源声明增强 | 已规划，未实施 | 新 spec 版本、枚举/默认值/索引/唯一约束及兼容保护 |

## 下一步施工顺序

首版后全部已知方向已规划；D8 基础使用快速验收、D9 版本/分发与 D10 可观测性已完成，当前执行 D11 单机部署。首版后产品范围见 [需求清单](docs/requirements.md#首版后的完整需求范围)。

1. 推荐排期：**D8 → D9 → D10 → D11 → D12 → D13 → D14 → D15 → D16 → D17**；技术前置见施工清单，阶段内部按任务依赖验收。
2. 当前实施入口为 **D11-P02 systemd/Nginx 模板**；P01 已确定 `--deployment=single-host`、四协议/四存储支持、隔离主机路径、TLS/代理信任及恢复回退边界。其他阶段按施工清单中的前置关系执行。
3. 技术/产品定稿项 Q01 至 Q09 按 [待决登记](docs/implementation.md#待决登记与规划完成条件) 关闭；除范围变化或新证据推翻假设，不再每完成一阶段重做整体路线。

完整范围、任务、失败路径及验收见 [施工清单](docs/implementation.md#阶段依赖与交付物)。流式 RPC、通用分布式锁、高级资源 DSL、多消息后端、工作流 UI、多租户/SSO、原生 Windows 与自动升级等已明确 [暂不实施及重新纳入条件](docs/requirements.md#已盘点但暂不实施的方向)。后续验证按改动风险定向执行，不默认重复 D7 全量长测。推送、正式发版、外部通知和生产部署不属于本次规划授权。

## 已知缺口与验证边界

- R06 已验收：Swagger `/v1/<name>` 路径与 Handler 一致，请求/响应 schema 与 DTO 对齐；curl 示例展开 `$TOKEN`、接受临时 `API_URL` 并在生成项目集成测试中实际执行。SQLite Casbin 使用真实 Manager/策略存储验证默认拒绝、允许、权限不足及授权先于 Redis Check。源码与验收入口：`internal/templates/resource_core.go`、`internal/templates/resource/http_integration_test.go.tmpl`。
- R07 已验收：HTTP/RPC/dual 资源工厂共享一次 Biz 构造；两个资源的固定版 Proto 复现、独立 Go 客户端导入、真实 TLS RPC 主体隔离/边界/零值更新，以及 dual 跨协议读写通过。R09 随后开放完整资源写入及登记更新。
- SchemaStore 是只读 schema 原型；CoreFiles 提供完整 Store，不得混用输出路径。ResourcePlan 提供完整 Write 与公共 CLI；R10 的生成项目组合和运行行为已完成验收。
- 模块组装版本 2 表示纯权限/单次组装；R09/R10 已覆盖 Context/schema、旧项目兼容及多模块协议复现。
- R08 已交付 Gateway RPC 资源映射、ProtoJSON 说明及多模块协议/Swagger 复现；R09 已开放 `gen resource` 并完成完整写入回滚；R10 完成全矩阵和外部服务验收。
- D7 全仓 `make race` 已通过，`tests/integration` 用时 2289.398 秒；D6 的完整 `make check`、真实服务矩阵、生成产物集成和 D7 的兼容性/交叉构建检查均通过。
- go-redis 取消、pgx 人工延迟 Write 竞争边界仍按 [依赖记录](docs/dependencies.md) 保留；固定版本不意味着上游问题消失。
- 仓库采用 MIT；第三方依赖及生成代码仍保留适用许可。源码仓库已经公开；本地 D8 验收完成不表示最新提交已推送或正式版本已发布。D8 仅覆盖指定干净源码构建、默认 HTTP 使用流程和一个真实 MySQL/纯 gRPC 代表项目，不等同于无缓存、独立机器或跨平台恢复验收。
- D9 已完成本地安装、可复现归档、许可证与 SHA-256 核验、v1 增量兼容及人工维护说明。macOS arm64、Rosetta 下 macOS amd64、Linux arm64 的包运行和生成检查通过；Linux amd64 仅构建与包校验通过，运行未验证；Windows 原生不在支持范围。D10 完成指标/追踪模板及本地 OTLP protobuf 接收器验收；D11 实施中，D12 至 D17 尚未实施。

## 最近完成

- 2026-10-14 21:04 D11-P01 部署契约完成：确定 `--deployment=single-host` 默认关闭、systemd/Nginx 仅作为 `deploy/` 文件副本；支持 HTTP/gRPC/dual/Gateway 及 SQLite/MySQL/PostgreSQL/无数据库，应用单实例绑定 loopback、双协议分离域名、Nginx TLS 终止/验证 gRPC 上游 TLS、不信任任意 forwarded headers。定稿专用服务账号、`/opt` releases+current、`/etc` 私有配置、`/var/lib` 状态和 `/var/backups` 备份目录；固定 SQLite 在线 `.backup`、MySQL InnoDB `mysqldump --single-transaction`、PostgreSQL custom `pg_dump` 及 schema 不自动回退边界。Docker 29.8.2 Linux/arm64 API 只读核验成功；D11-P02 进入实施。

- 2026-10-14 20:56 D10 可观测性完成：实现 `--metrics`/`--tracing` 独立选择、默认关闭、生成元数据兼容、依赖/源码裁剪；HTTP、gRPC、dual、Gateway 覆盖请求量/状态/延迟/依赖指标及 W3C TraceContext 父子传播，RPC 健康 Watch 也受追踪/指标拦截。Exporter 验证独立 endpoint 与 headers、HTTPS/loopback 限制、无 baggage/请求内容/凭据导出、3s 导出超时及共享 5s 失败清理/关闭预算。四协议指标和追踪项目 readonly 编译通过；本地 OTLP 接收器解码真实 protobuf 并核对 trace parent；HTTP 与 Gateway 双开组合、不可用出口限时关闭及观测包定向 race 通过。指标/追踪/组合集成矩阵用时 95.589s；race 与基准目标用时 9.149s。Apple M1 Pro / macOS arm64 微基准：Metrics `733.3 ns/op, 568 B/op, 5 allocs/op`，Trace `591.8 ns/op, 801 B/op, 8 allocs/op`，仅作环境基线，不代表线上请求端到端开销。根包定向单测、`make contract-check`（94 个链接、380 个交付文件）、`make fmt-check` 与 `git diff --check` 通过；未连接外部生产 Collector。

- 2026-10-14 19:53 D10-P01 观测契约与选型完成：固定 `--metrics`/`--tracing` 独立默认关闭、OpenTelemetry Go v1.47.0、OTLP/HTTP protobuf 主动推送、无入站指标端点、低基数标签、parent-based 0.1 采样、仅传播 TraceContext、远端 HTTPS/loopback 明文、3s 导出超时与 5s 关闭预算；记录 HTTP exporter 的 gRPC 依赖权衡。官方固定版本要求 Go 1.26.0，兼容项目工具链；P02 转为当前任务。

- 2026-10-14 19:37 D9 版本、分发与维护基线完成：生成器显示语义版本/提交/模板/spec；提供显式源码安装、固定 Go 1.26.4 的单平台本地包、依赖许可证、版本 manifest、可复现 SHA-256 与损坏拒绝；v1 旧式 metadata 能增量生成，不兼容版本在写前拒绝；维护流程和人工升级步骤已文档化。macOS arm64/amd64（Rosetta）与 Linux arm64 实际运行/生成通过；Linux amd64 运行保留未验证。D10 转为下一阶段；无 tag、公开 release 或远端推送。

- 2026-10-14 19:30 D9-P03 兼容与维护完成：将 metadata、template、resource spec 版本提升为独立常量；增量 CLI 继续兼容 v1 旧式字符串模块登记与新版映射，并对未知 metadata/template 版本报告受支持值且在计划阶段零文件变更。补充依赖单项升级审查/回归流程和既有项目人工升级步骤。P04 转为当前任务。

- 2026-10-14 19:27 D9-P02 安装与本地打包完成：新增固定 Go 1.26.4 的显式 `make install`、版本/提交/模板/spec 信息及单平台可复现 tar.gz 归档；归档附 Go 构建依赖许可证、JSON 版本清单、二进制摘要与 SHA-256 sidecar，并提供 `make verify-package`。安装和 macOS arm64 归档生成/复现通过；改包校验失败、脏工作区与非法版本均被拒绝。仅制作本地 QA 材料，未发布。

- 2026-10-14 19:21 D9-P01 版本、安装、平台与维护契约定稿：区分生成器语义版本、项目元数据格式、模板兼容版本和资源 spec 格式；本阶段仅生成本地安装/归档材料，明确 SHA-256 只用于损坏检测且不做 tag/release/公开分发。macOS/Linux amd64/arm64 保持候选，只有实际运行通过才列为已验证；更新策略为单依赖变更、复核许可证/维护证据/锁与 manifest，并人工升级既有项目。P02 转为当前任务。

- 2026-10-14 19:12 D8 基础使用快速验收完成：干净导出的已提交源码使用 Go 1.26.4 通过构建及 CLI 帮助/版本检查；默认 HTTP/SQLite 项目通过构建、初始化、迁移和健康检查，并实际完成认证 CRUD；MySQL 8.4.11＋纯 gRPC 项目通过生成构建、独立应用账号迁移、健康状态、未认证拒绝及认证 CRUD。D9 转为下一阶段；未推送或发布。

- 2026-10-14 19:04 按用户要求增加 D8 的 MySQL/gRPC 验证规划：在 P01/P02 基础上新增 P03，一个 MySQL＋纯 gRPC 项目覆盖真实数据库、显式迁移、RPC CRUD、健康及未认证拒绝；只启动专用 MySQL 容器，复用 Go 缓存与已生成 Proto，不扩展为组合矩阵。同步需求、施工清单和开发导航，后续计划共 39 个任务；未启动运行验证。

- 2026-10-14 19:02 按用户决定简化 D8 规划：仅保留 P01 干净源码构建与 P02 默认 HTTP/SQLite/JWT 使用流程，允许复用 Go 缓存；复杂组合、增量与失败路径仅按实际问题补测，版本/分发和开发工具恢复入口核对归 D9。同步需求、施工清单和开发导航，后续计划共 38 个任务；未启动运行验收。

- 2026-10-14 18:39 首版后全局规划完成：需求 F01 至 F10 对应 D8 至 D17，明确推荐顺序、技术依赖、各阶段任务、失败/验收门槛与 Q01 至 Q09 定稿项；流式 RPC、通用锁等单列暂不实施及重启条件。同步需求、架构、CLI 规划边界和开发导航，仅文档规划，未启动开发或长测。

- 2026-10-14 18:31 D8 规划完成：承接原 D7 交付核验与开发指南的恢复路径，定义 P01 至 P04 的依赖、范围、失败路径、验收和清理要求；下一实施入口为 P01，未启动运行验证。同步移除开发指南中 D7 进行中及 race 暂缓的失效描述，后续扩展方向仅列规划门槛。

- 2026-10-14 17:59 D7 首版交付核验完成：需求覆盖、README/开发指南、生成项目集成、构建、静态检查、真实 MySQL/PostgreSQL、Redis、容器及全仓 race 验收通过；兼容性原型、Linux amd64/arm64 与 Windows amd64 交叉编译、协议产物复现也通过。后续阶段尚未定义。

- 2026-10-14 17:13 D7 补充兼容性检查完成：依赖兼容原型与 Linux amd64、Linux arm64、Windows amd64 交叉编译通过；协议产物复现、格式和 vet 通过。兼容原型内需独立数据库凭据的 MySQL/PostgreSQL 子用例按设计跳过，对应真实数据库验收已由 R10 的 `make template-db` 完成。race 仍按用户决定暂缓。

- 2026-10-14 17:08 D7 首版范围与文档核对：对照需求验收项确认默认模板、数据库/无数据库、Redis、Casbin、四协议及 `gen module`/`gen resource` 均有已通过的集成或真实服务证据；同步开发指南至当前实现，核对 README 命令与 CLI 一致。race 按用户决定暂缓，D7 不标记完成。

- 2026-10-14 17:03 D6-R10 与 D6 收口完成：48 种 SQL×协议×Redis×Casbin 资源项目 readonly 编译、16 种无数据库拒绝、原有 64 项目协议/存储组合检查及四协议资源运行回归通过。真实 MySQL 8.4.11、PostgreSQL 18.6、TLS Redis 与 HTTP/gRPC/dual/Gateway/无数据库/Casbin 容器验收均通过，测试资源已清理。完整 `make check` 通过；额外全仓 race 按用户要求停止，未作为通过项记录。D7 转为下一阶段。

- 2026-10-14 14:26 D6-R09 公共资源生成完成：新增 `goseed gen resource <name> --spec <path> [--dry-run] [--migration]`，准备阶段编译固定 Proto/Gateway 产物、验证模块登记和 OpenAPI 投影，写入前重验快照，失败恢复新增文件与登记索引。HTTP CLI dry-run/写入及四协议生成、12 种 SQL×协议写入组合与失败注入测试通过；R10 转为当前任务。

- 2026-10-14 14:13 D6-R08 Gateway 资源验收完成：Gateway 资源复用 RPC Handler/Biz，生成路由映射、OpenAPI 配置和协议差异文档；协议脚本发现全部模块，固定工具在临时目录生成，并通过项目内严格 OpenAPI 合并器更新模块源、汇总和登记摘要。R09 转为下一开发项。

- 2026-10-14 14:00 D6-R07 gRPC/dual 资源验收完成：新增资源 Proto、RPC Handler 与精确 FullMethod 权限；dual 工厂只创建一次 Store/Biz 并供 HTTP/RPC 共用。Proto 脚本已支持多模块排序复现；公共资源写入、登记更新仍由 R09 实施。

- 2026-10-14 13:38 D6-R06 HTTP 资源验收完成：修正 Swagger 路径、严格请求与完整响应 schema，curl 示例支持 Token/临时服务地址并由集成测试执行；真实 SQLite Casbin 策略覆盖默认拒绝、允许、权限不足及 Redis 检查顺序。R07 成为下一开发项。

- 2026-10-12 16:40 使用 gh 创建公开仓库 setcreed/goseed，推送 main 并建立 origin/main 跟踪；保留全部本地 Git 历史。README 与开发指南补齐真实克隆地址，可在新电脑恢复开发。

- 2026-10-12 16:19 公开源码与换机文档准备：重写公开 README 和当前进度快照，按从零建设整理 D0 至 D7 及详细 D6 施工单，新增源码导航/环境恢复指南与 MIT LICENSE。去除特定外部项目名称检查规则、个人账号示例和旧会话依赖；保留现有代码、版权与 Git 历史。核对 R06 时发现文档/权限验收缺口，已回到待补齐状态。尚未创建远端或推送。

- 2026-10-12 15:57 R06 HTTP 源渲染提交 `2704b39`：提供 Handler、schema 核验工厂、Swagger/模块说明与 HTTPFiles 副本；只读准备检查多源合并。完整验收状态按上面的缺口执行，本条不认定全部 R06 门槛通过。
- 2026-10-12 15:23 R05 CRUD 核心提交 `de13630`：主体隔离 Model/Biz/Store、字段出现性、零值更新、分页、UTC DTO 与更新事务回滚；没有增加运行依赖。
- 2026-10-12 13:03 R04 提交 `a123019`：迁移版本/应用集合核验、SQL 草稿、资源只读 schema 与启动 Context；生成器不执行数据库操作。

## 最近验证

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

- 2026-10-14 15:42 R10 前置代码与生成项目矩阵：48 种 SQL×协议×Redis×Casbin resource 项目均经 `PrepareResource`/`Write` 生成并通过 `go test -mod=readonly -run '^$' ./...`；16 种无数据库拒绝由生成器测试覆盖。现有 gRPC/dual 32 种数据库/Redis/Casbin 项目组合通过；纯 gRPC、dual、Gateway 两资源生成、Proto 多模块逐字节复现及 RPC 运行用例通过。修复 gRPC Proto 源名导致的复现差异，并修正纯 gRPC 资源夹具不应读取 OpenAPI 的前提。此前的 protocol-check、contract-check（79 个本地链接、372 个交付文件）、fmt-check、lint（0 issues）、vet、定向 generator/CLI/templates 测试、build 与 diff 检查通过；真实数据库、Redis 和容器结果见本次 17:01 记录。

- 2026-10-14 14:26 R09：`go test ./internal/generator ./internal/cli ./internal/templates` 通过；`make contract-check`（79 个本地链接、371 个交付文件）、`make fmt-check`、`make lint`（0 issues）及 `git diff --check` 通过。覆盖四协议公共 CLI、四协议资源写入、SQLite/MySQL/PostgreSQL × 四协议共 12 种生成写入组合（Redis + Casbin）、无数据库拒绝、冲突/过期输入及注入失败后的字节级恢复。R10 接手真实项目组合与外部服务验收。

- 2026-10-14 14:13 R08：`go test ./internal/templates ./internal/generator`、`go test ./tests/integration -run 'TestResourceGRPCProjects/gateway' -count=1 -v`、`make contract-check`（78 个本地链接、369 个交付文件）、`make fmt-check`、`make lint`（0 issues）与 `git diff --check` 通过。真实 TLS Gateway 项目含内置模块与两个资源，复现后 Proto/Gateway/各模块 Swagger/汇总/摘要逐字节一致；测试 HTTP→RPC 请求 ID、JWT、主体隔离、非法 query、ProtoJSON int64 字符串与跨协议 CRUD。外部真实 SQL/Redis 与容器留 R10；测试临时项目由套件清理。

- 2026-10-14 14:00 R07：锁定 protoc 36.2 与插件安装后，双资源 gRPC/dual 集成项目通过 Proto 多模块二次逐字节复现、独立客户端导入、readonly 测试、vet、lint 与 build。真实 TLS RPC 覆盖两个资源、alice/bob 隔离、显式零值、必填/未知字段及 ID/分页/deadline 错误；dual 验证 RPC 创建 → HTTP 读取/更新 → RPC 读回。`make test GO_PACKAGES='./internal/templates ./internal/generator'`、`make protocol-check`、`make fmt-check`、`make lint`、`make contract-check`（78 个本地链接、365 个交付文件）与 `git diff --check` 通过。运行本地监听验收需沙箱外执行；测试临时项目已清理。

- 2026-10-14 13:38 R06：`make test GO_PACKAGES='./internal/templates ./tests/integration -run TestResource -count=1 -v'` 与 `make test GO_PACKAGES='./tests/integration -run TestResourceCoreProjects -count=1 -v'` 通过。五个 SQLite 资源组合生成项目完成 readonly race 测试、HTTP Handler 测试、bootstrap 编译、实际执行模块 curl、vet、固定 linter 与 build；Casbin+Redis 用例通过真实策略及检查顺序断言。根仓库定向 vet、`make lint`（0 issues）、`make contract-check`（78 个本地链接、361 个交付文件）、`make fmt-check` 与 `git diff --check` 通过。测试临时项目由套件清理；`.tmp` 共享缓存保留。

- 2026-10-14 13:28 R06 补齐 Swagger `/v1` 路径、请求/响应 schema、可执行 curl 示例与 SQLite Casbin Manager 验收夹具；`make contract-check`（78 个本地链接、361 个交付文件）和 `make fmt-check` 通过。两组 Go 测试未启动：固定 Go 1.26.4 工具链需从 `goproxy.cn` 下载，但当前 DNS 无法解析；本机 Go 1.27.2 不符合项目固定版本，未用于测试。R06 仍待用 Go 1.26.4 执行模板与资源集成测试。

- 2026-10-12 16:40 远端发布：gh 查询确认仓库为 PUBLIC、默认分支 main；GitHub API 返回的 main 提交与首次推送的本地 HEAD e895372 一致。认证有效；未新增构建产物或任务缓存。

- 2026-10-12 16:19 文档与公开准备：make contract-check（78 个本地链接、361 个交付文件）、git diff --check 通过；Make dry-run 核对构建、工具安装和核心测试入口。当前交付文件名称/个人路径扫描无候选；Git 历史 826 个可达 blob 的私钥/GitHub Token/AWS 访问键特征扫描无候选，不代表完整凭据审计。未重跑业务/数据库/容器测试，未在新电脑验证；未创建任务临时项目或缓存，既有本地产物未删除。

- 2026-10-12 15:57 R06：核心包测试及五类 SQLite 生成项目 readonly race/vet/lint/build 通过；基础与接口替身组合通过回环 HTTP Server。真实 Casbin 策略和 Swagger 最终路径未被该轮验收覆盖。文档契约、格式、vet、lint（0 issues）和 diff 检查通过，任务目录已清理。
- 2026-10-12 15:23 R05：五个 SQLite 核心项目、真实 MySQL/PostgreSQL 四个资源组合通过主体 CRUD、零值/int64、取消、并发、事务回滚与独立应用账号验收；template-db 七组共 18 个组合无跳过通过。未执行完整 64 项目矩阵或新增真实 Redis 验收。
- 2026-10-12 13:03 R04：SQLite 与真实 MySQL/PostgreSQL 迁移/schema、异常历史、宽表、Unicode/排序与取消验收通过；确认 MySQL 部分 DDL 保留边界。任务项目及本次容器/卷/网络已清理。
