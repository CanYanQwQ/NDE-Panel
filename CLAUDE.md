# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概览

TMS 面板（代码内产品名 **Flux / flux-panel**）——一个翻墙协议管理 / 转发中转 / 每用户限速·流量·到期的中央管理面板。架构是「一台中央面板 + 多台转发机（节点机）」：

- **springboot-backend/**：Spring Boot 2.7.18 (Java 21) 中央面板后端，端口 **6365**，API 前缀 `/api/v1`，管理所有节点
- **vite-frontend/**：React 18 + TypeScript + HeroUI 前端（PC 侧栏 + H5 底栏双布局）
- **go-gost/**：节点端定制二进制（基于 go-gost，`go.mod` 里 `replace github.com/go-gost/x => ./x` 指向本地定制的 x/）
- **android-app/ + ios-app/**：同款前端 `vite build` 产物打包的 WebView 壳（多面板地址管理）
- 根目录：`gost.sql`（完整 schema）、`install.sh`（节点端安装）、`panel_install.sh`（面板端安装 + `tms` 命令）、`docker-compose-*.yml`、`.github/workflows/`（版本化镜像构建）

> ⚠️ 本项目是翻墙工具，代码注释中大量使用「车友」「落地」「机场」等行话（「车友」= 订阅用户/客户，「落地」= 中转出口节点）。README 说明仅供个人学习研究，使用者自负合规风险。

## 当前文档与验证基线（2026-09-20）

- 理解项目时优先级为：当前代码 > `HANDOFF.md` > 本文件；`HANDOFF.md` 记录最近部署、发布和功能变更，本文件只保留长期架构约束。
- 协议管理中的“分配用户”和“我自己用”都支持 `publicPort`：空值自动分配，填写后按起始端口、起始端口+1……分配整机协议；公网端口属于 `Forward.inPort`，不是 `Inbound.listenPort`。
- 当前已验证：`cd vite-frontend && npx tsc --noEmit`、`npx eslint src`（0 error，仍有既有格式 warning）、`npm run build`、`cd go-gost && go test ./...` 均通过。项目仍没有业务自动化测试套件。
- 面板安全更新必须保留 MySQL volume：允许 `docker compose down`，禁止未经明确授权执行 `docker compose down -v`；更新前应先导出数据库备份。更新应用镜像可使用 `docker compose up -d --no-deps backend frontend`，避免重建数据库。
- Android/iOS 是 WebView 壳；当前源码没有稳定的 `vite-frontend/dist` 到 Android assets/iOS Bundle 的自动拷贝链路，移动端 clean checkout 构建前必须先确认资源打包流程。
- 当前已知安全薄弱点仍未全部修复：无盐 MD5、节点 secret 放在 query 参数、明文 HTTP 上报、全开放 CORS、用户列表可能返回 `pwd`、默认管理员凭据等。修改这些问题前必须评估 Java/Go/部署端兼容性。

## 部署架构

| 角色 | 部署方式 | 组件 |
|---|---|---|
| 面板端（一台） | Docker compose（mysql:5.7 + backend + frontend） | `docker-compose-v4.yml`（拉镜像）/ `docker-compose-hybrid.yml`（本地 build） |
| 节点端（每台） | `install.sh` 安装 gost 裸二进制 + systemd | gost 连面板 WebSocket；协议功能现装 sing-box v1.13.12（systemd 服务） |

**节点通信双通道**（核心机制）：
1. **WebSocket 常驻**：节点 `ws://面板:端口/system-info?type=1&secret=密钥` 连面板，每 2 秒上报系统信息（CPU/内存/流量 + singbox 安装/运行状态）。面板通过 `WebSocketServer.send_msg(nodeId, data, type)` 下发指令（AddService/PauseService/SetSingboxConfig/…），`CompletableFuture.get(10s)` 同步等响应。消息用 AES-256-GCM 加密（密钥 = SHA-256(node.secret)），Java 端 `AESCrypto` 与 Go 端 `crypto` 兼容。
2. **HTTP 上报**：节点 `x/service/traffic_reporter.go` 每 5 秒把每个服务的流量增量 POST 到 `/flow/upload?secret=密钥`（成功上报后重置计数），`/flow/config` 每 10 分钟上报当前 gost 配置供面板清理孤儿。

**服务命名规范**：gost 服务名 = `{forwardId}_{userId}_{userTunnelId}`，TCP/UDP 各一条（后缀 `_tcp`/`_udp`），隧道转发额外有 `_tls`（远端）和 `_chains`（链）。暂停/恢复/清理孤儿全部依赖这个命名，改动必须全局一致。

## 核心业务架构

### 三类业务模型

1. **协议搭建（合体面板，最核心）**：数据流 `客户端 → gost 公网口(20000-39999，限速/计流量/到期) → 127.0.0.1:sing-box 入站(协议解密) → 外网`。
   - `inbound` 表：协议入站，sing-box 一律 **listen 127.0.0.1**（端口 40000+），公网口由 gost 转发占用并限速
   - `inbound_user` 表：车友在某入站的凭证（uuid/password）+ `gost_forward_id` 指向限速用的转发
   - `inbound_line` 表：**线路 = 车友 × 机器 × 落地**。每条线路独立订阅 token / 配额 / 到期。已用流量不落库，实时汇总该线路各转发 `in_flow+out_flow`
   - `landing` 表：落地出口（粘贴分享链接建成，解析成 sing-box outbound JSON），有 `landing_id` 的入站流量经它出网（中转），空 = 直连
   - 支持协议：VLESS-Reality / Trojan-Reality / VMess(可ws) / SS-2022 / Hysteria2 / TUIC / AnyTLS
2. **转发中转（老业务）**：`tunnel`（隧道 = 入口节点+出口节点，type=1 端口转发 / type=2 隧道转发）+ `forward`（单条转发 = 用户×隧道，自动分配端口）+ `user_tunnel`（用户对隧道权限）。
3. **限速/流量/到期**：`speed_limit` 规则 + gost limiter（mode 0=共享`$`/1=每连接`$$`/2=每客户端IP）。协议分配走**车友专属限速器**，名 = `900000000 + userId`（`CheckGostConfigAsync.PER_USER_LIMITER_BASE`，必须与 `InboundServiceImpl.perUserLimiterName` 保持一致，且配置巡检要对它放行）。

### 订阅机制

`/api/v1/open_api/sub`（base64 链接）和 `/clash`（YAML）**免登录**，按 token 匹配：
- 先查 `user.all_sub_token`（聚合订阅：该车友所有未停线路 + 分给他的转发的 `client_link`）
- 再查 `inbound_user.sub_token`（单线路）

停用的线路（`inbound_line.status=0`）从订阅中消失。聚合订阅节点名带「机器名→落地名」前缀；`inbound.remark` 非空（用户改过名）时听用户的、不加前缀。

### 定时任务（common/task/）

| 任务 | 调度 | 职责 |
|---|---|---|
| `CheckExpiryAsync` | 每分钟 | 主动到期巡检：账号到期/停用 → 停其全部转发；线路到期/跑满 → 停该线路转发并置 line.status=0 |
| `ResetFlowAsync` | 每天0点 + 每分钟 | 按 `flow_reset_time`(0-31，月末兜底)重置流量；**恢复跑满停掉的线路但到期的绝不复活**（resumeForward 有到期守卫）；`forwardExpiry()` 每分钟停单条到期转发 |
| `StatisticsFlowAsync` | 每小时 | 采样用户流量写入 statistics_flow |
| `CheckGostConfigAsync` | 触发式 | 节点上报配置时清理孤儿 services/chains/limiters |

## 关键工程约束（改代码前必读）

1. **批量操作「先入库不推送、最后统一推一次」**：一键搭 6 个协议、批量分配车友都是这个模式，避免反复重启 sing-box 触发 systemd 启动限流。
2. **续费/重分配不换 UUID/端口**：`/inbound/line-update`（updateLine）是续费的唯一正确路径，改配额/到期/限速三件事缺一不可（线路记录 + 每条 gost 转发 exp_time + 限速器下发）。删了重分会作废车友已导入的订阅。
3. **端口分配避让**：转发公网口（20000-39999）避让 sing-box 本机口（40000+）；节点 OS 层被占时自动顺延（`MAX_PORT_RETRY=20` / `MAX_PORT_TRIES=25`）。
4. **订阅中的 node 地址**用 `node.domain`（配了域名）优先，否则 `server_ip`。
5. **流量统计是增量上报**（节点 5 秒周期增量），后端用 `UpdateWrapper.setSql` 原子累加 + `synchronized` 按 key 加锁防并发覆盖。勿改成「覆盖式」写入。
6. **服务名解析** `parseServiceName` 按 `_` split 成 4 段 `[forwardId, userId, userTunnelId, type]`，改动命名/解析必须同步。

## 常用命令

```bash
# 后端
cd springboot-backend && mvn spring-boot:run        # 本地起后端（需 .env 的 DB_HOST/DB_NAME/DB_USER/DB_PASSWORD/JWT_SECRET/LOG_DIR）
cd springboot-backend && mvn package -DskipTests     # 打包

# 前端
cd vite-frontend && npm install && npm run dev       # 开发（Vite）
cd vite-frontend && npm run build                    # 构建（tsc && vite build）

# 数据库
# 完整 schema 在根目录 gost.sql（MySQL 5.7 / utf8mb4）。老库升级用 hybrid-schema-v1/v2/v3.sql（加法式迁移，可重复执行，1060 报错忽略）。

# 部署
./install.sh -a 面板地址:端口 -s 节点密钥            # 节点端安装
# 面板端：curl ... panel_install.sh（生成 tms 命令：tms update / status / domain / export / purge）
# 本地构建联调：docker compose -f docker-compose-hybrid.yml --env-file .env up -d --build
```

> 无测试套件（后端无单测，前端无测试配置）。后端无 test 目录；前端 `package.json` 无 test 脚本。

## 安全注意（已知薄弱点，改动时留意）

- 密码 **MD5 无盐**（`Md5Util.md5`，登录/创建都走无盐）；种子账号 `admin_user/admin_user`，首次登录强制改密
- JWT 手写 HS256（`jwt-secret` 环境变量），有效期 90 天；`JwtUtil.getUserIdFromToken(String)` 重载**不验签**直接解析 payload
- 节点鉴权仅凭 QueryString 的 `secret` 参数；`/flow/upload` 无速率限制；`FlowController` 的 `setSql("in_flow = in_flow + " + value)` 数值来自节点上报
- CORS 全放开；`getAllUsers()` 返回车友列表**未清空 pwd 字段**
- 以上是已知现状，不要在未确认的情况下「顺手修掉」——涉及跨端（Java 后端 + Go 节点）协议一致性，改动需全局评估。

## 跨语言约定

面板 → 节点是 **Java(Spring Boot) ↔ Go(gost 定制版)** 两套实现，靠 WebSocket JSON 消息和 HTTP 上报对齐。以下任何改动必须两端同步：
- 服务命名 `{forwardId}_{userId}_{userTunnelId}`（Java 端 GostUtil / Go 端 websocket_reporter.go）
- 车友限速器 `900000000+userId`
- AES-256-GCM 加密协议（Java `AESCrypto` / Go `crypto`，密钥 = SHA-256(secret)）
- sing-box 配置 JSON 结构（Java `SingboxUtil` 生成 / Go `x/socket/singbox.go` 写入 `/etc/gost/sing-box.json` 并 systemd 重启）
- Reality 密钥对：Java 调 `GenerateRealityKeypair`，Go 用 `sing-box generate reality-keypair` 生成
- 自签证书路径固定 `/etc/gost/certs/self.crt|self.key`（Hy2/TUIC/AnyTLS 用，客户端 insecure）

## 前端视觉约束

- 禁止使用蓝紫色渐变或相近的蓝紫渐变作为页面、背景、按钮、卡片或主要装饰。
- 禁止使用表情包、Emoji 或其他表情符号作为界面功能、按钮、导航、状态或提示的视觉元素。
- 图标只能使用图标库或矢量图，不得使用其他类型的图标资源。
- 优先使用文字、CSS、现有组件和清晰的布局层级表达功能；不要为了装饰额外引入资源或依赖。

## 编码行为准则

以下准则用于减少不必要的代码改动和未经验证的假设；与本项目已有约束合并执行。

### 1. 编码前先思考

- 不要假设，不要隐藏不确定性；明确说明假设、权衡和风险。
- 存在多种解释时先指出，不要静默选择。
- 优先采用能完成任务的最简单方案；发现需求不清时先询问。

### 2. 简单优先

- 只实现用户要求的功能，不添加未要求的特性。
- 单次使用的逻辑不要提前抽象，不要为了“灵活性”增加配置项。
- 如果 200 行代码可以简化为 50 行，应继续简化。

### 3. 外科式修改

- 只修改解决当前问题所必需的代码，不顺手重构相邻代码、注释或格式。
- 匹配现有项目风格，不因个人偏好改写实现。
- 由本次修改产生的无用 import、变量或函数必须清理；已有死代码不要擅自删除。
- 每一处改动都必须能直接对应到用户需求。

### 4. 目标驱动并验证

开始多步骤任务前，先列出简短计划，并为每一步定义验证方式：

```text
1. [步骤] → 验证：[检查方式]
2. [步骤] → 验证：[检查方式]
3. [步骤] → 验证：[检查方式]
```

“功能完成”必须有可执行的验证标准；修复问题时优先先复现，再修改，再验证修复结果。
