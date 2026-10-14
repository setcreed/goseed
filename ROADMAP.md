# 项目进度

## 当前状态

goseed 是独立的 Go 项目与业务代码生成器，主模块为 `github.com/setcreed/goseed`，Go 固定 1.26.4，默认分支 main，许可证 MIT。源码已公开至 https://github.com/setcreed/goseed ，origin 使用 HTTPS，main 跟踪 origin/main；公开源码不代表首版功能已完成。

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
| D7 首版收口 | 进行中 | 需求覆盖与文档收口进行中；`make race` 按用户决定暂缓 |

## 下一步施工顺序

1. **D7（进行中）**：完成首版范围核验与使用说明收口；源码公开可以先于首版功能完成，README 按当前真实能力表述。race 暂缓，不能记录为通过。

详细修改范围、依赖与失败门槛统一见 [D6 清单](docs/implementation.md#d6-资源生成施工清单)。

## 已知缺口与验证边界

- R06 已验收：Swagger `/v1/<name>` 路径与 Handler 一致，请求/响应 schema 与 DTO 对齐；curl 示例展开 `$TOKEN`、接受临时 `API_URL` 并在生成项目集成测试中实际执行。SQLite Casbin 使用真实 Manager/策略存储验证默认拒绝、允许、权限不足及授权先于 Redis Check。源码与验收入口：`internal/templates/resource_core.go`、`internal/templates/resource/http_integration_test.go.tmpl`。
- R07 已验收：HTTP/RPC/dual 资源工厂共享一次 Biz 构造；两个资源的固定版 Proto 复现、独立 Go 客户端导入、真实 TLS RPC 主体隔离/边界/零值更新，以及 dual 跨协议读写通过。R09 随后开放完整资源写入及登记更新。
- SchemaStore 是只读 schema 原型；CoreFiles 提供完整 Store，不得混用输出路径。ResourcePlan 提供完整 Write 与公共 CLI；R10 的生成项目组合和运行行为已完成验收。
- 模块组装版本 2 表示纯权限/单次组装；R09/R10 已覆盖 Context/schema、旧项目兼容及多模块协议复现。
- R08 已交付 Gateway RPC 资源映射、ProtoJSON 说明及多模块协议/Swagger 复现；R09 已开放 `gen resource` 并完成完整写入回滚；R10 完成全矩阵和外部服务验收。
- D6 全仓 `make check`、真实服务矩阵和生成产物集成验证均通过。用户要求暂缓全仓 `make race`；它曾在集成测试运行 919.608 秒时中断，D7 不把它记录为通过。
- go-redis 取消、pgx 人工延迟 Write 竞争边界仍按 [依赖记录](docs/dependencies.md) 保留；固定版本不意味着上游问题消失。
- 仓库采用 MIT；第三方依赖及生成代码仍保留适用许可。源码仓库已经公开，首版交付仍按上述施工顺序推进。

## 最近完成

- 2026-10-14 17:06 D7 文档与命令入口核对：更新开发指南中仍描述 R06–R09 未完成的源码导航、资源 CLI、真实策略/缓存验收与后续施工顺序；核对 README 与当前 `gen module`/`gen resource` 命令契约一致。D7 继续进行，全仓 race 按用户决定暂缓。

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

- 2026-10-14 17:06 D7 文档与入口验证：`make contract-check`（79 个本地链接、372 个交付文件）、`git diff --check`、`make build` 通过；构建产物实际响应 `goseed --help`、`--version`、`gen module --help` 与 `gen resource --help`，资源参数显示 `--spec`、`--dry-run` 和 `--migration`，与 README 示例一致。全仓 race 按用户决定暂缓。

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
