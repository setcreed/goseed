# goseed

面向中小型 Go 后端业务的项目与业务代码生成器。生成的项目包含完整源码，可以独立开发、测试和部署，不依赖 goseed 运行时。

默认生成 **HTTP + SQLite + JWT** 项目，也可选择 MySQL、PostgreSQL、无数据库、Redis、Casbin、gRPC、dual 或 Gateway。

## 功能

- 生成可独立构建的 Go 服务，默认提供主体隔离的示例 CRUD。
- 按选项生成数据库、协议、认证授权、缓存、可观测性和单机部署文件。
- 使用 `gen module` 增加业务模块，使用 `gen resource` 生成 CRUD 资源和协议适配。
- 生成的服务带有配置校验、显式迁移、健康检查和有期限的优雅关闭。

## 快速开始

需要 Go 1.26.4 或更新的兼容版本、Git 和 Make。首次构建会从 Go 模块代理下载依赖。克隆并构建生成器：

```sh
git clone https://github.com/setcreed/goseed.git
cd goseed
make build
```

在一个新的空目录中生成默认项目并启动本地开发服务：

```sh
GOSEED="$(pwd)/bin/goseed"
mkdir -p ../goseed-demo
cd ../goseed-demo
"$GOSEED" --name demo --module example.com/demo

make build
./bin/apiserver dev init
./bin/apiserver migrate up
./bin/apiserver serve
```

`serve` 启动后默认监听 `127.0.0.1:8080`。另开终端，在生成项目目录中创建开发 Token 并调用示例接口：

```sh
cd /path/to/goseed-demo
TOKEN="$(./bin/apiserver dev token --subject alice --ttl 15m)"
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"first","description":"hello"}' \
  http://127.0.0.1:8080/v1/examples
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8080/v1/examples
```

`dev init` 只用于本地开发，会创建私有配置和随机 JWT 密钥。生产环境必须由部署方安全提供密钥；服务启动不会自动执行数据库迁移。生成项目的 README 包含该项目对应的运行、配置和部署说明。

## 选择能力

生成器默认使用 SQLite 和 HTTP。下面的示例展示其他组合；需要外部数据库或 Redis 时，先准备对应服务，再按生成项目 README 配置连接信息。

```sh
goseed --name shop --module github.com/acme/shop \
  --db=postgres --redis --authz=casbin --transport=dual
```

`--dry-run` 可先查看计划和文件冲突。项目生成、模块和资源命令的参数与组合规则见 [CLI 参考](docs/cli.md)；生成项目的协议、数据库、安全和缓存行为见 [模板指南](docs/templates.md)。

在已生成的项目中增加业务代码：

```sh
goseed gen module orders --dry-run
goseed gen module orders
goseed gen resource product --spec specs/product.yaml --dry-run
goseed gen resource product --spec specs/product.yaml --migration
```

资源 spec 示例和字段约束见 CLI 参考。`--migration` 只生成待审阅的 SQL 草稿，不连接或修改数据库。gRPC、dual 和 Gateway 项目在生成资源前，先于业务项目执行 `make proto-tools`。

## 部署

默认不生成主机部署文件。使用 `--deployment=single-host` 可加入 systemd/Nginx 配置副本和所选数据库的备份工具；这些文件不会安装服务、修改主机配置或执行迁移。操作步骤、权限、备份恢复和回退边界见[单机部署指南](docs/deployment.md)。

## 文档

使用者从快速开始出发，按需要查阅以下参考：

- [CLI 参考](docs/cli.md)：项目、模块、资源命令和参数。
- [模板指南](docs/templates.md)：生成服务的配置、协议、安全与运行行为。
- [单机部署指南](docs/deployment.md)：systemd/Nginx、权限、备份恢复和升级回退。

维护者资料、开发记录和后续规划单独列出，不影响使用者的上手路径：

- [开发指南](docs/development.md) · [架构说明](docs/architecture.md) · [资源生成设计](docs/resource-generation.md)
- [依赖与许可证记录](docs/dependencies.md) · [施工计划](docs/implementation.md) · [项目进度与验证记录](ROADMAP.md)
- [协作规范](AGENTS.md)

## 许可证

[MIT](LICENSE)。第三方依赖和生成代码继续保留各自适用的版权与许可证声明。
