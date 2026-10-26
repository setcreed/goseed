# 项目进度

## 当前状态

goseed 是独立的 Go 项目与业务代码生成器，主模块为 `github.com/setcreed/goseed`，最低 Go 版本为 1.26.4，日常使用本机兼容工具链，发行归档固定版本，默认分支 main，许可证 MIT。源码仓库已公开至 https://github.com/setcreed/goseed ；首版功能验收状态见阶段快照和最近验证。

初始项目生成与 gen module 可用，支持四协议、四存储及可选 Redis/Casbin，共 64 种初始项目组合。资源阶段 R01 至 R10 已交付，包含公共 `gen resource`、四协议生成、安全写入与失败回滚，以及完整资源组合和真实服务回归。

架构、命令行为及从零建设的工作顺序见 [架构说明](docs/architecture.md)、[CLI 参考](docs/cli.md) 和 [施工计划](docs/implementation.md)。本文件保留开发记录与当前状态；从现有进度继续，保留已交付代码与 Git 历史。

## 阶段快照

| 阶段 | 状态 | 交付边界 |
|---|---|---|
| D0 文档与规范 | 已交付 | 产品范围、架构、命令、规范、MIT 与开发指南 |
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

基础 GitHub Actions CI 已落地，[首轮检查](https://github.com/setcreed/goseed/actions/runs/38609806274) 全部通过；[v0.1.2](https://github.com/setcreed/goseed/releases/tag/v0.1.2) 已正式发布为 Latest，修复资源 `int64` 精度测试与常见 Biz 范围校验冲突。[发布提交的 CI](https://github.com/setcreed/goseed/actions/runs/38614318633)、四平台资产、固定版本和 Latest 安装均已验收。v0.1.1 的缓存修复与旧项目处理范围见 [CI 与补丁发布清单](docs/implementation.md#基础-ci-与-v011-补丁发布) 和[旧项目处理说明](docs/templates.md#构建缓存与旧项目处理)。已有项目仍需手动更新 Makefile，不自动清理用户缓存或改写已发布标签。

安装与 CLI 使用收口及真实远端安装已验收，范围见 [施工清单](docs/implementation.md#安装与-cli-使用收口施工清单)。四平台发行包及 SHA-256 可下载；当前没有已排定的新功能，后续按实际使用反馈维护。发布流程见 [首次版本发布清单](docs/implementation.md#首次版本发布清单)。

D0 至 D11 的已有交付保持不变，原 D12 至 D17 全部暂缓。v0.1.2 发布后商品管理试用未发现资源生成、手工扩展、增量登记及基础安全流程的功能阻断；试用中发现的 `MinInt64` 精度测试与常见非负业务约束冲突，已在 v0.1.2 将精度断言移至 Store 层。此前发现的 Makefile 工具链切换问题已在 v0.1.1 修复。

1. **Go 版本策略已落地**。各模块 `go.mod` 声明最低 Go 1.26.4；Makefile 和项目内验证使用 `GOTOOLCHAIN=local`，本机版本满足最低要求即可。可复现发行归档仍固定 Go 1.26.4；CI 从 `go.mod` 选择工具链。
2. **后续按证据维护生成能力**。仅修复实际试用中复现的阻断或正确性问题；没有具体问题时不新增功能。
3. **候选方向不自动恢复**。账号、消息、任务、多实例、Kubernetes 和资源声明增强均需具体需求及独立实施指令。

当前维护边界见 [施工计划](docs/implementation.md#证据驱动的生成质量维护)，候选方向及重新纳入条件见 [暂缓方向](docs/implementation.md#暂缓方向及重新纳入门槛)。后续仅按改动风险定向验证，不默认重复 D7 全量长测。

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

- 2026-10-26 12:26 增量写入逻辑收拢：模块与资源保留各自输入、路径和兼容复核，共用内部独占创建/索引原子替换/失败回滚实现；补充每个索引写入阶段失败、创建冲突及回滚失败的证据和清理回归。

- 2026-10-26 12:24 CI 生成编译回归：新增 make template-smoke 并接入普通 CI，覆盖四协议资源字段边界及 gRPC/Gateway 全能力组合；缓存键纳入模板依赖清单与观测校验和。README 和开发指南同步前置工具、编译范围及外部服务边界。

- 2026-10-26 12:22 观测组合依赖修复：OTLP 叠加保留协议已锁定的 gRPC/Gateway 版本，并补齐 x/text v0.42.0 要求的 x/sync v0.23.0 及校验和；资源生成编译回归增加 gRPC/Gateway 全能力组合。

- 2026-10-26 12:21 HTTP OpenAPI 整数契约修复：请求与响应共用字段 schema 渲染，所有业务 int64 字段完整声明 integer/int64，保留字符串长度和 bool 类型。

- 2026-10-26 12:21 增量能力保留修复：统一元数据到模板选项的转换，模块、资源及组装核验完整保留 deployment 等能力；四协议全选回归同时修复纯 gRPC 数据库指标与 Redis 检查之间缺少语句分隔的模板错误。

- 2026-10-26 11:32 元数据 YAML 序列化修复：改用现有 YAML 编码器写出完整结构并传播编码错误，保留两空格缩进；合法项目、模块和资源名称 null 不再破坏后续增量生成，兼容已有 v1 元数据。

- 2026-10-26 11:30 资源字段协议命名修复：区分业务字段与创建、更新、响应消息的 Proto Go 字段，覆盖保留方法、getter、数字和 optional 合成 oneof；修正数字分段的资源 spec GoName。协议插件保持固定模块/版本，编译器改为接受满足最低 Go 1.26.4 的本机版本。

- 2026-10-15 02:27 v0.1.2 补丁版发布：资源 Store 精度测试改在存储层直接验证，发布提交 `5af2d2b`；GitHub Actions run `38614318633` 通过。Go 1.26.4 构建 macOS/Linux amd64/arm64 四个平台包及 SHA-256，远端 8 个资产摘要核对一致；固定版本与 `@latest` 安装均为 v0.1.2，Release 已公开为 Latest，资产保留在 `dist/v0.1.2/`。

- 2026-10-15 02:14 资源 Store 精度测试与 Biz 规则解耦：生成的 `int64` 精度边界用例直接调用 Store，避免新增常见的非负业务规则后必须改动通用测试。修正仅影响当前模板源码，v0.1.1 发行内容未改变。

- 2026-10-15 02:06 v0.1.1 商品管理业务试用完成：使用正式 macOS arm64 发行归档生成 HTTP/SQLite 商品资源，加入手写价格/库存规则和 order 模块；认证 CRUD、零值 PATCH、跨主体隔离及增量生成保留手写文件均通过。按业务非负约束调整生成测试中的负数精度样本后，全量生成项目测试通过；未发现发布阻断。

- 2026-10-15 01:54 基础 CI 与 v0.1.1 发布完成：新增 `.github/workflows/ci.yml`，在 main 推送、PR 和手动触发时运行轻量检查，锁定官方 Actions SHA、按 go.mod 选择 Go 并缓存仓库实际目录。生成项目缓存修复和冷缓存测试适配随 v0.1.1 发布，标签指向 CI 已通过的 `85c3199`；四平台归档与摘要已公开，README 安装入口更新，v0.1.0 标签及资产保持不变。

- 2026-10-15 01:36 项目内只读缓存问题修复：生成 Makefile 不再强制覆盖 GOCACHE、GOMODCACHE 和 GOLANGCI_LINT_CACHE，避免普通构建在每个业务项目创建依赖缓存及只读目录，并保留用户的环境变量和 Go 配置。同步 SQL/无数据库 README 与旧项目手动更新、定向清理说明；未修改现有业务项目或发布补丁版本。

- 2026-10-15 01:17 v0.1.0 首次正式发布：版本标签指向干净源码 `e93e692`；Go 1.26.4 构建 macOS/Linux 的 amd64/arm64 四个包及四个摘要文件，均已上传至 GitHub Release，并从草稿公开为 Latest。发布说明、固定版本安装示例和 README 下载入口已同步；正式发行文件保留在 `dist/v0.1.0/`。

- 2026-10-15 01:08 真实远端安装验收完成：提交 `1a8c8b3` 已推送，使用真实 Go 模块代理分别安装 `@main` 与 `@latest`，均解析到 `v0.0.0-20261014165130-1a8c8b3b82d2`。安装后可以脱离源码目录生成和运行默认项目；尚未打标签或创建 Release。

- 2026-10-15 00:49 安装与 CLI 使用收口完成：施工清单 I01 至 I04 已实施；README 提供 `go install` 与源码编译入口，新增 `goseed init` 并共享生成逻辑，裸根命令改为帮助，带生成参数的旧入口继续兼容。安装版从 Go BuildInfo 读取模块版本，发行构建显式注入优先，本地开发版保留 dev；模板及资源 spec 版本不变。

- 2026-10-15 00:34 公开仓库文档整理：将快速开始和默认项目运行流程集中到 README，合并可选模板行为说明，删除重复的需求文档；保留 ROADMAP、施工计划和开发记录。精简贡献者开发指南，并修正生成项目 Makefile 强制下载 Go 1.26.4 的问题，使其使用满足最低版本要求的本机 Go。

- 2026-10-14 23:59 文档导航整合：将 HTTP、数据库/可选能力、gRPC、dual 与 Gateway 五份模板契约合并为 [模板指南](docs/templates.md)，更新 README、开发阅读顺序及引用；清除已完成部署能力被列为后续方向的旧表述。文档检查器现在会跳过工作区已删除的文件，使文档合并可正常验证。

- 2026-10-14 23:40 D9-P04 Linux amd64 运行补充验收：已校验的 Linux amd64 归档在 Apple Silicon Docker 的 Debian amd64 模拟容器中运行，并以该归档生成默认项目；D9 平台运行检查补齐。此结果不代表原生 amd64 硬件验收。

- 2026-10-14 23:17 业务试用路线收口：将已完成的小型业务试用从“下一步”改为已完成，并明确当前没有待启动的功能开发项；后续按实际反馈维护生成质量。原 D12 至 D17 继续暂缓，候选方向须有具体业务问题、最小范围、验收方式和对应实施指令。

- 2026-10-14 23:04 默认业务试用完成：Go 1.27.2 下生成 HTTP/SQLite/JWT 项目，生成 item CRUD 与 SQLite 迁移，再增量添加 checkout/returns 模块；生成项目构建、`go test ./...`、`go vet ./...` 通过。实际请求验证未认证拒绝、认证 CRUD、零值 PATCH、主体隔离及手写 checkout 操作；新增模块后手写 Biz/Handler 字节保持不变。没有发现生成流程的功能阻断。试用发现 Makefile 曾强制下载 Go 1.26.4；本机 Go 1.27.2 验证通过，因此将版本策略复核列为下一项。

## 最近验证

- 2026-10-26 12:26 写入重构验收：cmd、CLI、生成器与模板完整相关单测通过（生成器 29.078s），覆盖现有冲突/过期输入/符号链接和新增逐阶段回滚用例；最终 make template-smoke 四协议生成项目测试编译通过（55.579s），生成器构建、相关 vet、格式、契约及 diff 检查通过。本轮未运行 lint、race、Docker、外部数据库或远端 Actions。

- 2026-10-26 12:24 CI 本地验收：make template-smoke 实际执行通过（53.372s），包含四种生成项目 readonly 构建与全部测试编译；工作流 YAML 解析及步骤检查、格式、文档契约与 diff 检查通过。尚未推送，未运行远端 Actions；不代表外部服务运行验收。

- 2026-10-26 12:22 观测组合验收：通过临时生成项目的 go mod tidy -diff 和模块图定位版本差异，仅修正模板依赖，不在验收时 tidy；gRPC 与 Gateway 的 SQLite/Redis/Casbin/指标/追踪/单机部署全选项目 readonly 构建及全包测试编译分别通过（19.547s、18.507s），模板单测与相关 vet、格式、契约及 diff 检查通过。未连接外部 Redis、数据库或 Collector。

- 2026-10-26 12:21 OpenAPI 回归：合并后的 Swagger 在旧实现缺少响应 int64 格式的用例先失败；修复后模板完整单测、模板 vet、格式、契约及 diff 检查通过，覆盖创建、更新和响应的整数、文本与布尔字段。服务运行逻辑未变。

- 2026-10-26 12:21 能力保留回归：旧实现复现 single-host 在增量模块/资源生成后丢失；四协议、模块/资源及 Redis/Casbin/指标/追踪/部署全选的 8 个用例通过，模块与资源协议组合写入定向回归、相关 vet、格式、契约及 diff 检查通过。

- 2026-10-26 11:32 元数据回归：新增用例先在旧实现复现项目 null 与增量模块/资源 null 读取失败；修复后 cmd、CLI、生成器完整单测及相关 vet 通过，格式、契约与 diff 检查通过。未修改生成服务运行行为，无外部服务验收。

- 2026-10-26 11:30 字段命名回归：模板与生成器完整单测通过；四协议 SQLite 真实生成项目 readonly 构建及全部测试编译通过（50.887s），覆盖 15 个命名边界字段；相关 vet、格式、文档契约及 diff 检查通过。使用本机 Go 1.27.2，无 Docker、外部数据库或 race。

- 2026-10-15 02:27 v0.1.2 发布验收：CI 运行 `38614318633` 全部通过；四个发行包均验证 manifest 的源码提交、版本、Go 1.26.4、平台和二进制/许可证摘要，macOS arm64 本机版本/帮助通过。GitHub Release 的 8 个资产名与 SHA-256 匹配本地；隔离 GOBIN 的 `go install @v0.1.2` 与 `@latest` 均显示 v0.1.2。用已安装版本生成商品项目、加入手写非负 price/stock Biz 校验，Store/Biz 测试和 vet 通过；草稿公开后复核 Latest 状态。未运行 race、Docker 或外部数据库；发行临时目录随后清理。

- 2026-10-15 02:14 Store 精度回归验证：`make test GO_PACKAGES='./internal/templates'`、对应 vet、格式检查与 diff 检查通过；当前源码生成 SQLite 商品项目后手写 Biz 非负 price/stock 校验，Store/Biz 定向测试以及生成项目 `go test -mod=readonly -timeout=5m ./...` 和 `go vet -mod=readonly ./...` 均通过。未运行 race、Docker 或外部数据库；任务临时项目已清理。

- 2026-10-15 02:06 v0.1.1 商品资源试用验证：运行公开 macOS arm64 归档 `v0.1.1`，生成 `example.com/shop`，执行 SQLite schema 2 迁移、构建、全部测试及 `go vet`；真实回环请求覆盖未认证 401、必填/重复键/owner 注入/分页错误、手写负值拒绝、创建/读取/列表/零值更新/删除和 Alice/Bob 隔离。随后增量生成 `order`，10 个商品资源与手写文件 SHA-256 不变；`make fmt-check`、readonly 构建、生成项目全量测试和 vet 通过，项目内未产生 `.tmp`。按试用要求仅使用 SQLite/本机回环，未运行 Docker、外部数据库或 race；临时项目与服务日志已清理。

- 2026-10-15 01:54 CI 与补丁发行验收：真实 Actions run `38609806274` 在 Ubuntu 24.04、Go 1.26.4 下完整通过，用时 2 分 03 秒，覆盖格式/契约、锁定协议工具、cmd/internal 单测、缓存定向回归、vet 和构建。四平台包的源码/版本/平台/许可证摘要通过，macOS arm64 本机及 amd64 Rosetta 生成与 make build 成功、遵循 GOENV 缓存且无项目内 .tmp，arm64 归档重建摘要一致。真实 `@v0.1.1` 和 `@latest` 均显示 v0.1.1，帮助、init/dry-run、模块/资源生成、构建及 SQLite 迁移通过；gh API 核对标签、CI 提交和 8 个上传资产大小/SHA-256 一致，Release 已公开为正式版。任务临时目录与隔离安装缓存已清理，22 MiB 正式发行包保留在 dist/v0.1.1。未重复 Docker、外部数据库或全仓 race。

- 2026-10-15 01:48 轻量 CI 本地验证：工作流 YAML 解析、3 个官方 Actions 的固定 SHA/版本/运行时核对通过；cmd/internal 全部单元测试、缓存定向回归（2.393 秒）、相关 vet、生成器构建、格式/文档契约与 diff 检查通过。冷缓存回归改为允许正常下载依赖，缓存位置断言保留；远端 CI 与 v0.1.1 发布仍在进行中。

- 2026-10-15 01:36 缓存回归与清理验收：新用例在旧模板下因默认缓存及 Go 配置被覆盖而失败，修复后定向集成测试 4.068 秒通过；SQLite/无数据库两种生成项目核对默认位置、显式环境变量和隔离 GOENV 配置，实际 make build 成功且无项目内 .tmp。隔离下载已缓存的小型模块，复现当前用户所有的 0555 目录与普通递归删除 PermissionError，并验证显式 GOMODCACHE 的 go clean -modcache 无需 sudo 清理。模板/CLI/生成器测试、相关 vet、Go 1.26.4 下 lint（0 issues）、构建、格式、文档契约和 diff 检查通过。误启动的全量集成测试已中止，不计通过；未运行 Docker 或 race。新建临时项目和复现缓存均已清理，既有共享缓存与用户 demo 保留。

- 2026-10-15 01:17 v0.1.0 发布验收：四归档的 manifest、源码提交、平台、Go 版本、二进制和许可证 SHA-256 全通过，macOS arm64 重建摘要一致；macOS arm64 本机和 amd64 Rosetta 的版本/帮助/生成/构建通过。真实 `go install ...@v0.1.0` 和 `@latest` 均显示 v0.1.0，帮助、init、dry-run、模块/资源生成、readonly 构建、开发初始化及 SQLite 迁移通过。gh API 核对远端注释标签、目标提交、8 个资产的大小与 SHA-256，正式页面返回非草稿/非预发布；本次任务临时目录和安装缓存已清理，22 MiB 正式发行文件保留。未重复 Linux 原生运行、Docker 或全仓 race。

- 2026-10-15 01:12 v0.1.0 发布准备检查：gh 只读核对远端无已有标签或 Release；`make test GO_PACKAGES='./cmd/goseed ./internal/cli ./internal/generator'`、`make fmt-check`、`make contract-check`（73 个本地链接、392 个交付文件）和 `git diff --check` 通过。发行包、版本标签、固定版本安装及 Release 尚在进行中。

- 2026-10-15 01:08 GitHub 安装链路验证：本机 Go 1.27.2、隔离 GOBIN 和初始为空的任务 GOMODCACHE、复用共享构建缓存，执行 `go install github.com/setcreed/goseed/cmd/goseed@main` 与 `@latest` 均成功，版本指向 `1a8c8b3`。两者均通过帮助、init、dry-run 无写入、重复生成拒绝、模块和资源生成、readonly 构建、开发配置初始化、SQLite 显式迁移、随机回环端口就绪检查、未认证 401 及认证创建/列表。`make contract-check`（72 个本地链接、392 个交付文件）与 `git diff --check` 通过。服务与本次安装/模块缓存/项目/日志目录已清理，未覆盖全局安装；未运行 Docker 或 race。

- 2026-10-15 00:49 双安装与 CLI 验收：当前源码构造本地文件模块代理，以隔离 GOBIN 执行 `go install ...@v0.0.0-installcheck` 与 `@latest`，安装版正确显示模块版本。安装版和源码构建版均在仓库外完成帮助、init、dry-run、重复生成拒绝、模块与资源生成及生成项目 readonly 构建。命令/CLI/生成器定向测试与 vet、格式、文档契约和 diff 检查通过；本机 Go 1.27.2 的 lint 因现有 Go 1.26.4 构建的 linter 无法解析新标准库而失败，改用仓库已有 Go 1.26.4 工具链后全仓 lint 为 0 issues。未验证 GitHub 远端安装，未发布、运行 Docker 或 race；任务临时目录已清理，共享缓存保留。

- 2026-10-15 00:34 公开文档和生成模板回归：默认生成项目已按 README 流程完成构建、开发配置初始化、SQLite 迁移、健康检查及认证创建/列表请求；`make fmt-check`、`make test GO_PACKAGES='./internal/templates'`、`make contract-check`（71 个本地链接、392 个交付文件）与 `git diff --check` 通过。仅模板包测试使用缓存；未运行 Docker、外部数据库或 race。

- 2026-10-15 00:06 文档导航整合验证：`make contract-check` 通过（99 个本地链接、393 个实际交付文件），Markdown 本地链接核对与 `git diff --check` 通过；旧模板文件名和已知过期部署/路线表述检索无命中。未运行代码、数据库、Docker 或 race 测试。

- 2026-10-14 23:41 Linux amd64 发行包验收：`make verify-package ARCHIVE=.tmp/dist/goseed_v0.0.0-d9.1_linux_amd64.tar.gz` 通过；发行包在 Debian `trixie-slim` 的 `linux/amd64` 模拟容器中运行 `--version`、`--help` 并生成 52 文件默认项目。生成项目以 Go 1.27.2、`GOOS=linux GOARCH=amd64 CGO_ENABLED=0` readonly 构建；容器中显式初始化 SQLite 迁移、启动服务，health/ready、未认证拒绝（401）和认证创建/列表通过。`make contract-check`（97 个本地链接、397 个交付文件）与 `git diff --check` 通过。测试运行于 Apple Silicon Docker amd64 模拟环境，不代表原生硬件；专用容器和本轮临时目录已清理。
