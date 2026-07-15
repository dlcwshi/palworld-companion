# 部署说明

本文件仅描述后续 Linux AMD64 部署方式。本轮没有连接、安装或重启任何服务器服务。

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

v0.1.0 没有 Companion 自身账户系统。公网使用前必须由 HTTPS 反向代理增加访问认证和速率限制，并保持 Companion 与 Palworld REST API 端口不直接暴露。部署、安装 unit 与重启服务应作为独立的明确授权任务执行。