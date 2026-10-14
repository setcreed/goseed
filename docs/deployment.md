# 单机部署

本文定义 D11 的 `systemd` + Nginx 单机部署模板。生成物只描述应用服务和反向代理，不安装软件、不修改主机、不创建账号/证书/数据库、不运行迁移，也不启停服务。

## 生成与支持范围

创建项目时显式选择 `--deployment=single-host`；默认 `none`，不改变已有默认输出。该选项生成 `deploy/systemd/<name>.service`、按协议生成 `deploy/nginx/` 配置，以及该项目专属的部署、备份恢复和升级操作说明。模板仅作为可审阅文件副本，不由运行服务读取，也不覆盖业务配置。

单机模板适用于 Linux + systemd，应用单实例运行。HTTP、gRPC、dual 和 Gateway 均支持：HTTP 服务由 Nginx 终止公网 TLS 后代理到应用 loopback；纯 gRPC 使用独立 gRPC 域名和 Nginx TLS 前端，并以验证服务证书的 TLS 连接代理到 loopback gRPC；dual 使用不同的 HTTP 与 gRPC 域名；Gateway 只代理其 HTTP 入口，应用到内部 gRPC 的 TLS 校验仍由应用配置负责。生产应用的监听地址固定为 `127.0.0.1`，数据库/Redis 不暴露为公网入口。未选择数据库时仍可生成服务与代理模板，但没有数据库备份操作。HTTP/2 gRPC 前端要求 Nginx 1.25.1 或更新版本，以使用当前 `http2 on` 配置语法。

SQLite、MySQL、PostgreSQL 的部署路径均支持。SQLite 作为单机本地文件，放在 `/var/lib/<name>/`；MySQL/PostgreSQL 保持外部服务，由应用账号连接，不在 Nginx 或 systemd 模板中创建或托管数据库。生产 SQL 与 Redis 连接必须启用 TLS。应用生产 gRPC 继续要求 TLS；HTTP 可以在 loopback 上使用明文，因为公网 TLS 在 Nginx 终止。

不支持把 SQLite 文件置于 NFS/共享盘供多个实例访问；不生成容器编排、数据库安装、证书签发、云防火墙或高可用配置。Nginx 信任代理边界固定：应用不信任 `X-Forwarded-*` 客户端输入，Gin 不启用 trusted proxies；Nginx 覆写转发头只用于标准代理信息，不使应用把它当认证、授权或可信客户端地址。若未来需要可信客户端 IP，必须单独设计可信代理 CIDR 和失败行为。

## 主机路径与权限

- 应用发布目录：`/opt/<name>/releases/<version>/`，当前版本通过 `/opt/<name>/current` 符号链接选择；发布目录由 root 拥有，服务账号只读。
- 私有运行配置：`/etc/<name>/app.yaml`；密钥环境文件：`/etc/<name>/app.env`，权限 `root:<name>`、`0640`；文件不生成真实凭据。
- 服务账号：固定专用 `<name>` 用户/组，不共享登录账号，不授予 sudo。
- SQLite 状态目录：`/var/lib/<name>/`，仅服务账号可写；MySQL/PostgreSQL 模式不授予应用写系统目录权限。
- 备份目录：`/var/backups/<name>/`，仅受控备份操作者可写；备份按加密存储/离机复制策略管理，示例工具不替代异地保管。
- 公网仅暴露 Nginx 的 80/443；应用端口保持 loopback。systemd 使用 `ProtectSystem=strict`、`NoNewPrivileges=yes`、专用 `StateDirectory` 和最小可写目录；日志交由 journald，密钥不写进 unit、命令行、Nginx 配置或日志。

部署前必须审阅 Nginx 域名与证书路径，提供匹配域名的有效证书；gRPC 代理需配置可验证的上游 CA/证书名称。必须先在新 release 目录完成 readonly 构建和测试，再原子切换 `current`。配置与证书先写入对应私有目录并核验权限。执行生成的 SQL 迁移命令是单独的人工步骤，生成/安装/重启不会自动迁移。

## 备份与恢复

SQLite 使用 `sqlite3` 在线 `.backup` 生成一致快照，可在应用运行、WAL 开启时执行；不能以复制单个主数据库文件代替在线备份。备份后用 `PRAGMA integrity_check` 验证，并在新的空路径恢复和执行应用级只读核验。恢复不得覆盖来源备份或当前生产数据库文件。

MySQL 使用固定工具版本的 `mysqldump --single-transaction`（仅对事务表提供一致快照；备份期间不得并行执行 ALTER、DROP、RENAME、TRUNCATE 等 DDL），PostgreSQL 使用 `pg_dump --format=custom`；认证从权限为 `0600` 的客户端凭据文件读取，不把密码放入 argv。恢复总是创建随机隔离目标数据库，由独立测试账号创建，恢复后核对 schema 版本和代表性数据。文档中的备份命令明确区分一致性范围、DDL/非事务表限制、凭据文件和清理对象；不使用生产账号创建或删除数据库。

备份成功不代表可恢复。每次数据库或迁移方案变化后，应在隔离目标执行恢复演练；在验收数据完整、服务能读取之前，不切换生产数据路径。

## 升级与回退

新版本先部署到新的 release 目录，核对校验、配置兼容、运行用户权限和健康检查。SQLite 执行在线备份；外部 SQL 按组织的备份策略完成备份。审阅迁移 SQL 和版本范围，显式应用迁移后再切换符号链接并重启 systemd 服务。确认 readiness、HTTP/gRPC 健康和关键业务检查后，才保留新版本为当前版本。

应用启动失败且 schema 未变化或保持向后兼容时，可将 `current` 原子切回旧 release 并重启。二进制回退不回滚 schema；迁移已改变旧程序不兼容的 schema 时停止自动回退，按已批准的数据恢复/前向修复方案处理。恢复备份必须目标为新路径/新数据库，核验后再由操作者显式切换连接；模板不覆盖生产数据、不自动删除旧 release，也不承诺零停机升级。

## 验收边界

D11 验收在专用隔离 Linux 环境中运行 systemd、Nginx 和随机测试数据，固定 Debian 13 基线与其中 systemd/Nginx 包版本，并记录实际镜像 digest。测试覆盖服务启动、SIGTERM 排空与重启、权限/只读边界、有效与错误证书、伪造转发头不被信任、HTTP/gRPC 路由、SQLite WAL 备份恢复、MySQL/PostgreSQL 隔离恢复以及新旧 release 回退。语法检查不代替真实 systemd/Nginx 运行；测试环境不可用时，相关运行验收必须明确保留未通过。

实现依据：[Nginx gRPC 模块](https://nginx.org/en/docs/http/ngx_http_grpc_module.html)、[Nginx HTTP/2 模块](https://nginx.org/en/docs/http/ngx_http_v2_module.html)、[Debian systemd.exec 手册](https://manpages.debian.org/trixie/systemd/systemd.exec.5.en.html)、[SQLite Online Backup API](https://www.sqlite.org/backup.html)、[MySQL 8.4 备份方法](https://dev.mysql.com/doc/refman/8.4/en/backup-methods.html)、[PostgreSQL 18 pg_dump](https://www.postgresql.org/docs/18/app-pgdump.html) 与 [pg_restore](https://www.postgresql.org/docs/18/app-pgrestore.html)。
