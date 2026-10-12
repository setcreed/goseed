# 项目进度

## 当前状态

goseed 是独立的 Go 项目与业务代码生成器，主模块为 `github.com/setcreed/goseed`，Go 固定 1.26.4，默认分支 main，许可证 MIT。当前尚未配置 Git 远端；公开源码准备不代表已经上传或发布。

代码基线为 `2704b39`：初始项目生成与 gen module 可用，支持四协议、四存储及可选 Redis/Casbin，共 64 种初始项目组合。资源阶段已经交付字段解析、只读计划、权限目录、迁移/schema、CRUD 核心与 HTTP 源渲染；公共 gen resource、完整 RPC/Gateway 资源适配与安全写入尚未交付。

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
| D6 模块与资源 | 进行中 | gen module 已交付；资源 R01 至 R05 已验收，R06 适配器已提交但完整验收待补 |
| D7 首版收口 | 待开始 | 全部需求覆盖、文档/部署边界、交付验证 |

## 下一步施工顺序

1. **补齐 R06**：修正资源 Swagger 路径和可执行 curl 示例，使用真实 Casbin Manager/策略验收权限与依赖顺序，核对响应 schema。已提交 HTTP 渲染不等于完整安全/文档验收。
2. **R07**：资源 Proto、RPC Handler、精确 FullMethod 权限、完整 dual 工厂及共享 Biz；协议脚本发现多个模块并逐字节复现。
3. **R08**：Gateway 资源映射、ProtoJSON 输入、转发安全链、多模块 Swagger 聚合与复现。
4. **R09**：完整 Prepare/Write、元数据与登记更新、过期/冲突拒绝、回滚、旧项目兼容及公共 gen resource CLI。
5. **R10**：48 种合法资源组合、16 种无数据库拒绝、原有 64 项目组合；三数据库、真 Redis/Casbin、协议/容器及文件保护完整回归。
6. **D7**：首版范围核验与使用说明收口。源码公开可以先于首版功能完成，README 按当前真实能力表述。

详细修改范围、依赖与失败门槛统一见 [D6 清单](docs/implementation.md#d6-资源生成施工清单)。

## 已知缺口与验证边界

- R06 Swagger 当前生成 `/<name>`，基础源 basePath 是 `/`，真实 Handler 为 `/v1/<name>`；合并成功没有验证最终路径一致性。入口：`internal/templates/resource_core.go` 的 resourceOpenAPI。
- 资源模块说明用单引号包裹含 `$TOKEN` 的 Authorization，Shell 不展开变量；字符串示例固定为 `value`，可能超过字段 max_length。入口：同文件的 resourceHTTPGuide。
- R06 的 Casbin HTTP 夹具使用允许所有请求的 Authorizer，Redis 使用 Check 替身；不能据此声明实际策略允许/拒绝、缓存或依赖故障已验收。入口：`resource/http_integration_test.go.tmpl`。
- HTTPFiles 为 dual 返回的工厂目前只有 HTTP 成员；完整 dual 资源工厂与 RPC 共享 Biz 属于 R07，不能把该文件直接当完整 dual 交付物。
- SchemaStore 是只读 schema 原型；CoreFiles 提供完整 Store，不得混用输出路径。当前 ResourcePlan 没有 Write/公共 CLI。
- 模块组装版本 2 只表示纯权限/单次组装，不能代替 Context/schema 和多模块协议兼容检查。R09 必须完整核验旧项目。
- 协议复现脚本目前仍围绕内置模块；Gateway 复现的单源汇总覆盖问题由 R07/R08 处理。
- 历史集成验证曾出现排空超时和宿主 Go/GOROOT/linter 不匹配；R10 按当前代码重新核验，不能靠旧记录声明完整矩阵通过。
- go-redis 取消、pgx 人工延迟 Write 竞争边界仍按 [依赖记录](docs/dependencies.md) 保留；固定版本不意味着上游问题消失。
- 仓库 MIT 已选择；第三方依赖及生成代码仍保留适用许可。发布目标和远端地址尚未确定，未执行 push。

## 最近完成

- 2026-10-12 16:19 公开源码与换机文档准备：重写公开 README 和当前进度快照，按从零建设整理 D0 至 D7 及详细 D6 施工单，新增源码导航/环境恢复指南与 MIT LICENSE。去除特定外部项目名称检查规则、个人账号示例和旧会话依赖；保留现有代码、版权与 Git 历史。核对 R06 时发现文档/权限验收缺口，已回到待补齐状态。尚未创建远端或推送。

- 2026-10-12 15:57 R06 HTTP 源渲染提交 `2704b39`：提供 Handler、schema 核验工厂、Swagger/模块说明与 HTTPFiles 副本；只读准备检查多源合并。完整验收状态按上面的缺口执行，本条不认定全部 R06 门槛通过。
- 2026-10-12 15:23 R05 CRUD 核心提交 `de13630`：主体隔离 Model/Biz/Store、字段出现性、零值更新、分页、UTC DTO 与更新事务回滚；没有增加运行依赖。
- 2026-10-12 13:03 R04 提交 `a123019`：迁移版本/应用集合核验、SQL 草稿、资源只读 schema 与启动 Context；生成器不执行数据库操作。

## 最近验证

- 2026-10-12 16:19 文档与公开准备：make contract-check（78 个本地链接、361 个交付文件）、git diff --check 通过；Make dry-run 核对构建、工具安装和核心测试入口。当前交付文件名称/个人路径扫描无候选；Git 历史 826 个可达 blob 的私钥/GitHub Token/AWS 访问键特征扫描无候选，不代表完整凭据审计。未重跑业务/数据库/容器测试，未在新电脑验证；未创建任务临时项目或缓存，既有本地产物未删除。

- 2026-10-12 15:57 R06：核心包测试及五类 SQLite 生成项目 readonly race/vet/lint/build 通过；基础与接口替身组合通过回环 HTTP Server。真实 Casbin 策略和 Swagger 最终路径未被该轮验收覆盖。文档契约、格式、vet、lint（0 issues）和 diff 检查通过，任务目录已清理。
- 2026-10-12 15:23 R05：五个 SQLite 核心项目、真实 MySQL/PostgreSQL 四个资源组合通过主体 CRUD、零值/int64、取消、并发、事务回滚与独立应用账号验收；template-db 七组共 18 个组合无跳过通过。未执行完整 64 项目矩阵或新增真实 Redis 验收。
- 2026-10-12 13:03 R04：SQLite 与真实 MySQL/PostgreSQL 迁移/schema、异常历史、宽表、Unicode/排序与取消验收通过；确认 MySQL 部分 DDL 保留边界。任务项目及本次容器/卷/网络已清理。
