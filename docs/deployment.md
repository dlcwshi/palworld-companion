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

## v0.3.0-dev 本地认证部署要求

0.3.0-dev 不依赖 Steam OpenID、Steam Web API 或外部认证代理。旧配置可以原样保留：`auth.enabled`、`public_base_url` 和 `admin_steam_ids` 会被读取但不参与认证；`auth.session_ttl` 继续生效。

推荐保留：

```yaml
auth:
  session_ttl: 720h
  enabled: true                 # deprecated, unused
  public_base_url: https://pal.gravioncloud.com  # deprecated, unused
  admin_steam_ids: []           # deprecated, unused
```

升级前只停止 `palworld-companion.service`，在同一个带时间戳目录备份：

- `/usr/local/bin/palworld-companion`
- `/etc/palworld-companion/config.yaml`
- `/etc/systemd/system/palworld-companion.service`
- `companion.db`、`companion.db-wal`、`companion.db-shm`（存在时）

schema 4 保留 schema 3 的用户 ID、Session 与任务外键，新增本地密码、审批字段和持久化初始化状态。旧库存在管理员时初始化保持关闭；不存在管理员时 `setupRequired=true`。旧 Steam 用户没有 `password_hash`，不得无密码登录，需要管理员或 CLI 重置密码。

部署后验证：

```bash
/usr/local/bin/palworld-companion setup status \
  --config /etc/palworld-companion/config.yaml
```

当前无用户的生产库升级后应输出 `{"setupRequired":true}`。由项目所有者本人打开 HTTPS `/setup` 创建首任管理员；部署人员不得猜测或代设真实用户名和密码。

恢复 CLI 的密码只从交互式 TTY 无回显读取：

```bash
/usr/local/bin/palworld-companion users create-admin --config /etc/palworld-companion/config.yaml --username <username>
/usr/local/bin/palworld-companion users approve --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
/usr/local/bin/palworld-companion users reject --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
/usr/local/bin/palworld-companion users reset-password --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
/usr/local/bin/palworld-companion users reset-password --config /etc/palworld-companion/config.yaml --username <username>
```

回滚时必须同时恢复旧二进制、旧配置和升级前数据库/WAL/SHM；schema 3 程序会拒绝 schema 4 数据库。迁移或启动失败时不要修改原数据库，不要让旧二进制直接打开已迁移数据库。

PWA Service Worker 的 navigation fallback 排除整个 `/api/`，不会缓存 setup、登录、注册、账号、任务或管理员响应。
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

## 2026-07-15 Steam 认证与任务归属部署记录

- 部署源码提交为 `13f004e`，版本为 `0.2.0-dev`；Linux AMD64 产物大小为 `16542935` 字节，SHA-256 为 `61771fee2cab4e15a913155bb18c98598e5c3651abfe9ab6ee08cdfd870a9a22`。
- 更新前备份位于 `/root/palworld-companion-backup-20260715-093903`，包含旧二进制、配置、unit 和 `companion.db`；停止服务后 WAL/SHM 文件不存在，因此备份中没有这两个文件。
- 配置仅新增 `auth.enabled=true`、公网基址 `https://pal.gravioncloud.com`、`session_ttl=720h` 和空的 `admin_steam_ids`；管理员 SteamID 尚未确认，未创建伪造用户或管理员。
- SQLite schema 已从 1 自动迁移到 3，原有任务迁移为无归属共享任务；验收时 users、sessions、tasks 均为 0，OpenID 验证请求产生 2 条限时 auth flow，journal mode 保持 WAL。
- Companion 服务保持 enabled 和 active，监听 `0.0.0.0:8091`。health、version、capabilities、summary、players、Steam 登录跳转、非法 returnTo 拒绝、未认证任务拒绝及静态 PWA 资源均已验证。
- 公开玩家对象仍仅含 `name`、`level`、`ping`、`position`；服务日志未发现 session cookie、OpenID 签名或认证 Header 泄漏。
- 公网 HTTP 首页返回 301，HTTPS 首页、health、capabilities、manifest、图标和 Service Worker 均返回 200；未修改 Nginx、FRP、防火墙或 8212。
- 本次只停止并启动 `palworld-companion.service`。部署前后 `palworld-server.service` 保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`，`palworld-pst.service` 保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`；未修改配置、数据库或存档，未执行 `clean-seeds`，未发生回滚。
- 真人首次 Steam 登录尚未执行。项目所有者需在 Palworld 在线时通过公网登录，确认账号绑定后再使用 `users set-role` 将该真实 SteamID 提升为管理员。
