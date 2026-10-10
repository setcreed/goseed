# 命令契约

本文件是待实现的首版命令规范。当前仓库没有可执行的 `goseed`。

## 创建项目

```bash
mkdir shop
cd shop
goseed
```

根命令不带参数时，在当前目录生成 HTTP + SQLite + JWT 项目。当前目录必须没有业务文件；可保留 `.git` Git 元数据和 `.DS_Store`、`Thumbs.db` 系统文件，不遍历或修改这些保留内容。其他已有文件会导致生成前失败。

默认名称取目录名。首版项目与模块业务名称使用小写字母开头，后续允许小写字母和数字；目录名不符合规则时需通过 `--name` 指定，不静默改名。

默认 Go 模块路径为 `example.com/<name>`，可以直接编译，但不代表可公开获取的模块。真实路径通过 `--module` 指定，不通过猜测 Git 远端来决定。

### 参数

| 参数 | 支持值 | 默认 | 含义 |
|---|---|---|---|
| `--name` | 合法项目名 | 当前目录名 | 项目与服务名称 |
| `--module` | 合法 Go 模块路径 | `example.com/<name>` | 生成项目的导入路径 |
| `--db` | `sqlite`、`mysql`、`postgres`、`none` | `sqlite` | 选择持久化方案 |
| `--auth` | `jwt`、`none` | `jwt` | 选择身份认证 |
| `--authz` | `casbin`、`none` | `none` | 选择通用授权组件 |
| `--redis` | 布尔值 | `false` | 增加 Redis 客户端与缓存示例 |
| `--transport` | `http`、`grpc`、`dual`、`gateway` | `http` | 选择业务协议入口 |
| `--dry-run` | 布尔值 | `false` | 校验并展示计划，不写入文件 |

```bash
goseed --module github.com/chengwz/shop
goseed --db=postgres --redis --authz=casbin --transport=dual
goseed --db=mysql --transport=gateway
goseed --db=none --auth=none
```

示例中的 GitHub 路径仅展示模块参数格式，生成器不创建远端仓库、不提交或推送。

### 协议语义

- `http`：Gin HTTP Handler 直接调用 Biz。
- `grpc`：gRPC Handler 调用 Biz，不启动业务 HTTP 服务。
- `dual`：Gin HTTP 与 gRPC 独立 Handler 调用同一套 Biz，无内部 HTTP → gRPC 转发。
- `gateway`：gRPC + HTTP Gateway；HTTP 请求转发至 gRPC，使用同一套 RPC Handler，不重复生成同一业务的 Gin Handler。

### 能力组合规则

- Casbin 依赖 JWT 提供身份；`--auth=none --authz=casbin` 首版报错，不静默关闭授权。
- Casbin + SQLite/MySQL/PostgreSQL：策略使用所选数据库，并生成对应迁移。
- Casbin + `--db=none`：生成只读策略文件；不生成策略写入或管理 API。
- Redis 独立于数据库选项，可与 `--db=none` 组合。
- `--db=none`：移除 ORM、数据库驱动和数据库迁移；生成无持久化示例。
- 参数决定生成源码与依赖，不是服务运行时开关；已有项目不支持通过重复执行根命令补装或替换能力。
- TLS、pprof、Reflection、缓存是否必需等属于服务运行配置，不增加平行的生成参数。
- 未知参数、未知枚举值、无效组合直接报错。首版不提供覆盖开关。

## 生成业务模块

```bash
goseed gen module order
goseed gen module order --dry-run
```

在当前已生成项目中登记模块，并生成与当前协议、数据库匹配的代码骨架。数据库存在时生成仓储接口和 Store 骨架；无数据库时只生成协议与业务骨架，不提供虚假内存持久化。

元数据缺失、版本不支持、能力配置与项目不一致、模块重复或目标文件冲突时失败。不接受用命令参数临时覆盖项目已有数据库和协议。

复杂业务由开发者补充，不自动生成可执行的支付、库存或订单状态规则。

## 生成 CRUD 资源

```bash
goseed gen resource product --spec specs/product.yaml
goseed gen resource product --spec specs/product.yaml --dry-run
goseed gen resource product --spec specs/product.yaml --migration
```

首版字段声明采用 YAML，最小形态如下：

```yaml
version: 1
fields:
  - name: title
    type: string
    required: true
    max_length: 120
  - name: enabled
    type: bool
```

资源名称来自位置参数，避免在文件中维护第二个名称来源。第一版字段类型计划支持 `string`、`bool`、`int64`；统一生成 ID、创建时间和更新时间，声明不得覆盖这些保留字段。不支持的类型、属性、重复字段及非法约束直接报错；完整字段格式在对应开发检查点先补充本文件再实现。

生成模型、协议输入输出、基础校验、转换、CRUD、注册、文档及必要测试。无数据库项目不能生成持久化 CRUD，报错并指向 `gen module`。

`--migration` 只生成匹配当前数据库方言的迁移草稿，默认不生成迁移文件；命令不连接数据库、不应用 schema。若没有明确可用的表结构迁移，生成后的 CRUD 尚不能被声明为可运行，输出中必须说明下一步。

选择 gRPC/Gateway 时，需要匹配版本的协议生成工具；在写入前检查工具可用性。已有协议注册和接口文档通过明确标记的生成文件汇总，不任意改写手工 Proto 或业务代码。

## 生成器保护规则

- 写入位置限定在当前项目根目录；目标路径不能包含路径越界或穿过指向外部的符号链接。
- dry-run 不创建业务文件、不修改元数据、不运行数据库或服务。
- 先验证输入、版本、文件归属和全部冲突，再进行生成。工具或渲染错误发生在准备阶段，不留下部分业务文件。
- 新业务文件不覆盖已存在内容；需要更新的生成登记文件若被手工修改，先报冲突，不静默重写。
- 文件系统写入失败时尝试撤回本次写入，并报告仍残留的文件；不声称跨多个文件的操作天然原子。
- 生成器不自动更新模板版本、不执行 Git 操作、不安装全局依赖、不启动容器或服务。

## 服务开发与运行入口

计划提供 `make build`、`make test`、`make lint`、`make check`；各命令只执行其明确职责。生成业务项目的 `make dev` 将组织开发初始化、显式迁移和服务启动，具体行为在实现前写入生成项目文档。

开发初始化可创建不入 Git 的本地配置、随机开发密钥和测试 Token；不得覆盖已有密钥或在生产环境开放开发签发入口。生成命令本身不创建真实环境密钥。

以上入口当前均未实现；可执行状态和已验证用法只在实现后写入仓库或生成项目 README。
