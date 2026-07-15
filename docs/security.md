# 安全说明

## 密码

- 接受 8–128 字节密码，不强制字符组合。
- 使用 Argon2id、19 MiB 内存、2 次迭代、单并行度、16 字节随机盐和 32 字节输出。
- PHC 字符串保存算法版本和参数，以支持未来升级。
- 验证时限制内存、迭代、并行度、盐和输出长度，并使用恒定时间比较。
- 密码、确认密码、密码哈希和完整认证请求体不会写入日志。

## Session

Session Token 由 `crypto/rand` 生成，原始值只进入 Secure、HttpOnly、SameSite=Lax、Path=/ Cookie。数据库保存 SHA-256 哈希。注销、禁用、删除、管理员重置密码和显式撤销均设置 `revoked_at`。

用户修改密码会验证当前密码、更新 Argon2id 哈希并撤销除当前 Token 之外的 Session。管理员重置密码不读取旧密码或旧哈希，并撤销全部目标 Session。

## 初始化

`system_settings.setup_completed` 是唯一初始化开关。创建首任管理员时在事务内重新检查该值；成功后永久设置为 true。删除或损坏管理员不会重开 Web 初始化，CLI `users create-admin` 用于受控恢复。

## 注册与身份

玩家注册只接受 SteamID64 和密码。角色、状态、Palworld 标识和内部 ID 均由服务端生成，未知 JSON 字段会被拒绝。注册实时调用 `/players`，严格匹配 `userId=steam_<SteamID64>`；不可用、离线或 stale 数据不会创建申请。

数据库对 SteamID64、Palworld userId 和非空 playerId 建立唯一索引。角色名不是身份键。所有生命周期状态保留唯一标识，软删除和拒绝不能通过重复注册绕过。

## 管理员与任务

管理员保护在事务中统计 active 管理员，阻止自禁用、自删除以及最后一个 active 管理员被禁用、删除或降级。普通玩家不能调用审批、密码重置或角色接口。

个人任务只对所有者和管理员可见；共享任务对所有登录用户可见，但只允许创建者和管理员修改。无权对象按不存在处理。

## 限速与错误

登录、注册、初始化、修改密码和重置密码使用有限容量、每 IP 的进程内窗口限速。错误响应只包含稳定 code 和安全 message；SQL、路径、内部堆栈、上游凭据和完整玩家对象不会返回。

## 已停用的 Steam OpenID

旧 Steam 路由固定返回 HTTP 410 和 `steam_auth_disabled`。运行时没有 OpenID verifier 或外部 HTTP 客户端装配，不会向 `steamcommunity.com` 或 Steam Web API 发请求。旧 `auth_flows` 表和兼容配置字段暂时保留，不参与认证。
