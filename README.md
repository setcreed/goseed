# goseed

面向中小型 Go 后端业务的项目脚手架与业务代码生成器项目。

## 当前状态

Go 工具链固定为 **1.26.4**，主模块为 **`github.com/setcreed/goseed`**。仓库默认分支为 `main`。

当前 CLI 支持 `--help` 和 `--version`。无参数执行、未知参数或命令会返回错误，并且不会创建项目文件。开发检查点以 [ROADMAP.md](ROADMAP.md) 为准；完整产品命令契约见设计文档。

内部模板渲染入口为 `internal/templates.Default`，返回校验后的文件计划，展开项目名称及模块路径，并格式化 Go 源码。渲染不直接写入文件。

默认 HTTP 模板包含 Handler → Biz → Store 分层、SQLite、JWT、示例 CRUD、配置与日志、健康检查和优雅退出。提供开发密钥初始化、开发 Token 签发、显式迁移、严格输入校验、资源主体隔离、TLS 与 CORS；没有公开的临时签发接口。服务命令与验收边界见 [默认 HTTP 模板](docs/template-http.md) 。

渲染项目包含完整 `go.mod` / `go.sum`、AGENTS/README/ROADMAP、Makefile、配置示例、SQL 迁移、Swagger 2.0 和非 root Dockerfile，可以独立构建及运行。

`make test` 实际渲染独立项目，执行 readonly 测试、vet、race、lint 与构建，并通过实际服务进程验证初始化、迁移、JWT/CRUD、资源隔离、非法输入、TLS/CORS、启动失败、日志脱敏和重启持久化。需先 `make tools`，同时需要本机回环监听。

## 构建与检查

需要 Go；首次构建会下载 Go 1.26.4 工具链及固定依赖。以下命令在仓库根目录执行：

```sh
make build
./bin/goseed --help
./bin/goseed --version

make tools
make check
make race

# 显式启动本次专用容器验证，需要 Docker
make template-container
```

`make tools` 显式安装固定版本 linter 到 `.tools/bin/`。构建与检查使用项目 `.tmp/` 缓存，不安装全局工具，不自动生成源码或修改依赖清单。`template-container` 实际构建独立业务镜像，使用随机回环端口和全新数据卷验证生产禁用开发签发、显式迁移、CRUD 与重启持久化；结束清理本次容器、卷及测试镜像，普通检查不启动 Docker。

## 技术兼容性验证

固定版本的业务运行依赖在 `tests/compatibility/` 的独立模块中核验。主模块只依赖 CLI 库。

```sh
make compatibility
make compatibility-cross
make compatibility-db

make protocol-tools
make protocol-check
```

兼容性测试需要本机回环监听；`compatibility` 在没有外部数据库配置时明确跳过 MySQL/PostgreSQL。`compatibility-cross` 以纯 Go 方式交叉编译 Linux amd64/arm64 和 Windows amd64 测试程序。检查脚本需要 Python 3.9+；`compatibility-db` 还需要 Docker 与 Compose，会启动本次专用数据库、随机回环端口和全新卷，退出时清理本次容器与卷；不使用已有数据库。

协议测试使用已生成源码，无需 protoc。`protocol-tools` 显式安装固定版本工具到 `.tools/`；`protocol-check` 校验版本并重新生成到临时目录，比对提交范围内的验证产物。依赖选型、许可证、维护证据与测试边界见 [依赖核验](docs/dependencies.md) 。

验证程序覆盖以下范围，验证结果统一记录在 ROADMAP：

| 对象 | 环境与检查范围 |
|---|---|
| SQLite | 真实磁盘文件；多连接 PRAGMA、迁移、约束错误、事务回滚、Context、锁等待和关闭连接后读取 |
| MySQL 8.4.11、PostgreSQL 18.6 | 固定镜像摘要的真实数据库服务；显式迁移、Casbin 策略保存与重载、权限判定、删除和保存失败回滚 |
| Casbin 适配原型 | 三种数据库共用现有 GORM 连接；缺表报错且不隐式建表，原型仅用于技术验证 |
| gRPC、Gateway | 本机回环服务的实际调用和转发、Swagger 2.0 产物解析、固定工具重新生成比对 |
| 其他运行依赖 | 配置读取、日志与 DI 组装、JWT 签发与解析、Redis 客户端构造与关闭 |

纯文档改动可使用 `make contract-check` 检查模块、工具版本、本地链接与文件格式。项目开发及功能完成后的 Git 提交流程见 [项目规范](AGENTS.md) 。

## 文档入口

- [项目规范](AGENTS.md)：目录职责、工程边界、工作流与验证要求。
- [产品需求](docs/requirements.md)：使用场景、功能范围和验收标准。
- [技术架构](docs/architecture.md)：分层、目录、技术选型、配置、日志和服务生命周期。
- [命令契约](docs/cli.md)：默认行为、参数、能力组合和业务生成约束。
- [实施方案](docs/implementation.md)：开发顺序、检查点及验证方式。
- [依赖核验](docs/dependencies.md)：固定版本、官方来源、维护情况与兼容性约束。
