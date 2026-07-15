# 部署说明

本文件描述 Linux AMD64 部署方式，并记录不含凭据或玩家隐私的部署基线。

## 产物

目标基线为 Ubuntu 24.04 x86_64，不使用 Docker。构建机需要 Go 与 Node.js；最终服务器只需要单个二进制和 YAML 配置。

```bash
make build-linux
```

推荐路径：

- 程序：`/usr/local/bin/palworld-companion`
- 配置：`/etc/palworld-companion/config.yaml`
- 数据：`/var/lib/palworld-companion`
- unit：`/etc/systemd/system/palworld-companion.service`

## 服务与安全

使用独立低权限账户 `palworld-companion`。该账户不应拥有 Palworld、PST 或存档目录写权限。示例 unit 只给 `/var/lib/palworld-companion` 写权限。

从 `deploy/config.example.yaml` 复制配置，使用文件权限保护真实用户名和密码。默认监听 `127.0.0.1:8091`。

v0.2.0 开发版本需要 SQLite 写入权限。生产环境建议设置 `database.path: /var/lib/palworld-companion/companion.db`；旧配置缺少该字段时会自动使用此路径。运行用户必须能写入 `/var/lib/palworld-companion`，unit 使用 `UMask=0027` 保护数据库及 WAL 文件。数据库初始化或迁移失败时应用会停止启动，不会删除已有数据或退回内存存储。

v0.1.0 没有 Companion 自身账户系统。公网使用前必须由 HTTPS 反向代理增加访问认证和速率限制，并保持 Companion 与 Palworld REST API 端口不直接暴露。部署、安装 unit 与重启服务应作为独立的明确授权任务执行。

## 2026-07-15 部署记录

- 目标：`192.168.3.113`，Ubuntu 24.04.4 LTS x86_64。
- 版本：`v0.1.0`，源码提交 `9dd9777`。
- Linux AMD64 产物 SHA-256：`8d9c66e89e9a99531255de0fa20aa1f29073df13c4d17d9e393d9d7f28cad306`。
- 服务：`palworld-companion.service` 已启用并运行，当前监听 `0.0.0.0:8091`，供局域网访问。
- 上游：通过回环地址 `127.0.0.1:8212` 读取真实 `/v1/api/info`、`/metrics`、`/players`，`mock_mode=false`。
- 实际字段兼容：`serverfps` 为整数；`serverfpsaverage`、`serverframetime`、玩家延迟与二维坐标为浮点数；人数、运行时间、世界天数、基地数量与玩家等级为整数。当前公开模型所需字段均可正常解析。
- 隐私检查：公开玩家对象仅含 `name`、`level`、`ping`、`position`；原始 `iP`、`playerId`、`userId`、`accountName` 未向 Companion API 返回。
- 故障检查：使用临时回环测试实例和未监听的上游端口验证；上游失败时 health 与页面仍可用，数据接口返回明确错误，测试实例与配置已清理。
- 影响边界：部署前后 Palworld 与 PST 的主进程、启动时间和重启计数保持不变；未修改或重启其服务、配置或存档。
- 局域网检查：`http://192.168.3.113:8091/`、health、version、summary、players 与 PWA manifest 均可访问；360、390、430 像素宽度下无页面级横向溢出。
- PWA 安装：manifest、图标和 Service Worker 资源存在，但普通局域网 HTTP 地址不属于浏览器安全上下文，Service Worker API 不可用，因此当前不能安装 PWA。后续需要 HTTPS；本次未修改防火墙或代理。
- 配置变更备份：`/root/palworld-companion-backup-20260715-033359/config.yaml`。

## 2026-07-15 v0.2.0-dev 更新记录

- 部署源码：`479b5ad`，版本 `0.2.0-dev`，Linux AMD64 产物大小 `16411480` 字节，SHA-256 为 `1ee3eb90c1625944bb5b040e02f1978030562c98fd2288ad28d0d7c68c5359f6`。
- 部署路径保持不变：程序 `/usr/local/bin/palworld-companion`、配置 `/etc/palworld-companion/config.yaml`、数据目录 `/var/lib/palworld-companion`、unit `/etc/systemd/system/palworld-companion.service`。
- 更新前备份位于 `/root/palworld-companion-backup-20260715-072446`，包含旧二进制、配置和 unit；更新前不存在 `companion.db`，因此没有数据库备份。
- 现有配置未修改，继续使用 `0.0.0.0:8091`、真实 Palworld 上游和 `mock_mode=false`；unit 只新增 `UMask=0027`，原有 `ReadWritePaths=/var/lib/palworld-companion` 保持不变。
- SQLite 已在 `/var/lib/palworld-companion/companion.db` 初始化，目录权限为 `0750`，数据库权限为 `0640`，所有者为 `palworld-companion:palworld-companion`。迁移版本为 `1`，包含 `schema_migrations` 与 `tasks` 表，journal mode 为 WAL。
- 服务保持 enabled 和 active，监听 `0.0.0.0:8091`。health、version、capabilities、summary、players 与 tasks 接口均返回 HTTP 200，`tasks=true`；玩家公开字段仍仅包含 `name`、`level`、`ping`、`position`。
- 使用部署测试任务验证了 SQLite 跨 Companion 服务重启持久化；重启后任务仍存在，随后删除返回 HTTP 204，任务表恢复为空。
- 公网链路保持现状：`http://pal.gravioncloud.com/` 返回 301 并跳转到 HTTPS，HTTPS 首页及 health、version、capabilities、tasks 均正常；未修改公网 Nginx 或 FRP。
- 390 像素手机视口人工检查通过：首页、服务器状态、在线玩家、底部任务入口和任务页正常；新增、编辑、排序、完成、恢复、筛选、刷新持久化、首页任务摘要及删除确认均已验证，无页面级横向溢出或控制台错误。HTTPS 页面包含 manifest，manifest、图标和 Service Worker 资源均可访问，具备 PWA 安装前置条件。
- 本次仅重启 `palworld-companion.service`。Palworld 与 PST 的服务状态和 active 时间戳前后保持不变；未修改其服务、配置、端口或存档，未执行 `clean-seeds`，未发生回滚。

服务器现有 Palworld REST API 监听为 `0.0.0.0:8212`。这是 Companion 部署前已存在的状态，本次任务未修改 Palworld 配置、启动参数或防火墙；Companion 上游地址仍固定为 `127.0.0.1:8212`。
