# 命令契约

本文件定义当前根命令的项目生成行为和后续业务生成命令契约。可执行能力以仓库 README 为准。

## 创建项目

根命令不带子命令时生成项目；内部模板渲染与根 CLI 使用相同的能力契约。

```bash
mkdir shop
cd shop
goseed
```

根命令不带参数时，在当前目录生成 HTTP + SQLite + JWT 项目。当前目录必须没有业务文件；可保留 `.git` Git 元数据和 `.DS_Store`、`Thumbs.db` 系统文件，不遍历或修改这些保留内容。其他已有文件会导致生成前失败。

HTTP、dual 与 Gateway 项目会同时生成内置 `example` 模块的 `api/openapi/example.yaml` 源文件和规范化的 `api/openapi/swagger.json` 汇总索引；纯 gRPC 项目不生成 HTTP OpenAPI 文件。生成器内部的索引更新计划会复核 `goseed.yaml`、模块源文件与目标字节，模块源变化时重建待写索引；若当前汇总索引与登记摘要不符则失败。此计划尚未作为公开 CLI 命令提供。

默认名称取目录名。首版项目与模块业务名称使用小写字母开头，后续允许小写字母和数字，对应 `^[a-z][a-z0-9]*$`；例如 `shop`、`shopapi`、`shop2`。不接受大写字母、连字符、下划线或路径分隔符；目录名不符合规则时需通过 `--name` 指定，不静默改名。

默认 Go 模块路径为 `example.com/<name>`，可以直接编译，但不代表可公开获取的模块。真实路径通过 `--module` 指定，不通过猜测 Git 远端来决定。

### 项目名、模块路径与目录

- `--name` 指定项目与服务名称，用于模板中的服务标识和名称相关默认配置；省略时使用当前目录名。
- `--module` 独立指定 `go.mod` 的模块路径及源码导入路径；省略时使用 `example.com/<name>`，其中 name 为最终项目名。
- 写入位置始终为当前目录。`--name` 不创建子目录，也不重命名当前目录；目录名、项目名和模块路径末段可以不同。
- 显式传入不合法的名称或模块路径时，在任何文件写入前失败；不自动转小写、删字符或修正路径。

以下为计划用法，目录名包含连字符时，可以显式指定合法项目名：

```bash
mkdir shop-service
cd shop-service
goseed --name shop --module github.com/setcreed/shop
```

该命令计划在 `shop-service` 当前目录生成项目，项目与服务名称为 `shop`，Go 模块路径为 `github.com/setcreed/shop`；不会再创建 `shop` 子目录。

### 参数

| 参数 | 支持值 | 默认 | 含义 |
|---|---|---|---|
| `--name` | 合法项目名 | 当前目录名 | 项目与服务名称 |
| `--module` | 合法 Go 模块路径 | `example.com/<name>` | 生成项目的导入路径 |
| `--db` | `sqlite`、`mysql`、`postgres`、`none` | `sqlite` | 选择持久化方案 |
| `--authz` | `casbin`、`none` | `none` | 选择通用授权组件 |
| `--redis` | 布尔值 | `false` | 增加 Redis 客户端与缓存示例 |
| `--transport` | `http`、`grpc`、`dual`、`gateway` | `http` | 选择业务协议入口 |
| `--dry-run` | 布尔值 | `false` | 校验并展示文件、依赖版本及冲突，不写入文件 |

```bash
goseed --module github.com/acme/shop
goseed --db=postgres --redis --authz=casbin --transport=dual
goseed --db=mysql --transport=gateway
goseed --db=none --redis --transport=grpc
```

示例中的 GitHub 路径仅展示模块参数格式，生成器不创建远端仓库、不提交或推送。

### 协议语义

- `http`：Gin HTTP Handler 直接调用 Biz。
- `grpc`：gRPC Handler 调用 Biz，不启动业务 HTTP 服务。
- `dual`：Gin HTTP 与 gRPC 独立 Handler 调用同一套 Biz，无内部 HTTP → gRPC 转发。
- `gateway`：gRPC + HTTP Gateway；HTTP 请求转发至 gRPC，使用同一套 RPC Handler，不重复生成同一业务的 Gin Handler。

### 能力组合规则

- Casbin 依赖固定 JWT 基础能力提供身份；授权默认关闭。
- Casbin + SQLite/MySQL/PostgreSQL：策略使用所选数据库，并生成对应迁移。
- Casbin + `--db=none`：生成只读策略文件；不生成策略写入或管理 API。
- Redis 独立于数据库选项，可与 `--db=none` 组合。
- JWT 是固定基础能力，不提供关闭认证的生成参数；Casbin 在 JWT 身份基础上增加授权。
- `--db=none`：移除 ORM、数据库驱动和数据库迁移；生成无持久化示例。
- 参数决定生成源码与依赖，不是服务运行时开关；已有项目不支持通过重复执行根命令补装或替换能力。
- TLS、pprof、Reflection、缓存是否必需等属于服务运行配置，不增加平行的生成参数。
- 未知参数、未知枚举值、无效组合直接报错。首版不提供覆盖开关。

## 生成业务模块

```bash
goseed gen module order
goseed gen module order --dry-run
```

`name` 使用与项目名称相同的规则 `^[a-z][a-z0-9]*$`，模块名在项目内唯一，并与资源共用一个名称空间。命令只在当前项目根目录运行，不接受覆盖项目能力的参数。

命令登记模块，并生成匹配当前协议的 Handler 适配骨架和 Biz 骨架；选择数据库时额外生成仓储接口与 Store 骨架，无数据库时不生成 Store 或虚假内存持久化。新 Handler 和 RPC/Gateway 登记不包含业务操作或虚构服务方法；路径登记只是模块占位，开发者需要补充真实接口后才能提供业务能力。生成的 Store 是扩展边界骨架，不表示已实现数据库读写。

模块名、资源名共享 `goseed.yaml` 中的模块清单；`example` 为模板预置名称，不能再次生成。元数据缺失、版本不支持、元数据校验和登记索引不一致、模块重复、能力不兼容或目标文件冲突时，在写入前失败。不接受用命令参数临时覆盖项目已有数据库和协议。

复杂业务由开发者补充，不自动生成可执行的支付、库存或订单状态规则。

## 生成 CRUD 资源

```bash
goseed gen resource product --spec specs/product.yaml
goseed gen resource product --spec specs/product.yaml --dry-run
goseed gen resource product --spec specs/product.yaml --migration
```

`--spec` 指向项目内 `specs/` 下的 YAML 文件。文件使用 UTF-8、单个 YAML 文档和版本 1；未知键、重复键、重复字段、别名/合并键和非预期类型均拒绝。字段声明如下：

```yaml
version: 1
fields:
  - name: title
    type: string
    required: true
    max_length: 120
  - name: enabled
    type: bool
  - name: retry_count
    type: int64
```

字段契约：

| 键 | 规则 |
|---|---|
| `version` | 必填整数，固定为 `1`。 |
| `fields` | 必填，1 至 32 项；按声明顺序生成。 |
| `name` | 必填小写 `snake_case`，匹配 `^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`；转换为 Go 导出字段时不得产生重复名称。 |
| `type` | 必填，限 `string`、`bool`、`int64`。 |
| `required` | 可选布尔值，默认 `false`；表示创建请求必须提供该字段，不表示禁止 `false`、`0` 或空字符串。 |
| `max_length` | 仅 `string` 可用的可选整数，范围 1 至 4096；省略时为 255。按 Unicode 字符数校验。 |

其他字段属性暂不支持，包括默认值、索引、唯一约束、枚举、数组、嵌套对象、数据库列名覆盖及字段级授权。资源名只从位置参数读取，必须是合法且未登记的名称；资源名与字段名均限 63 个 ASCII 字符。资源名 `examples` 与内置表冲突；字段映射的 Go 名不得与 ID、Owner、CreatedAt、UpdatedAt、TableName 冲突，例如 `i_d` 和 `table_name`，均在准备阶段拒绝。规范化后 API 集合路径为 `/v1/<resource>`，单项路径为 `/v1/<resource>/{id}`，不做隐式英文复数转换。

字段使用声明名作为 JSON 和 Proto 字段名，生成 Go 导出字段及对应存储列；客户端只能写入声明字段。`id`、`owner`、`created_at`、`updated_at` 为保留字段，由生成代码管理，不能在 spec 中声明或由请求赋值。owner 从已认证主体注入，所有读写和列表查询均按 owner 隔离，响应 DTO 不暴露可写 owner 字段。

创建请求必须提供 `required: true` 字段；可选字段未提供时采用 Go 零值。PATCH 通过字段是否出现区分未修改与写入零值，拒绝 `null`、未知键、重复键及空更新。字符串拒绝无效 UTF-8 与 NUL，不自动 trim。所有字段先做类型与范围校验，再进入 Biz；不从字段定义推断复杂业务规则。数据库与协议映射不得丢失 `int64` 精度或 PATCH 字段存在性语义。ID、分页、响应 DTO、权限目录及迁移/协议接入细节见 [资源接入方案](resource-generation.md) 。

`gen resource` 要求项目已选择 SQLite、MySQL 或 PostgreSQL；`--db=none` 时在写入前拒绝，并提示使用 `gen module`。命令生成模型、协议输入输出、基础校验、转换、完整主体隔离 CRUD、协议注册、接口文档和必要测试。`--migration` 生成当前数据库方言的迁移草稿；默认不生成迁移，命令不连接数据库、不应用 schema。未生成并显式执行迁移前，不声称新资源可运行。

选择 gRPC、dual 或 Gateway 时，需要匹配项目固定版本的协议生成工具；在写入前检查工具可用性。生成新的 Proto 与对应 Go 代码，不改写其他模块的手工 Proto、生成代码或业务文件。

## 业务生成登记与文件归属

模块工厂文件还提供不构造业务的 `<name>Permissions() modulePermissions`，声明当前协议的操作权限；工厂返回值必须与纯描述一致。选择 Casbin 时，生成索引的 `bootstrap.PermissionCatalog()` 汇总资源/操作并供启动与离线策略导入共用，声明本身不授予任何用户。组装入口版本 2 要求既有模块保留此函数；旧入口或缺声明时明确拒绝并提示人工升级，不自动重写手工文件。

模块命令同时创建 `internal/apiserver/bootstrap/module_<name>.go`，作为由开发者维护的显式依赖组装函数。固定启动入口通过强类型 `moduleDependencies` 将应用数据库、日志和所选缓存交给组装函数，所有模块仅构造一次；dual 的 HTTP 与 RPC Handler 共用同一 Biz。工厂返回构造错误时终止启动，由应用回收共享资源。生成索引汇总组装结果及协议登记，不覆盖手工组装代码。生成前检查组装版本、启动调用入口和登记索引的完整确定性投影；旧项目若缺少入口，应显式更新后再生成，不自动改写 `providers.go`。准备计划时记录启动文件及既有模块工厂的字节，执行前发现变化则拒绝写入。

`goseed.yaml` 是生成器维护的版本化元数据和模块清单，模块登记包含名称、类型（`module` 或 `resource`）；资源项另记录 `specs/` 下的 spec 相对路径与字段规范版本。新项目使用结构化登记项；已生成的版本 1 项目若使用旧字符串登记，读取时按 `type: module` 兼容解释。它不是服务运行配置。`internal/apiserver/bootstrap/modules_gen.go` 是由该清单确定性生成的启动登记索引，统一接入依赖组装并输出协议注册描述：HTTP 描述包含模块名、操作权限和 `func(*gin.RouterGroup)` 路由回调；gRPC 描述包含 FullMethod 权限和 `func(grpc.ServiceRegistrar)` 注册回调；Gateway 描述包含模块名、HTTP 路径、允许的 query 参数、JSON body 方法及 `func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error` 映射回调。对应协议不存在时裁剪其描述类型与依赖。`example` 作为内置登记项参与同一索引。

模块/资源源码、spec、Proto 源、迁移、模块 Markdown 文档和测试在首次生成后归开发者维护。各模块的 `api/openapi/<module>.yaml` 是 Swagger 2.0 源文件，`api/openapi/swagger.json` 是由所有源确定性合并的生成索引。输入必须是单个 YAML 映射文档，拒绝重复键、别名、merge key 和自定义标签；所有输入的 `swagger` 必须为 `2.0`。来源按模块名排序；`paths` 合并不同操作，同一路径的同一操作及全局重复的 `operationId` 冲突时报错；`definitions`、`parameters`、`responses` 和 `securityDefinitions` 按名称合并，重名内容必须一致；`tags` 按名称合并且重名内容必须一致。其他顶层字段必须语义一致。输出使用 UTF-8、两空格缩进的确定性 JSON，并以最终字节计算 SHA-256。生成器仅创建新业务文件，遇到任何已存在的模块文件均报冲突；后续增加其他模块时，只更新 `goseed.yaml`、`modules_gen.go` 和 `api/openapi/swagger.json` 这三个明确的生成登记/汇总文件。`modules_gen.go` 中记录模块清单和 Swagger 汇总的摘要；每次写入前验证清单、索引摘要及索引生成标记，任何不一致或手工修改均停止并报告修复对象，不覆盖或自动修复。命令不重新生成既有模块源码，不对既有数据库结构做差异推断。

新增注册前先读取并校验整个计划及所有目标文件，dry-run 展示新增源码、登记索引差异、迁移草稿和工具需求，不落盘。文件写入失败时仅撤回本次创建的文件；更新登记文件失败时恢复其原始字节，并报告无法恢复的对象。

## 生成器保护规则

- 写入位置限定在当前项目根目录；目标路径不能包含路径越界或穿过指向外部的符号链接。
- dry-run 不创建业务文件、不修改元数据、不运行数据库或服务。
- 先验证输入、版本、文件归属和全部冲突，再进行生成。工具或渲染错误发生在准备阶段，不留下部分业务文件。
- 新业务文件不覆盖已存在内容；需要更新的生成登记文件若被手工修改，先报冲突，不静默重写。
- 文件系统写入失败时尝试撤回本次写入，并报告仍残留的文件；不声称跨多个文件的操作天然原子。
- 生成器不自动更新模板版本、不执行 Git 操作、不安装全局依赖、不启动容器或服务。

## 服务开发与运行入口

生成项目提供 `make build`、`make test`、`make lint`、`make check`；各命令只执行其明确职责。默认 HTTP 模板的服务命令、开发初始化、迁移和 Token 规则见 [默认 HTTP 模板](template-http.md) 。

生成器的 `--transport`、`--db` 等选择生成哪些源码；业务服务的 `serve --address`、`--grpc-address` 覆盖当前模板的监听配置。后者在业务项目中执行，不是 `goseed` 根命令参数。

| 已实现模板 | 业务服务监听参数 | 默认监听地址 | 详细契约 |
|---|---|---|---|
| HTTP | `serve --address` 覆盖 HTTP | `127.0.0.1:8080` | [HTTP](template-http.md) |
| gRPC | `serve --address` 覆盖 gRPC | `127.0.0.1:9090` | [gRPC](template-grpc.md) |
| dual | `serve --address` 覆盖 HTTP，`--grpc-address` 覆盖 gRPC | HTTP 8080、gRPC 9090，均为回环地址 | [双协议](template-dual.md) |
| gateway | `serve --address` 覆盖 Gateway HTTP，`--grpc-address` 覆盖 gRPC | HTTP 8080、gRPC 9090，均为回环地址 | [Gateway](template-gateway.md) |

监听覆盖只适用于 `serve`；开发、迁移及策略命令拒绝这些参数。生产 gRPC 必须配置 TLS，dual／Gateway 两侧分别遵循各自协议配置；dual 并发排空，Gateway 先排空 HTTP 再关闭 RPC；数据库、Redis、Casbin 的使用契约见 [可选模板](template-options.md) 。

开发初始化可创建不入 Git 的本地配置、随机开发密钥和测试 Token；不得覆盖已有密钥或在生产环境开放开发签发入口。生成命令本身不创建真实环境密钥。

`goseed.yaml` 记录元数据版本、模板版本、项目名、模块路径、数据库、JWT、授权、Redis、协议和初始 `example` 模块；业务服务不依赖或读取该文件。该元数据供后续业务生成器检查能力兼容性。

根命令生成项目后，README 列出实际可执行的命令；`gen module` 已提供业务骨架生成。资源阶段已提供内部 CRUD 核心与 HTTP/dual 适配器、工厂、Swagger 源和模块文档渲染，但 `gen resource` 仍未开放写入：gRPC/Gateway 适配、登记与索引安全刷新、完整写入和公共 CLI 按施工清单继续实施。
