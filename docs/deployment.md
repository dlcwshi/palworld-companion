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

## v0.3.1-dev 角色名注册部署要求

0.3.1-dev 不依赖 Steam OpenID、Steam Web API 或外部认证代理。玩家注册实时按唯一在线角色名绑定，批准后可使用角色名或 SteamID64 登录。旧配置可以原样保留：`auth.enabled`、`public_base_url` 和 `admin_steam_ids` 会被读取但不参与认证；`auth.session_ttl` 继续生效。

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

v0.3.1-dev 继续使用 SQLite schema 4，不新增表或迁移。schema 4 保留用户 ID、Session 与任务外键；升级不得重置现有用户、Session、任务或 `setup_completed`。旧 Steam 用户没有 `password_hash` 时仍不得无密码登录，需要管理员或 CLI 重置密码。

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

## 2026-07-15 v0.3.0-dev 本地认证部署记录

- 部署源码提交为 `516a48b`，版本为 `0.3.0-dev`，构建时间为 `2026-07-15T11:43:02Z`。Linux AMD64 产物大小为 `16821646` 字节，SHA-256 为 `14d442114fdbbe83412e96a2bae627b9772282f2ac74d6b31df7ab7ef6a3243b`。
- 上传产物保留在 `/tmp/palworld-companion-516a48b`；部署程序为 `/usr/local/bin/palworld-companion`。更新前备份位于 `/root/palworld-companion-backup-20260715-114451`，包含旧二进制、配置、unit 和停止服务后的 `companion.db`；当时 WAL/SHM 不存在，因此备份中没有这两个文件。
- 现有 `/etc/palworld-companion/config.yaml` 未修改，部署前后 SHA-256 一致。旧 Steam 认证字段仍可解析但不参与认证；未修改 Nginx、FRP、防火墙、8212 或 Palworld 上游配置。
- SQLite schema 从 3 自动迁移到 4，验收时 `users=0`、`sessions=0`、`tasks=0`，历史 `auth_flows=5` 保留，`system_settings.setup_completed=false`，`setupRequired=true`，journal mode 为 WAL，`PRAGMA foreign_key_check` 无违规。数据库权限为 `0640`，所有者为 `palworld-companion:palworld-companion`。
- Companion 服务保持 enabled、active 和 running，PID 为 `119593`，监听 `0.0.0.0:8091`。health、system version、capabilities、setup status、未认证任务拒绝和 CLI setup status 均已验证；本地认证、初始管理员、玩家注册、用户审批、任务归属能力均启用，Steam 认证能力关闭。
- 公网 HTTP 首页返回 301 并跳转 HTTPS；HTTPS 首页、`/setup`、health、system version、capabilities、setup status、manifest、图标、Service Worker 和玩家接口均返回 200。两个旧 Steam 路由返回 HTTP 410，未认证任务接口返回 HTTP 401。
- 公网玩家对象仅包含 `name`、`level`、`ping`、`position`。部署后 Companion 日志未命中密码、Cookie、Authorization、Session token、OpenID 签名或 Steam 域名，也未发现 panic、fatal、迁移错误或外键错误；进程验收时无已建立的外部连接。
- 本次仅停止并启动 `palworld-companion.service`。`palworld-server.service` 保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`；`palworld-pst.service` 保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`。未修改或重启 PalServer/PST，未触碰其配置、数据库或存档，未执行 `clean-seeds`，未使用 Docker，未发生回滚。
- 自动化和浏览器测试覆盖了空库 Setup、本地登录、注册审批状态、密码与 Session、安全边界、移动端 390 像素页面和 PWA 前置链路。仍需项目所有者本人完成首任管理员初始化、使用真实在线玩家注册并在管理员页面审批，以及在实体手机上执行“安装到主屏幕”的最终点击；部署人员未创建或猜测任何真实账号、密码或身份映射。

## 2026-07-15 v0.3.1-dev 角色名注册部署记录

- 部署源码提交为 `b56e1f5`，版本为 `0.3.1-dev`，构建时间为 `2026-07-15T14:37:57Z`。Linux AMD64 产物大小为 `11247800` 字节，SHA-256 为 `7915984cb17cf09d0a02e33520dce8f0537de0928f1b50c021c15bb1bce716e6`。
- 已核验产物保留在 `/tmp/palworld-companion-b56e1f5`，部署程序为 `/usr/local/bin/palworld-companion`。更新前备份位于 `/root/palworld-companion-backup-20260715-144157`，包含旧二进制、配置、unit 和停止服务后的 `companion.db`；当时 WAL/SHM 不存在。
- `/etc/palworld-companion/config.yaml` 未修改，部署前后 SHA-256 均为 `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`。未修改 Nginx、FRP、防火墙、8212 或 Palworld 上游配置。
- SQLite 保持 schema 4 和 WAL。备份库与部署后生产库均为 `users=2`、`sessions=3`、`tasks=2`、`setup_completed=true`，`PRAGMA foreign_key_check` 无违规；已有账号、会话、任务和 Setup 状态未丢失或重置。
- Companion 服务保持 enabled、active 和 running，部署后 PID 为 `122533`，active 时间为 `2026-07-15 14:41:57 UTC`，监听仍为 `0.0.0.0:8091`。health 与 system version 均返回 `0.3.1-dev`。
- 公网 HTTP 首页返回 301 并跳转 HTTPS；HTTPS 首页、`/login`、`/register`、health、manifest、图标和 Service Worker 均返回 200。旧 Steam 路由继续返回 HTTP 410，未认证任务接口继续返回 HTTP 401。
- 部署静态资源为 `/assets/index-CNh61_YE.js` 与 `/assets/index-BChX2bQ3.css`，包含角色名注册字段、新错误码、48px 注册入口、16px 字号和 `focus-visible` 规则。相同源码的 390×844 浏览器验收无横向溢出或控制台错误。
- 部署后 Companion 日志未命中密码、Cookie、Authorization、Session token、OpenID/Steam 域名或 panic、fatal、迁移、外键错误；验收结束时无 Companion 已建立连接。
- 本次仅停止并启动 `palworld-companion.service`。`palworld-server.service` 保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`；`palworld-pst.service` 保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`。未修改或重启 PalServer/PST，未触碰配置、数据库或存档，未执行 `clean-seeds`，未使用 Docker，未发生回滚。
- 一次失败的 tar 上传在 `/tmp/palworld-companion-linux-amd64` 留下 `11236352` 字节无效残片；该文件未用于部署，也未在缺少删除授权时清理。仍需使用真实在线角色完成角色名注册与管理员审批，并在实体手机完成“安装到主屏幕”最终点击。

## 2026-07-16 v0.4.0-dev 玩家持久化名册部署记录

- 部署源码提交为 `f1190bb9d51b38e75205258a43ab5b95cc3fc608`，版本为 `0.4.0-dev`，构建时间为 `2026-07-16T02:35:27Z`。Linux AMD64 制品大小为 `11284664` 字节，SHA-256 为 `4dc3eab8add5a94b668c02e31881775d054401c0b0595d9012623c2d4249ac5d`；已核验上传制品保留在 `/tmp/palworld-companion-f1190bb`。
- 更新前备份位于 `/root/palworld-companion-backup-20260716-023841`，包含旧二进制、配置和停止服务后的 `companion.db`。停服复制时源数据库 WAL/SHM 均不存在；随后对备份库进行只读核验时 SQLite 在备份目录生成了 0 字节 WAL 和 32768 字节 SHM，不代表遗漏源侧车数据。回滚方案为同时恢复旧二进制和完整 schema 4 数据库；本次未触发回滚。
- SQLite 从 schema 4 事务迁移至 schema 5，journal mode 保持 WAL。迁移前后均为 `users=2`、`sessions=5`、`tasks=2`、`auth_flows=5`；迁移后 `player_roster=1`，`PRAGMA foreign_key_check` 为 0。`setup_completed=true` 且公开 setup 状态为 `setupRequired=false`，现有用户、Session、任务和 Setup 状态未丢失或重置。
- 名册接口验收时 `available=true`、`currentStatusKnown=true`、`stale=false`，持久名册共 1 行；响应包含在线状态与 `lastOnlineAt`。再次重启 Companion 后 PID 从 `138933` 变为 `139046`，重启前后名册均为 1 行，随后玩家接口继续返回 HTTP 200，确认名册跨服务重启持久化。
- 公共玩家 JSON 的键级检查未发现 SteamID、Palworld userId/playerId、accountName、IP 或内部 ID；summary 在线人数与名册 `currentOnline` 一致。自动测试覆盖上游失败、非法快照、事务回滚和 stale/SQLite 回退，确认这些路径不会把玩家错误标记离线，也不会修改最后在线或最后成功同步时间。
- `/etc/palworld-companion/config.yaml` 未修改，部署前后 SHA-256 均为 `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`。确认旧的 `/tmp/palworld-companion-linux-amd64` 不是当前运行文件且未被使用后，已仅删除该无效残片。
- Companion 服务保持 enabled、active 和 running，最终 PID 为 `139046`，active 时间为 `2026-07-16 02:40:05 UTC`，程序权限为 `root:root 0755`。health 返回 `0.4.0-dev`，system version 准确返回部署源码提交及构建时间。
- 公网 HTTP 首页返回 301；HTTPS 首页、`/login`、`/register`、health、玩家接口、manifest、图标和 Service Worker 均返回 200。旧 Steam 路由继续返回 HTTP 410，未认证任务接口继续返回 HTTP 401。相同前端源码的 390×844 浏览器验收无横向溢出，筛选可点击，键盘焦点清晰，控制台无错误。
- 部署后 Companion 日志未发现密码、Cookie、Authorization、Session token、OpenID 签名、Steam 域名或 panic、fatal、迁移、外键错误。生产环境未创建测试账号或填写测试密码。
- 本次仅停止、启动和再次重启 `palworld-companion.service`。`palworld-server.service` 始终保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`；`palworld-pst.service` 始终保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`。未修改或重启 PalServer/PST，未修改 Nginx、FRP、防火墙、8212、配置或存档，未执行 `clean-seeds`，未使用 Docker。
- 仍需真人验收：两名真实玩家同时在线与一人退出后的状态切换、最后在线时间、角色改名不产生重复身份、角色名注册及管理员审批、离线角色名登录，以及实体手机安装到主屏幕和筛选/状态徽标体验。

## 2026-07-16 v0.4.1-dev 首页与 PWA 更新部署记录

- 部署源码提交为 `9460128929a33ae7dfac5f572d05318ae8b5342e`，部署记录为本提交，版本为 `0.4.1-dev`，构建时间为 `2026-07-16T05:02:15Z`。Linux AMD64 制品大小为 `11284664` 字节，SHA-256 为 `5aa7da9138212daef2687d34a54017a3f9828331cc5d3bd256f160cef95b0bd3`；已核验上传制品保留在 `/tmp/palworld-companion-9460128`。
- 更新前备份位于 `/root/palworld-companion-backup-20260716-050408`，包含旧二进制、配置、unit 和停止服务后的 `companion.db`。停服后 WAL/SHM 已干净关闭且不存在，因此未复制侧车文件；未发生回滚。
- SQLite 保持 schema 5 和 WAL。部署前后均为 `users=2`、`sessions=5`、`tasks=2`、`auth_flows=5`、`player_roster=1`，`setup_completed=true`、`setupRequired=false`，`PRAGMA foreign_key_check=0`；现有用户、Session、任务、名册和 Setup 状态未丢失或重置。
- `/etc/palworld-companion/config.yaml` 未修改，部署前后 SHA-256 均为 `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`。Companion 服务保持 enabled、active 和 running，部署后 PID 为 `141488`，active 时间为 `2026-07-16 05:04:08 UTC`；health 和 system version 返回 `0.4.1-dev`、源码提交 `9460128` 及正确构建时间。
- 生产首页引用 `/assets/index-B7F_VMpp.js` 和 `/assets/index-B3Wux41q.css`，SHA-256 分别为 `fa1dba5e02ff05633d08faa8000c25619bf30fb9778d8334594b5860ed0fbe4c` 与 `c0947e0fa436ec64efbbca1b27ca18c50d3c984a23d1a7d1064c7d82057b6602`；Service Worker SHA-256 为 `20ef665559341f9c965c899839b29dbd01ea1881a0161a1fd59a7bd394041d06`，manifest SHA-256 为 `feddf9b3363cdfee99f27656c74e14d2ff9037c4fa57dfee13d31995f56f72e1`。Worker 预缓存本次 JS/CSS，包含 `skipWaiting`、`clientsClaim`，且没有 API 预缓存或 runtime cache。
- `index.html`、manifest 和 `sw.js` 由 Companion 返回 `Cache-Control: no-cache`，内容哈希 assets 返回 `public, max-age=31536000, immutable`。部署前直出资源已是 0.4.0，但同一公网浏览器仍被旧 Worker 控制并渲染 `V0.2 DEV`、`TONIGHT` 和“在线玩家”，确认旧 UI 主因是既有 PWA 客户端未完成接管，不是生产二进制嵌入了错误 dist。
- 部署后保留的旧公网浏览器会话执行一次普通刷新，约 2 秒后由新 Worker 自动接管并显示 `V0.4.1 DEV`、`TASKS`、任务互斥分组、`PLAYERS` 和完整名册；再次刷新仍保持新版本，无无限刷新和控制台错误。独立本地已登录会话刷新后仍保留登录状态及任务数据，更新流程未清除 Session Cookie。
- 390×844 本地浏览器验收无横向溢出，任务徽标去重正确，个人与共享任务各只渲染一次，键盘焦点可见。默认玩家筛选为“全部”，自动 Mock 快照出现离线玩家时仍保留该玩家且不显示旧 ping/坐标；在线和离线筛选均正确。生产名册实际只有 1 人且验收时在线，因此未伪造生产离线玩家，离线与未知状态依靠自动测试和本地浏览器验证。
- 公网 HTTP 首页继续返回 301 到 HTTPS；HTTPS 首页、静态资源、manifest、Service Worker 和 API 正常。旧 Steam 路由返回 HTTP 410，未认证任务接口返回 HTTP 401；公共玩家字段检查无 SteamID、Palworld userId/playerId、accountName、IP 或内部 ID，summary 在线人数与名册 `currentOnline` 一致。部署后日志未发现密码、Cookie、Authorization、Session token、OpenID/Steam 域名或 panic、fatal、迁移、外键错误。
- 本次仅停止并启动 `palworld-companion.service`。`palworld-server.service` 保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`；`palworld-pst.service` 保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`。未修改或重启 PalServer/PST，未修改 Nginx、FRP、防火墙、8212、配置或存档，未执行 `clean-seeds`，未使用 Docker。
- 仍需真人验收：实体手机已安装 PWA 从旧版本重新打开后的最终体验；个人任务与共享任务同时存在时的实体手机视觉；两名真实玩家一在线一离线时的名册；真实上游状态未知提示；底部导航和长角色名在实体手机上的显示。

## 2026-07-16 v0.4.2-dev 移动首页与登录回跳部署记录

- 部署源码提交为 `bcec2440777f1f976f869eef8fc3e2e6edb90534`，版本为 `0.4.2-dev`，构建时间为 `2026-07-16T06:34:25Z`。Linux AMD64 纯 Go 制品使用 `CGO_ENABLED=0` 构建，大小为 `11288760` 字节，SHA-256 为 `cc86db1c5890e2b8d5f7c29c52ece6ffc0c91dced4ae91db84b5d42bcf5da85c`；已核验上传制品保留在 `/tmp/palworld-companion-bcec244`。
- 更新前备份位于 `/root/palworld-companion-backup-20260716-063715`，包含旧二进制、配置、unit 和停止服务后的 `companion.db`。停服后 WAL/SHM 均不存在；未发生回滚。`/etc/palworld-companion/config.yaml` 部署前后 SHA-256 均为 `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`。
- SQLite 保持 schema 5 和 WAL。部署前后均为 `users=2`、`sessions=6`、`tasks=3`、`auth_flows=5`、`player_roster=1`，`setup_completed=true`、`setupRequired=false`，`PRAGMA foreign_key_check=0`；现有用户、Session、任务、名册和 Setup 状态未丢失或重置。
- Companion 服务保持 enabled、active 和 running，部署后 PID 为 `146426`，active 时间为 `2026-07-16 06:37:16 UTC`。health 和 system version 返回 `0.4.2-dev`、完整源码提交及正确构建时间。首次启动后的即时探测早于端口监听而未连接，随后带就绪重试的完整验收通过，未触发回滚。
- 生产首页引用 `/assets/index-8b2M7UxY.js` 与 `/assets/index-BtcA7pn1.css`，SHA-256 分别为 `23b26c425eee9a82e6e2fce2db6979c3040ec31dece23a1cdecd7e50ae2c4814` 与 `183b0cc331892322753f99bc7aa006a9cb44ed0bb98fc419cd78b65123ce7e20`；Service Worker SHA-256 为 `b6668edee1706c30dfc066d232dfe73199bd29e1ce540610ff6d4ccf3fd07e09`，manifest SHA-256 为 `feddf9b3363cdfee99f27656c74e14d2ff9037c4fa57dfee13d31995f56f72e1`。Manifest 为 `start_url=/`、`scope=/`、`display=standalone`；Worker 包含 `skipWaiting` 与 `clientsClaim`，不预缓存 `/api/`。
- `index.html`、manifest 和 `sw.js` 继续返回 `Cache-Control: no-cache`，哈希 assets 返回 `public, max-age=31536000, immutable`。保留的公网浏览器会话打开后约 2 秒自动显示 `V0.4.2 DEV`，再次刷新仍稳定显示新首页、任务和玩家，控制台无错误或无限刷新；实体手机已安装 PWA 的最终自动更新仍需真人确认。
- 登录回跳在隔离本地实例完成实际交互验证：直接登录进入 `/`；未登录恢复 `/account` 先进入无 `returnTo` 的 `/login`，登录后进入 `/`；明确访问 `/tasks` 登录后返回 `/tasks`；外部 URL 被忽略并进入 `/`。生产未填写真实密码，确认未认证访问 `/account` 最终进入不带回跳参数的 `/login`。刷新后本地测试 Session 保留。
- 390×844 实测品牌区高 `50px`、标题约 `34px`、刷新按钮 `44×44px`、服务器卡 `218px`、四张指标卡均为 `100px`，任务标题顶边为 `548px`；生产同尺寸服务器卡为 `187px`、任务标题顶边为 `516px`。412×915 与 430×932 的任务标题顶边分别为 `555px`、`559px`。三档均无横向溢出，刷新按钮键盘焦点为 3px 可见轮廓；底部导航高 `68px`，页面保留 `84px` 加 safe-area 的底部空间。
- 账户页自动化浏览器验收确认不再显示“安全边界”、Argon2id、Cookie 属性或 Token 哈希技术卡片，仅保留账号信息、修改密码、退出、管理员入口和弱化版本文字。生产 JS/CSS 与该本地验收构建的哈希完全一致。
- 自动测试新增登录返回目标白名单与公共离线名册回归：A、B 首次成功快照均在线，后续仅 A 在线时公共响应仍返回 A、B，`counts.total=2`、`currentOnline=1`，B 为 offline、ping/position 为 null 且最后在线时间保留。生产名册实际仍只有 1 人，未修改真实数据库制造离线玩家。
- 公网 HTTP 首页继续 301 跳转 HTTPS；HTTPS 首页、登录、注册、health、manifest 和 Service Worker 均返回 200。旧 Steam 路由继续返回 410，未认证任务接口返回 401；公共玩家键仅含 `name`、`level`、`status`、`lastKnownStatus`、`lastOnlineAt`、`ping`、`position`，未发现内部身份字段。部署后日志未命中密码、Cookie、Authorization、Session token、OpenID/Steam 域名或 panic、fatal、迁移、外键错误。
- 本次仅停止并启动 `palworld-companion.service`。`palworld-server.service` 保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`；`palworld-pst.service` 保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`。未修改或重启 PalServer/PST，未修改 Nginx、FRP、防火墙、8212、配置或存档，未执行 `clean-seeds`，未使用 Docker。
- 仍需真人验收：实体手机已安装 PWA 自动更新至 0.4.2-dev、实体 PWA Session 失效后登录直接进入首页、普通真实账号浏览器登录、手机首屏主观密度、长服务器名和长角色名、第二名真实玩家上线后名册变为共 2 人及其退出后继续保留离线，以及 Android 手势条不遮挡底部导航。

## 2026-07-16 v0.4.3-dev 玩家名册故障退避部署记录

- 部署源码提交为 `3608aa1496b66e7e4b421c49baf90bab96a66104`，版本为 `0.4.3-dev`，构建时间为 `2026-07-16T08:21:31Z`。Linux AMD64 纯 Go 制品使用 `CGO_ENABLED=0` 构建，大小为 `11288760` 字节，SHA-256 为 `3b41a986a15ecc3cc6785b31083484175595a7bc4cda6c625fd04a9ab713c806`；已核验上传制品保留在 `/tmp/palworld-companion-3608aa1`。
- 更新前备份位于 `/root/palworld-companion-backup-20260716-082713`，包含旧二进制、配置和停止服务后的 `companion.db`。停服后 WAL/SHM 均不存在，因此未复制侧车文件；未发生回滚。`/etc/palworld-companion/config.yaml` 部署前后 SHA-256 均为 `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`。
- SQLite 保持 schema 5 和 WAL。部署前后均为 `users=2`、`sessions=6`、`tasks=4`、`auth_flows=5`、`player_roster=1`，`setup_completed=true`、`setupRequired=false`，`PRAGMA foreign_key_check=0`；现有用户、Session、任务、名册和 Setup 状态未丢失或重置。
- Companion 服务部署前为 PID `146426`、active 时间 `2026-07-16 06:37:16 UTC`，部署后为 PID `147951`、active 时间 `2026-07-16 08:27:13 UTC`，始终保持 enabled、active 和 running。health 与 system version 均返回 `0.4.3-dev`、完整源码提交和正确构建时间；首次就绪探测早于端口监听，后续重试成功。
- 生产首页引用 `/assets/index-B4_L1ML9.js` 与 `/assets/index-BtcA7pn1.css`，SHA-256 分别为 `ec42b1c48a01454e1696fcb8bf926e1c77c4d20118f018698e1132a67065030f` 与 `183b0cc331892322753f99bc7aa006a9cb44ed0bb98fc419cd78b65123ce7e20`；Service Worker SHA-256 为 `37c000c34a3b6858dbd8ec07aca631e3c6a3f4a1e3bc9b9ab741b248aba05034`。Manifest 保持 `start_url=/`、`scope=/`、`display=standalone`，构建配置继续拒绝 `/api/` navigation fallback 且没有 API runtime cache。
- 公网 HTTP 首页继续 301 跳转 HTTPS；HTTPS 首页、`/login`、`/account`、`/tasks`、health、版本、manifest、JS、CSS 和 Service Worker 均正常。旧 Steam 路由继续返回 410，未认证任务接口继续返回 401；正常玩家接口返回 `available=true`、`currentStatusKnown=true`、`stale=false`，名册共 1 人。
- 回归测试证明失败冷却从上游尝试完成时开始：慢失败后的排队请求在 TTL 内不重复访问 `/players`，恢复刷新仍保持单次上游调用，失败不改变在线状态、最后在线或最后成功同步时间；`FreshPlayers` 始终强制实时请求且成功后清除失败状态。生产未中断 Palworld REST API 制造故障，故障场景仅以确定性自动测试验收。
- `go test -count=1 ./...` 和 `go vet ./...` 通过；当前 Windows Go 环境为 `CGO_ENABLED=0`，`go test -race -count=1 ./internal/roster/...` 明确受限于 `-race requires cgo`，未为此改变纯 Go 边界。前端 `npm ci`、audit（0 vulnerabilities）、type-check、lint、首页任务分组、登录回跳、生产构建和构建校验均通过。
- 部署后日志未发现 panic、fatal、迁移或外键错误。本次仅停止并启动 `palworld-companion.service`；`palworld-server.service` 保持 PID `113468`、active 时间 `2026-07-15 09:35:34 UTC`，`palworld-pst.service` 保持 PID `107527`、active 时间 `2026-07-15 01:48:58 UTC`。未修改或重启 PalServer/PST，未修改 Nginx、FRP、防火墙、8212、配置或存档，未执行 `clean-seeds`，未使用 Docker。
- 仍需真人验收：旧实体手机已安装 PWA 自动升级、真实管理员登录跳转、真实普通玩家登录跳转、第二名真实玩家上线与退出、长服务器名和角色名显示、Android 手势安全区域，以及实体手机任务布局。

## 2026-07-16 v0.4.4-dev UI deployment record

- Source commit: `29a553ca5250ecd004278d3816e1ad7fe302ed30`; version: `0.4.4-dev`; UTC build time: `2026-07-16T09:50:32Z`. The pure-Go Linux AMD64 artifact was built with `CGO_ENABLED=0`, size `11296952` bytes, SHA-256 `6468ab9f13140cbbd04ee7a44f99ee3647e5acc039ac4ee2f663c52e580fae53`, and retained on the host as `/tmp/palworld-companion-29a553c`.
- Embedded frontend assets are `assets/index-DTcWfPSc.js` (SHA-256 `ec8e2025e81cd0f92cf013c4c04c9e92f88ca92f5945d95bb484ac8e99aff2be`) and `assets/index-CHPPINm0.css` (SHA-256 `f87b026c1d7c31fa33f12f9c3f9442f113f6437eced1ec232cd415045f3f152a`); `sw.js` SHA-256 is `2f684e0476a510227fcbfdd7de04c4ba565e4955f42809e3b19228dc32570541`. Production downloads matched all local hashes. The manifest remains `start_url=/`, `scope=/`, and `display=standalone`; no PWA cache or authentication behavior was changed.
- The deployment backup is `/root/palworld-companion-backup-20260716-095440`, containing the previous binary, unchanged configuration, and the database copied after Companion stopped. WAL/SHM existed before shutdown but were cleanly checkpointed and absent after shutdown, so no sidecar files were copied. No rollback occurred.
- `/etc/palworld-companion/config.yaml` was not modified; its SHA-256 remained `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`. SQLite remained schema 5 in WAL mode with `foreign_key_check=0` and `setupRequired=false` (setup complete). Before deployment counts were `users=2`, `sessions=7`, `tasks=3`, `auth_flows=5`, and `player_roster=1`; after deployment the durable business counts remained `users=2`, `sessions=7`, `tasks=3`, and `player_roster=1`. `auth_flows` became 3 because the existing startup cleanup removes flows expired for more than 24 hours; no user, session, task, roster, or setup data was lost.
- `palworld-companion.service` remained enabled, active, and running. Its PID changed from `147951` (active since `2026-07-16 08:27:13 UTC`) to `149379` (active since `2026-07-16 09:54:40 UTC`), and health returned `0.4.4-dev`. Public HTTP still redirects 301 to HTTPS; HTTPS `/`, `/login`, `/tasks`, and `/account` returned 200; unauthenticated tasks returned 401; both legacy Steam routes returned 410; the public players endpoint returned 200. Post-deployment logs contained no panic, fatal, migration, or foreign-key errors.
- Browser acceptance used the exact deployed frontend build. Local authenticated checks covered 390x844, 412x915, 430x932, 768x1024, and 950x690 with no horizontal overflow or console errors. The account page rendered no password fields by default; expanding focused the current-password field, failure preserved the form and entered values, and cancel/unmount cleared them. A retained production PWA client first rendered its old cached bundle, then the existing update flow supplied `index-DTcWfPSc.js` to a new client; the five inline-SVG navigation icons rendered without console errors. No production password or test account was used.
- Only `palworld-companion.service` was stopped and started. `palworld-server.service` remained PID `113468`, active since `2026-07-15 09:35:34 UTC`; `palworld-pst.service` remained PID `107527`, active since `2026-07-15 01:48:58 UTC`. Nginx, FRP, firewall, port 8212 configuration, Palworld/PST configuration and data, player saves, and the SQLite schema were not modified. Docker and `clean-seeds` were not used.
- Remaining real-person acceptance: bring a second real player online, confirm total/current-online increases, then have that player leave and verify the next successful snapshot retains the roster row and `lastOnlineAt` while status becomes offline and ping/position are empty. This does not block the UI deployment.

## 2026-07-16 v0.4.5-dev compact server card deployment record

- Source commit: `dbf0c0d263090dbf933ee145de08350cfd2f8198`; version: `0.4.5-dev`; UTC build time: `2026-07-16T10:27:10Z`. The pure-Go Linux AMD64 artifact was built with `CGO_ENABLED=0`, size `11296952` bytes, SHA-256 `2ce8ca5cfa46f00160c30b486e74fde890bc43e7448fdbe28c1015fb442fee57`, and retained on the host as `/tmp/palworld-companion-dbf0c0d`.
- Embedded assets are `assets/index-B1V96bex.js` (SHA-256 `edc505c52b1afe210804457c29a2ce3e7c500009dc2c00a77bee26e2697bca24`) and `assets/index-3ssc7E1T.css` (SHA-256 `c720c26187ba8ca931b0c4da571143066f83354c4f0a6f072e595cb16b69b2e7`); `sw.js` SHA-256 is `19c0b103b32c7628fcb74472b85e0e48fe17b050ddbd35983a86c6a578392880`. Production downloads matched all local hashes. Manifest and Service Worker behavior were not changed.
- The home server description node was removed rather than hidden, and all three obsolete `.description` layout rules were removed. The card now has three natural-height layers: status plus version, clamped server name, and a single online-count row. The summary API and its `description` field remain unchanged.
- Authenticated local browser acceptance used the exact deployed frontend build. At 390x844, 412x915, 430x932, 768x1024, and 950x690, server-card heights were respectively `134.47`, `136.31`, `137.03`, `170.78`, and `176.22` px; task-title top offsets were `460.55`, `463.97`, `465.03`, `460.78`, and `466.22` px. Every viewport had zero horizontal overflow, zero server-description nodes/text, and zero console errors. The prior saved 0.4.4 390px reference card was approximately 158px high, while the user-provided real-phone reference was approximately 360px.
- The deployment backup is `/root/palworld-companion-backup-20260716-102840`, containing the previous binary, unchanged configuration, and the database copied after Companion stopped. WAL/SHM existed before shutdown but were cleanly checkpointed and absent after shutdown, so no sidecar files were copied. No rollback occurred.
- `/etc/palworld-companion/config.yaml` was unchanged; SHA-256 remained `194a975a1e55fa0be3e6d3c5f9e1a3239abbce4dff1d8302752287b7989df58e`. SQLite remained schema 5 in WAL mode with `foreign_key_check=0` and `setupRequired=false`. Before and after deployment, durable counts remained `users=2`, `sessions=7`, `tasks=3`, and `player_roster=1`. `auth_flows` changed from 3 to 0 because the existing startup cleanup removed expired one-time flows; no user, session, task, roster, or setup data was lost.
- `palworld-companion.service` remained enabled, active, and running. PID changed from `149379` (active since `2026-07-16 09:54:40 UTC`) to `150300` (active since `2026-07-16 10:28:40 UTC`), and health returned `0.4.5-dev`. HTTP continued to redirect 301 to HTTPS; HTTPS `/`, `/login`, `/tasks`, and `/account` returned 200; unauthenticated tasks returned 401; both legacy Steam routes returned 410; the players endpoint returned 200. Logs contained no panic, fatal, migration, or foreign-key errors.
- A retained production PWA tab initially remained on its previous Worker and asset, then a new client loaded `index-B1V96bex.js` through the existing update flow with no console errors. No production password or test account was used.
- Only `palworld-companion.service` was stopped and started. `palworld-server.service` remained PID `113468`, active since `2026-07-15 09:35:34 UTC`; `palworld-pst.service` remained PID `107527`, active since `2026-07-15 01:48:58 UTC`. Nginx, FRP, firewall, port 8212 configuration, Palworld/PST configuration or data, player saves, and SQLite schema were not modified. Docker and `clean-seeds` were not used.
