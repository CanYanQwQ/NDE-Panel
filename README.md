# NDE Panel

> NDE Panel 是一个面板 + 多节点的协议管理、转发中转、限速、流量与线路订阅系统。

- 项目仓库：<https://github.com/CanYanQwQ/NDE-Panel>
- 上游项目：<https://github.com/Teminuosi/Tms>
- 许可证：Apache License 2.0，见 [LICENSE](LICENSE)

> 本项目是基于上游 TMS 的二次开发版本。NDE Panel 的新增代码、界面、部署调整和功能修改由当前维护者维护；上游项目的版权、许可证和归属信息继续保留，不代表本项目是上游项目的原始作者版本。

## 项目能力

| 模块 | 功能 |
|---|---|
| 协议管理 | 创建 VLESS-Reality、Trojan-Reality、VMess、Shadowsocks 2022、Hysteria2、TUIC、AnyTLS |
| 节点管理 | 管理多台 gost 节点，查看在线状态、系统信息、流量和 sing-box 状态 |
| 中转与落地 | 前置机协议 + 落地出口，支持落地链接测试和线路管理 |
| 用户与订阅 | 用户、线路、订阅 token、普通订阅和 Clash/Mihomo YAML 订阅 |
| 公网端口 | 分配用户或“我自己用”时指定公网端口起始值，也支持自动分配 |
| 限速与流量 | 用户独立限速、TCP/UDP 流量统计、配额、重置和到期控制 |
| 传统转发 | 普通端口转发、双节点隧道转发、暂停/恢复和诊断 |
| 多端界面 | React 管理面板、H5 布局，以及 Android/iOS WebView 壳 |

## 系统架构

```text
浏览器 / Android / iOS
          |
          v
NDE Panel 前端 + Spring Boot 后端 + MySQL
          |
          | WebSocket 指令 / HTTP 流量上报
          v
多台 gost 节点
          |
          v
公网 gost 端口 -> 127.0.0.1 sing-box -> 直连或落地出口
```

### 面板端

- 前端：React 18、TypeScript、Vite、HeroUI
- 后端：Spring Boot 2.7.18、Java 21、MyBatis-Plus
- 数据库：MySQL 5.7
- 默认后端端口：6365
- 默认前端端口：6366

### 节点端

- 定制 gost 二进制位于 `go-gost/`
- 节点通过 WebSocket 连接面板并接收服务、限速器、链路和 sing-box 配置
- 节点每 2 秒上报系统状态，每 5 秒上报流量增量
- 节点可使用 systemd、OpenRC、SysV 或容器 supervisor 管理服务
- 协议入站由 sing-box 监听本机回环地址，公网端口由 gost 负责

## 快速部署

### 1. 部署面板

找一台安装了 Linux 和 Docker 的服务器执行：

```bash
curl -fsSL https://raw.githubusercontent.com/CanYanQwQ/NDE-Panel/main/panel_install.sh \
  -o panel_install.sh && chmod +x panel_install.sh && ./panel_install.sh
```

安装脚本会自动：

1. 生成随机数据库名称、数据库账号、数据库密码和 JWT secret；
2. 下载当前仓库的 compose 与 `gost.sql`；
3. 创建 MySQL、backend、frontend 三个容器；
4. 初始化面板后端地址；
5. 安装 `tms` 管理命令。

默认端口：

- 前端：6366
- 后端：6365

默认管理员账号仅用于首次进入：

```text
账号：admin_user
密码：admin_user
```

首次登录后必须立即修改密码。

### 2. 添加节点

1. 登录面板；
2. 进入“转发机”；
3. 点击“新增”，填写节点名称、服务器 IP、端口范围；
4. 保存节点；
5. 点击节点的“安装命令”；
6. 将面板生成的命令复制到节点服务器执行。

节点安装命令会自动带上面板地址和该节点专属 secret，不要手动复用其他节点的 secret。

国内网络访问 GitHub 不稳定时，可以使用镜像下载脚本：

```bash
curl -L https://ghfast.top/https://raw.githubusercontent.com/CanYanQwQ/NDE-Panel/main/install.sh \
  -o install.sh && chmod +x install.sh \
  && ./install.sh -c -a 面板地址:6365 -s 节点密钥
```

如果镜像不可用，可将 `ghfast.top` 换成其他可用镜像，并通过 `GH_MIRROR` 指定节点内部下载镜像。

## 本地源码部署

用于开发和联调：

```bash
git clone https://github.com/CanYanQwQ/NDE-Panel.git
cd NDE-Panel
cp .env.example .env  # 如果项目提供该文件；否则按 compose 变量创建 .env
docker compose -f docker-compose-hybrid.yml --env-file .env up -d --build
```

源码构建版使用本地：

- `gost.sql`
- `springboot-backend/`
- `vite-frontend/`
- `go-gost/`

## 数据和安全更新

### 数据库持久化

Docker 使用以下 named volume：

- `mysql_data`：MySQL 数据
- `backend_logs`：后端日志

正常更新应用时保留数据卷：

```bash
docker compose -f docker-compose-hybrid.yml --env-file .env \
  up -d --build --no-deps backend frontend
```

更新前建议先导出备份：

```bash
tms export
```

除非明确要清空数据库，否则禁止执行：

```bash
docker compose down -v
```

`down -v` 会删除 MySQL 数据卷，所有用户、节点、协议、线路、订阅和流量记录都会丢失。

### 旧库迁移

- 新安装使用根目录的 `gost.sql`；
- 旧转发业务升级到协议/线路功能时，需要根据版本执行 `hybrid-schema-v1.sql`、`hybrid-schema-v2.sql`、`hybrid-schema-v3.sql`；
- 不要把新安装和旧库迁移路径混用；
- 迁移前必须先导出数据库。

## 订阅

普通订阅：

```text
/api/v1/open_api/sub?token=...
```

Clash/Mihomo 订阅：

```text
/api/v1/open_api/clash?token=...
```

订阅地址免登录，通过 token 匹配用户聚合订阅或单线路订阅。

节点地址优先使用节点配置中的 `domain`，没有域名时使用 `server_ip`。

停用、到期或流量用尽的线路会从订阅中消失。续费应使用线路更新功能，不要删除后重新分配，否则可能更换用户已经导入的 UUID、端口或订阅内容。

## 公网端口规则

- `Inbound.listenPort` 是 sing-box 本机端口，通常位于 40000 以上；
- `Forward.inPort` 是用户实际连接的 gost 公网端口；
- 分配用户和“我自己用”都支持公网端口起始值；
- 空值表示自动分配；
- 批量分配会使用起始端口、起始端口 + 1、起始端口 + 2；
- 指定端口冲突不会静默换成其他端口；
- 续费和线路更新不能随意更换已有端口或 UUID。

## 常用开发命令

前端：

```bash
cd vite-frontend
npm install
npm run dev
npx tsc --noEmit
npm run build
```

后端：

```bash
cd springboot-backend
mvn spring-boot:run
mvn package -DskipTests
```

节点：

```bash
cd go-gost
go test ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o gost .
```

当前项目没有完整的业务自动化测试套件，发布前应重点测试：登录、权限、节点上线、协议分配、订阅、端口、流量、到期和续费流程。

## 当前已知限制

- Android/iOS 是 WebView 壳，源码中的前端 dist 到原生 assets/Bundle 的自动拷贝链路需要单独确认；
- 节点 secret 当前仍属于高敏感认证信息，部署时不要将其提交到仓库或公开日志；
- 密码历史存储方式、JWT 和节点 HTTP 上报安全策略仍需单独进行兼容性评估；
- 面板、节点、sing-box 和数据库版本需要保持协议兼容。

## 版权、上游和许可证

NDE Panel 是基于上游项目 [Teminuosi/Tms](https://github.com/Teminuosi/Tms) 的二次开发版本。当前仓库地址为 [CanYanQwQ/NDE-Panel](https://github.com/CanYanQwQ/NDE-Panel)。

- 上游项目的版权、许可证和归属信息继续保留；
- 本项目新增或修改部分由当前维护者维护；
- 项目整体继续遵守 Apache License 2.0；
- 再分发时必须同时保留 [LICENSE](LICENSE) 和 [NOTICE](NOTICE)；
- 不得移除上游许可证、版权或修改声明；
- `tms`、TMS 数据表/环境变量、容器名以及 Android 的 `com.flux` 等属于兼容性标识，不代表当前项目的品牌归属，未经兼容性评估不要重命名。

## 免责声明

本项目仅供合法、合规的个人学习与研究使用。使用者必须遵守所在国家或地区的法律法规，并对自己的配置、网络行为、数据安全和服务使用承担全部责任。

项目维护者不对因使用、配置、部署或修改本项目造成的直接或间接损失承担责任，也不保证项目适用于任何特定生产环境。禁止将本项目用于未经授权的访问、攻击、数据窃取、滥用或其他违法行为。

如不同意上述说明，请停止使用本项目。
