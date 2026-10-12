# 资源生成接入方案

本文是 D6-R01 定稿的接口与技术决策，供 D6-R02 至 R10 实施；不是已实现能力说明。施工顺序见 [实施方案](implementation.md#d6-剩余施工清单) ，当前状态见 [ROADMAP](../ROADMAP.md) 。字段 DSL 与公共命令仍以 [命令契约](cli.md#生成-crud-资源) 为准。

## 生成计划与文件范围

内部入口确定为 `PrepareResource(root, name string, options ResourceOptions) (ResourcePlan, error)`；`ResourceOptions` 只包含 `Spec string` 与 `Migration bool`，项目能力从校验后的元数据读取，不接受第二套数据库/协议覆盖参数。计划拥有输入快照、新增目标、登记更新、迁移与协议工具需求、冲突信息；查询与预览返回副本，不能由调用方修改已验证输入。

R02 先实现不落盘的目标规划、输入快照、预览和 `Revalidate()`；不提供 Write 或公共 CLI，不把未渲染目标冒充完整交付。R05 至 R08 提供实际源码与协议产物后，R09 补齐完整准备/Write；执行前再次校验同一份计划。最终预览必须包含登记差异，R02 的预览明确标为目标规划。

| 输出 | 路径/规则 | 所有权 |
|---|---|---|
| 模型与字段存在性 | `internal/apiserver/model/<name>.go`、对应测试 | 新建，开发者维护 |
| Biz、DTO、校验与转换 | `internal/apiserver/biz/<name>/biz.go`、对应测试 | 新建，开发者维护 |
| SQL Store | `internal/apiserver/store/<name>/store.go`、对应测试 | 新建，开发者维护 |
| HTTP Handler | `internal/apiserver/handler/http/<name>/handler.go`、对应测试；仅 HTTP/dual | 新建，开发者维护 |
| RPC Handler | `internal/apiserver/handler/grpc/<name>/handler.go`、对应测试；gRPC/dual/Gateway | 新建，开发者维护 |
| 模块工厂与纯描述 | `internal/apiserver/bootstrap/module_<name>.go` | 新建，开发者维护 |
| Proto | `api/proto/<name>/v1/<name>.proto` | 新建，开发者维护 |
| Go 协议产物 | `pkg/api/<name>/v1/<name>.pb.go`、`<name>_grpc.pb.go`；Gateway 另有 `<name>.pb.gw.go` | 新资源命令只创建；显式协议复现可更新声明的产物 |
| Gateway 配置 | `api/proto/<name>/v1/http.yaml`、`openapi.yaml` | 新建，开发者维护 |
| Swagger 源 | `api/openapi/<name>.yaml`；纯 gRPC 裁剪 | HTTP/dual 为手工源；Gateway 为明确标识的协议生成源 |
| 模块使用文档 | `docs/<name>.md` | 新建，开发者维护 |
| 可选迁移 | `migrations/<version>_<name>.sql`；版本至少五位补零 | 仅 `--migration` 新建，开发者审阅维护 |
| 登记/汇总 | `goseed.yaml`、`internal/apiserver/bootstrap/modules_gen.go`、有 HTTP 文档时的 `api/openapi/swagger.json` | 校验后更新，保留既有业务文件 |

spec 是用户提供的输入，不复制或改写。业务名称使用现有小写字母/数字规则，`example` 保留；表名显式使用资源原名，不交给 ORM 复数化。Go 导出名、Proto 包与 Service 名、文件路径从同一份已验证名称/字段映射产生，遇到碰撞拒绝，不自动加后缀。

输入快照包括项目清单、登记投影、既有工厂和启动入口、spec、OpenAPI 源/索引、所用协议脚本与工具身份、SQL 迁移目录和文件。现有受保护输入的父路径与最终文件均检查符号链接；输出独占创建。准备后改变任一相关输入、新增目标或同号迁移时 Revalidate 拒绝。协议工具的检查/执行临时数据在受控临时位置处理，结束清理，不改变业务项目树。

## CRUD 与协议语义

| 项目 | 确定行为 |
|---|---|
| 操作 | Create、Get、List、Update、Delete；权限 create/read/read/update/delete |
| HTTP 路径 | `/v1/<name>`、`/v1/<name>/{id}`，不推断复数 |
| ID | Go/Proto 使用 uint64；有效范围 1 至 9223372036854775807，与 SQL 有符号主键兼容 |
| 分页 | page 默认 1，范围 1 至 100000；page_size 默认 20，范围 1 至 100；只允许这两个单值参数 |
| 排序与列表 | 固定 id 升序；空列表输出空数组；计数、记录均按 owner 限定 |
| 字段 | string/bool/int64；SQL 非空，未提供的可选字段为 Go 零值；不用 float64 中转整数 |
| 字符串 | 不自动 trim；空字符串合法；拒绝无效 UTF-8 和 NUL，按 Unicode 字符数校验 max_length |
| 创建/PATCH | 使用指针/Proto optional 保存出现性；required 只要求创建时出现；PATCH 无字段、null、未知/重复键拒绝 |
| 归属 | owner 从 JWT 主体取得，Biz/Store 双重约束；外主体单项操作按不存在处理；响应 DTO 不暴露 owner |
| 时间 | UTC；HTTP 使用 RFC3339Nano，RPC 使用 Timestamp，Gateway 遵循 ProtoJSON；不直接返回含 owner 的持久化模型 |
| HTTP 成功 | Create 201、Get/List/Update 200、Delete 204；Gateway 保持现有成功统一 200 与 ProtoJSON 格式 |
| RPC | 独立 `<Name>Service`；精确 FullMethod 登记；List 分页和所有可写字段保留出现性；不增加业务流 |

API DTO 在 Biz 包定义，不依赖 Gin、gRPC 或 GORM；Handler 只做协议解析/转换和错误映射。Repository 使用模型与字段修改集合；Store 复用传入数据库，所有 SQL 使用 Context 和 owner 条件，PATCH 用明确字段 map 保留零值。保持简单 CRUD，不抽象通用框架、不推断字段间业务约束。

## 纯模块描述与权限目录

模块工厂文件增加不访问外部服务的 `<name>Permissions() modulePermissions`，描述所选协议的 HTTP method → operation 与 RPC FullMethod → operation。模块工厂与生成登记从同一函数取值，避免另写一份权限字符串；内置模块由固定启动层提供相同纯描述。描述只能声明权限，不构造 Store/Biz/Handler，不访问配置、数据库或 Redis。

选择 Casbin 时，生成索引提供 `bootstrap.PermissionCatalog() (authz.Catalog, error)`：调用所有模块的纯描述，按清单绑定资源名并收集操作，验证重复 FullMethod、非法名称/操作与协议描述一致性。目录不读取运行时 goseed.yaml。普通模块初始描述为空，开发者添加业务时维护自己的纯描述，不会自动获得 CRUD 权限。

`authz.Catalog` 是显式资源/操作目录，authz 构造时校验并复制，Manager 内部保存不可由调用者改写的快照。目录显式传给策略校验、Manager、SQL/File Store 和导入入口；启动与离线 `policy replace` 使用同一 `PermissionCatalog()`。不得设置全局可变权限目录，或通过启动整套 App 获取目录。

路由/RPC 权限从同一纯描述登记，JWT → Casbin → Biz/依赖检查顺序不变；未知资源、操作或 FullMethod 默认拒绝。无数据库文件策略只读、SQL 全量替换事务和失败保留旧快照不变。启用资源目录不授予默认角色或用户权限，策略仍需显式导入并重启实例。

## 迁移版本与资源 schema

迁移版本不再是模板固定常量。`migrations.ExpectedVersion() (int64, error)` 从 embed FS 的 SQL 文件名解析正整数版本，检查文件名和版本重复，返回最大值；保留现有 Goose SQL 格式，不要求号码连续，也不创建额外版本登记文件。生成草稿采用现有最大版本加 1，检查溢出及同号冲突。

服务启动只读核对实际迁移记录与期望版本、内置基础表及所选策略表；每个资源 Store 提供 `CheckSchema(context.Context) error`，工厂在构造 Biz 前调用，校验资源表、必要列、类型/非空和长度约束的方言兼容性。启动依赖增加限时 Context，传入所有工厂；核验失败回收应用资源，返回不含 DSN 的错误，不自动修表。

`--migration` 草稿为当前数据库创建明确表名、主键、owner、时间与声明字段；SQLite/MySQL/PostgreSQL 使用各自已锁定驱动和方言，不由客户端选择。迁移只有显式审阅执行后才改变数据库，不能以 schema 校验触发隐式迁移。

不带 `--migration` 时可以生成源码；模块文档说明表结构和人工显式迁移入口。用户可自行编写迁移或管理外部 schema，服务仍必须通过同样的只读核验；没有资源表的项目拒绝启动，不能宣称生成即可运行。不提供已有 schema 差异推断或覆盖历史 SQL。

## Redis 与运行依赖

新资源首版 CRUD 不生成缓存键、回填和失效逻辑，全部读写走 Store，避免把示例缓存语义复制成通用框架。选了 Redis 时，Biz 只接收最小 `Check(context.Context) error` 依赖接口，在认证/授权与输入校验之后、Store 之前检查；可选模式允许数据库回源，必需模式返回现有 Unavailable。Redis 的启动、就绪、预算及关闭继续复用应用共享 Client。

Biz 不接收 Redis SDK、缓存配置或 moduleDependencies；未选 Redis 时接口/构造参数/检查代码裁剪。以后若要给某个资源增加缓存，由开发者针对领域一致性显式维护，不扩大本次生成器需求。

## 协议与 OpenAPI 复现

协议输入以 `api/proto/<module>/v1/` 的源文件显式发现，稳定排序；每个 Proto 使用完整模块导入路径和唯一 Proto 文件身份，不能让不同目录下同名文件互相遮蔽。普通行为骨架没有 Proto 时可跳过，已有 Proto 模块必须具备对应所需映射；输入不使用服务运行时元数据。

gRPC/dual 用所锁定 protoc、Go/Go-gRPC 插件；Gateway 再使用现有 Gateway/OpenAPI 插件。检查项目内工具实际版本和二进制构建身份；缺少/错版明确失败，不自动下载或全局安装。新资源命令仅生成新资源协议产物；显式 `make proto-generate` 可更新已发现模块的协议产物，必须先全部生成成功再写回，不重写手工 Proto、工厂或业务文件。

Gateway 的模块 Swagger 是明确生成源：按 Proto + http/openapi 配置重新生成，不把它当手工文档编辑入口。脚本仅更新本次协议生成源，HTTP/dual 手工 OpenAPI 源保持原样；聚合使用与生成器相同的严格合并/规范化/冲突规则，更新 Swagger 与登记摘要时校验原始字节并失败恢复。不能复制内置 Swagger 覆盖全项目汇总。

Gateway 模板增加项目内 `scripts/openapi/main.go` 开发工具，复用已锁定且当前模板已包含的 `go.yaml.in/yaml/v3 v3.0.5`，承载严格合并和受保护汇总更新；`proto.py` 显式调用。应用运行包不导入此工具，业务运行不依赖它，不要求安装 goseed 或 Python YAML 库。必要时把已有 YAML 依赖声明为直接工具依赖，版本不变；相同语料与生成器输出逐字节对照，防止两份实现漂移。

## 兼容与验收反例

新增纯描述与 Context 工厂入口时，模块组装版本由 1 升到 2；元数据/字段规范仍为版本 1。R03 先交付纯描述与目录并升级对应入口，R04 补齐 Context/schema，R07/R08 升级协议脚本；R09 对所选能力检查全部必要入口，再开放完整资源命令。R02 基础计划只核对当前已交付组装入口，后续任务同步增强校验，不把基础预览当完整兼容判定。

不兼容旧项目在写入前提示具体缺少入口及人工升级范围，不自动覆盖 providers、旧工厂、迁移或脚本；已有生成索引的完整确定性投影继续校验。只添加登记标记不等于兼容，必须检查调用与结构。

| 反例 | 预期 |
|---|---|
| 无数据库生成资源、与普通模块重名、非法/保留字段 | 准备阶段拒绝，无业务写入 |
| 描述只绑定其他资源、未登记 FullMethod、任意未知策略资源 | 默认拒绝，不靠名称正则放行 |
| required 字段缺失、PATCH 空/null/重复键、写入 owner | 输入拒绝；false/0/空字符串明确出现时可写 |
| 读取/修改另一主体记录 | 不返回数据，不改变记录，统一不存在 |
| 新资源未迁移、迁移版本超出/落后、缺列或不兼容类型 | 启动只读核验失败，不隐式修 schema |
| 计划后修改 spec/工厂/文档/工具/迁移目录，手工改登记 | 过期/冲突拒绝，不覆盖旧文件 |
| 第二资源加入后执行协议生成 | 两资源与内置模块的协议/Swagger 全部保留，摘要一致 |
| 生成/写入/恢复中途失败 | 原始字节恢复；恢复失败列出残留对象 |

上述决策的实际测试、真实数据库/Redis 和四协议组合范围按施工清单逐项验收；未实施/验证前不写入 README 的当前能力。
