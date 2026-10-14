# 项目进度

## 当前状态

goseed 是独立的 Go 项目与业务代码生成器，主模块为 `github.com/setcreed/goseed`，Go 固定 1.26.4，默认分支 main，许可证 MIT。源码已公开至 https://github.com/setcreed/goseed ，origin 使用 HTTPS，main 跟踪 origin/main；公开源码不代表首版功能已完成。

代码基线为 `5b3f716`：初始项目生成与 gen module 可用，支持四协议、四存储及可选 Redis/Casbin，共 64 种初始项目组合。资源阶段已交付字段解析、只读计划、权限目录、迁移/schema、CRUD 核心及 HTTP/gRPC/dual 源渲染；R01 至 R07 已验收。公共 gen resource、Gateway 资源适配与安全写入尚未交付。

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
| D6 模块与资源 | 进行中 | gen module 已交付；资源 R01 至 R07 已验收，R08 至 R10 待推进 |
| D7 首版收口 | 待开始 | 全部需求覆盖、文档/部署边界、交付验证 |

## 下一步施工顺序

1. **R08**：Gateway 资源映射、ProtoJSON 输入、转发安全链、多模块 Swagger 聚合与复现。
2. **R09**：完整 Prepare/Write、元数据与登记更新、过期/冲突拒绝、回滚、旧项目兼容及公共 gen resource CLI。
3. **R10**：48 种合法资源组合、16 种无数据库拒绝、原有 64 项目组合；三数据库、真 Redis/Casbin、协议/容器及文件保护完整回归。
4. **D7**：首版范围核验与使用说明收口。源码公开可以先于首版功能完成，README 按当前真实能力表述。

详细修改范围、依赖与失败门槛统一见 [D6 清单](docs/implementation.md#d6-资源生成施工清单)。

## 已知缺口与验证边界

- R06 已验收：Swagger `/v1/<name>` 路径与 Handler 一致，请求/响应 schema 与 DTO 对齐；curl 示例展开 `$TOKEN`、接受临时 `API_URL` 并在生成项目集成测试中实际执行。SQLite Casbin 使用真实 Manager/策略存储验证默认拒绝、允许、权限不足及授权先于 Redis Check。源码与验收入口：`internal/templates/resource_core.go`、`internal/templates/resource/http_integration_test.go.tmpl`。
- R07 已验收：HTTP/RPC/dual 资源工厂共享一次 Biz 构造；两个资源的固定版 Proto 复现、独立 Go 客户端导入、真实 TLS RPC 主体隔离/边界/零值更新，以及 dual 跨协议读写通过。资源 API 仍只读预览，文件写入和登记更新留给 R09。
- SchemaStore 是只读 schema 原型；CoreFiles 提供完整 Store，不得混用输出路径。当前 ResourcePlan 没有 Write/公共 CLI。
- 模块组装版本 2 只表示纯权限/单次组装，不能代替 Context/schema 和多模块协议兼容检查。R09 必须完整核验旧项目。
- Gateway 资源映射与多模块文档聚合仍未实施，属于 R08。
- 历史集成验证曾出现排空超时和宿主 Go/GOROOT/linter 不匹配；R10 按当前代码重新核验，不能靠旧记录声明完整矩阵通过。
- go-redis 取消、pgx 人工延迟 Write 竞争边界仍按 [依赖记录](docs/dependencies.md) 保留；固定版本不意味着上游问题消失。
- 仓库采用 MIT；第三方依赖及生成代码仍保留适用许可。源码仓库已经公开，首版交付仍按上述施工顺序推进。

## 最近完成

- 2026-10-14 14:00 D6-R07 gRPC/dual 资源验收完成：新增资源 Proto、RPC Handler 与精确 FullMethod 权限；dual 工厂只创建一次 Store/Biz 并供 HTTP/RPC 共用。Proto 脚本已支持多模块排序复现；公共资源写入、登记更新仍由 R09 实施。

- 2026-10-14 13:38 D6-R06 HTTP 资源验收完成：修正 Swagger 路径、严格请求与完整响应 schema，curl 示例支持 Token/临时服务地址并由集成测试执行；真实 SQLite Casbin 策略覆盖默认拒绝、允许、权限不足及 Redis 检查顺序。R07 成为下一开发项。

- 2026-10-12 16:40 使用 gh 创建公开仓库 setcreed/goseed，推送 main 并建立 origin/main 跟踪；保留全部本地 Git 历史。README 与开发指南补齐真实克隆地址，可在新电脑恢复开发。

- 2026-10-12 16:19 公开源码与换机文档准备：重写公开 README 和当前进度快照，按从零建设整理 D0 至 D7 及详细 D6 施工单，新增源码导航/环境恢复指南与 MIT LICENSE。去除特定外部项目名称检查规则、个人账号示例和旧会话依赖；保留现有代码、版权与 Git 历史。核对 R06 时发现文档/权限验收缺口，已回到待补齐状态。尚未创建远端或推送。

- 2026-10-12 15:57 R06 HTTP 源渲染提交 `2704b39`：提供 Handler、schema 核验工厂、Swagger/模块说明与 HTTPFiles 副本；只读准备检查多源合并。完整验收状态按上面的缺口执行，本条不认定全部 R06 门槛通过。
- 2026-10-12 15:23 R05 CRUD 核心提交 `de13630`：主体隔离 Model/Biz/Store、字段出现性、零值更新、分页、UTC DTO 与更新事务回滚；没有增加运行依赖。
- 2026-10-12 13:03 R04 提交 `a123019`：迁移版本/应用集合核验、SQL 草稿、资源只读 schema 与启动 Context；生成器不执行数据库操作。

## 最近验证

- 2026-10-14 14:00 R07：锁定 protoc 36.2 与插件安装后，双资源 gRPC/dual 集成项目通过 Proto 多模块二次逐字节复现、独立客户端导入、readonly 测试、vet、lint 与 build。真实 TLS RPC 覆盖两个资源、alice/bob 隔离、显式零值、必填/未知字段及 ID/分页/deadline 错误；dual 验证 RPC 创建 → HTTP 读取/更新 → RPC 读回。`make test GO_PACKAGES='./internal/templates ./internal/generator'`、`make protocol-check`、`make fmt-check`、`make lint`、`make contract-check`（78 个本地链接、365 个交付文件）与 `git diff --check` 通过。运行本地监听验收需沙箱外执行；测试临时项目已清理。

- 2026-10-14 13:38 R06：`make test GO_PACKAGES='./internal/templates ./tests/integration -run TestResource -count=1 -v'` 与 `make test GO_PACKAGES='./tests/integration -run TestResourceCoreProjects -count=1 -v'` 通过。五个 SQLite 资源组合生成项目完成 readonly race 测试、HTTP Handler 测试、bootstrap 编译、实际执行模块 curl、vet、固定 linter 与 build；Casbin+Redis 用例通过真实策略及检查顺序断言。根仓库定向 vet、`make lint`（0 issues）、`make contract-check`（78 个本地链接、361 个交付文件）、`make fmt-check` 与 `git diff --check` 通过。测试临时项目由套件清理；`.tmp` 共享缓存保留。

- 2026-10-14 13:28 R06 补齐 Swagger `/v1` 路径、请求/响应 schema、可执行 curl 示例与 SQLite Casbin Manager 验收夹具；`make contract-check`（78 个本地链接、361 个交付文件）和 `make fmt-check` 通过。两组 Go 测试未启动：固定 Go 1.26.4 工具链需从 `goproxy.cn` 下载，但当前 DNS 无法解析；本机 Go 1.27.2 不符合项目固定版本，未用于测试。R06 仍待用 Go 1.26.4 执行模板与资源集成测试。

- 2026-10-12 16:40 远端发布：gh 查询确认仓库为 PUBLIC、默认分支 main；GitHub API 返回的 main 提交与首次推送的本地 HEAD e895372 一致。认证有效；未新增构建产物或任务缓存。

- 2026-10-12 16:19 文档与公开准备：make contract-check（78 个本地链接、361 个交付文件）、git diff --check 通过；Make dry-run 核对构建、工具安装和核心测试入口。当前交付文件名称/个人路径扫描无候选；Git 历史 826 个可达 blob 的私钥/GitHub Token/AWS 访问键特征扫描无候选，不代表完整凭据审计。未重跑业务/数据库/容器测试，未在新电脑验证；未创建任务临时项目或缓存，既有本地产物未删除。

- 2026-10-12 15:57 R06：核心包测试及五类 SQLite 生成项目 readonly race/vet/lint/build 通过；基础与接口替身组合通过回环 HTTP Server。真实 Casbin 策略和 Swagger 最终路径未被该轮验收覆盖。文档契约、格式、vet、lint（0 issues）和 diff 检查通过，任务目录已清理。
- 2026-10-12 15:23 R05：五个 SQLite 核心项目、真实 MySQL/PostgreSQL 四个资源组合通过主体 CRUD、零值/int64、取消、并发、事务回滚与独立应用账号验收；template-db 七组共 18 个组合无跳过通过。未执行完整 64 项目矩阵或新增真实 Redis 验收。
- 2026-10-12 13:03 R04：SQLite 与真实 MySQL/PostgreSQL 迁移/schema、异常历史、宽表、Unicode/排序与取消验收通过；确认 MySQL 部分 DDL 保留边界。任务项目及本次容器/卷/网络已清理。
