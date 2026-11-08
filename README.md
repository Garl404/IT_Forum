# CALUSE · IT 技术论坛

CALUSE 是一个使用 Go 编写的前后端分离 IT 技术论坛，面向最高 9000 并发在线用户设计，内置内容审核、DDoS 自动防护、访问统计与每周自动数据报告。

- 仓库：https://github.com/Garl404/IT_Forum
- 提交 Bug：https://github.com/Garl404/IT_Forum/issues/new
- 提交补丁：https://github.com/Garl404/IT_Forum/pulls

## 开源协议

[MIT](LICENSE) · Copyright (c) 2026 Garl404 —— 所有用户均可自由使用、修改、分发，欢迎提交 Bug 与补丁。

## 参与贡献

- 贡献指南：[CONTRIBUTING.md](CONTRIBUTING.md)
- 安全漏洞报告：[SECURITY.md](SECURITY.md)（请勿公开提交漏洞）
- Bug 提交：[Issues](https://github.com/Garl404/IT_Forum/issues/new/choose)
- 补丁提交：[Pull Requests](https://github.com/Garl404/IT_Forum/pulls)

本地开发常用命令（需 Go 1.22+ 与 Node.js）：

```bash
make build   # 编译
make test    # 单元测试
make vet     # 静态检查
make check   # CI 同款全量检查（build + vet + test + JS 语法 + 脚本语法）
```

CI（GitHub Actions）会在每次推送与 PR 时自动执行 `make check` 相同流程。

## 功能总览

| 模块 | 说明 |
|---|---|
| 用户系统 | 注册 / 登录（Argon2id 加密）、JWT 会话、10 级用户等级（注册时长 + 发帖频率 + 评论 + 举报贡献） |
| 发帖 | Markdown 正文、单帖上限 **65536 字**、分类浏览、搜索、评论、浏览计数 |
| 内容安全 | **离线违禁词库**（多语言、带严重等级）、变体归一化检测（全角/形近字/leet/分隔符绕过）、命中即转人工审核 |
| 举报与审核 | 支持**匿名举报** → 管理员人工审核：无异常则保留帖子并**赔偿精神损失（+2GB 上传额度，最多 3 次）**；确认违禁则下架帖子并**按违禁程度封禁**（1级=1天、2级=3天、3级=7天、4级=30天、5级=永久） |
| 文件上传 | 普通用户总配额 **5GB**、**10 级用户 15GB**（含补偿额度），单文件 ≤2GB，危险扩展名拦截、SHA256 记录、仅所有者可下载 |
| DDoS 防护 | 实时 RPS/单 IP 频率监控，**流量异常自动关闭网站**（保护盾模式），并向管理员邮箱发送**数据异常告警邮件**；流量恢复自动解除；单 IP 高频自动临时封禁 |
| 访问统计 | 全量访问日志（IP/UA/路径）、实时仪表盘（RPS、今日访问、Top IP）、7 日趋势图 |
| 周度报告 | **每周一（公历 ISO 周）自动生成**上一周网站数据报告（访问量、独立 IP、新增用户/帖子/评论、上传、举报、封禁等），存入数据库 + JSON 归档 + 邮件发送 |
| 管理后台 | 仪表盘、内容审核队列、举报处理、用户管理（封禁/等级/配额）、**告警邮箱管理、管理员可信 IP/设备指纹管理**、盾手动开关、周报查看、操作日志、违禁词库查看 |
| 管理员安全 | 站长账号登录记录 IP + 浏览器指纹，**新设备登录需邮箱验证码**（OTP）；可信设备可一键清除 |
| 前端 | 玻璃拟态（毛玻璃）+ 扁平化 UI，**亮色/深色/跟随系统三模式**（`prefers-color-scheme` 自动检测），**13 种语言**（简中/繁中/英/日/韩/俄/德/法/西/葡/阿/印地/越），零外部依赖 SPA |

## 目录结构

```
Linux-Forum/
├── backend/
│   ├── cmd/server/main.go          # 入口
│   ├── internal/
│   │   ├── config/                 # 配置加载
│   │   ├── database/               # SQLite（WAL）、建表、种子数据
│   │   ├── models/                 # 数据模型
│   │   ├── auth/                   # Argon2id、JWT、OTP、指纹
│   │   ├── middleware/             # 安全头、限流、认证、封禁、盾检查
│   │   ├── monitor/                # DDoS 检测、访问统计、周报、邮件
│   │   ├── moderation/             # 违禁词库与检测引擎
│   │   ├── level/                  # 用户等级
│   │   ├── handlers/               # API 处理器
│   │   ├── router/                 # 路由与静态资源
│   │   └── version/                # 版本号
│   ├── data/
│   │   ├── banned_words.json       # 离线违禁词库（可自行扩充）
│   │   ├── reports/                # 周报 JSON 归档（运行时生成）
│   │   └── outbox/                 # 未配置 SMTP 时的邮件存档（运行时生成）
│   ├── uploads/                    # 用户上传文件（运行时生成）
│   ├── config.yaml                 # 主配置
│   └── caluse-server               # 编译产物（不入库）
├── frontend/
│   ├── index.html
│   ├── css/style.css               # 玻璃拟态设计系统（明暗双主题）
│   └── js/                         # i18n / pages / markdown / charts / api / app / admin
├── scripts/caluse.sh               # 启动管理脚本（systemd/OpenRC/SysV 通用）
├── .github/                        # CI 工作流、Issue/PR 模板
├── Makefile                        # build/test/vet/check 等开发命令
├── CONTRIBUTING.md                 # 贡献指南
├── SECURITY.md                     # 安全漏洞报告流程
└── LICENSE                         # MIT
```

## 快速开始

```bash
# 需要 Go 1.22+
cd backend
go build -o caluse-server ./cmd/server
./caluse-server
# 打开 http://localhost:8080
```

**站长账号**（首次启动自动创建）：`caluse_admin` / `caluse_665544Xx`
> 首次登录后请立即在「个人中心 → 修改密码」，并在管理后台「告警邮箱」中把默认占位邮箱换成真实邮箱。

## 启动脚本（Linux 主流发行版通用）

[scripts/caluse.sh](scripts/caluse.sh) 一个脚本搞定启动管理，自动识别 systemd / OpenRC / SysV init：

```bash
cd scripts
./caluse.sh start        # 启动（无二进制时自动编译 + 健康检查）
./caluse.sh stop         # 停止
./caluse.sh restart      # 重启
./caluse.sh status       # 运行状态 + 健康检查
./caluse.sh logs         # 实时日志
./caluse.sh build        # 仅编译
sudo ./caluse.sh install # 安装为系统服务（开机自启、崩溃自动拉起）
sudo ./caluse.sh uninstall
```

- 自动构建：检测到 Go 则编译，否则使用已有二进制；国内网络自动走 goproxy.cn
- 系统服务：Debian/Ubuntu/CentOS/RHEL/Rocky/Alma/Fedora/Arch/openSUSE → systemd；Alpine/Gentoo → OpenRC；老旧系统 → SysV init
- systemd 单元自带安全加固（NoNewPrivileges、ProtectSystem、PrivateTmp）与资源限制（LimitNOFILE=65535）
- 可选环境变量：`CALUSE_BACKEND`（后端目录）、`CALUSE_CONFIG`（配置文件）、`CALUSE_USER`（服务运行用户）、`CALUSE_GOPROXY`
- 安装后如需对外 80/443 端口，请配置 Nginx/Caddy 反向代理，并开启 `server.trust_proxy`

## 配置说明（config.yaml）

- `security.jwt_secret`：生产环境**必须修改**为随机长字符串
- `server.trust_proxy`：仅在 Nginx 等反向代理后开启（否则 X-Forwarded-For 可被伪造绕过防护）
- `ddos.*`：触发阈值（RPS）、突增倍数、恢复条件、单 IP 阈值、白名单 IP
- `email.*`：SMTP 配置；未开启时告警邮件写入 `data/outbox/`
- `content.*`：字数限制、上传配额（5GB/15GB）、补偿额度、违禁词库路径
- `report.weekly_iso_day: 1`：每周一自动生成周报

## 部署建议（9000 并发）

1. **反向代理**：Nginx/Caddy 前置，开启 `trust_proxy`，由代理限制请求体大小
2. **数据库**：SQLite 已启用 WAL + 大缓存 + busy_timeout，可满足中等负载；超大并发建议将 `database` 层替换为 PostgreSQL（代码为 `database/sql` 标准接口，迁移成本低）
3. **系统参数**：`ulimit -n 1048576`；内核 `net.core.somaxconn=65535`、`net.ipv4.tcp_tw_reuse=1`
4. **邮件**：配置真实 SMTP，否则告警邮件仅存档本地
5. **违禁词库**：按需扩充 `data/banned_words.json` 后可在后台一键重新加载（无需重启）

## API 摘要

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/register` `/api/auth/login` `/api/auth/verify-otp` | 认证（管理员新设备触发 OTP） |
| GET/POST | `/api/posts` | 帖子列表 / 发帖（自动违禁检测） |
| GET | `/api/posts/{id}` `/api/posts/{id}/comments` | 详情 / 评论 |
| POST | `/api/posts/{id}/report` `/api/comments/{id}/report` | 举报（可匿名） |
| POST/GET | `/api/uploads` | 上传 / 我的文件（配额控制） |
| GET | `/api/stats` `/api/categories` `/api/health` | 公开统计 |
| GET | `/api/admin/dashboard` | 实时流量仪表盘 |
| GET/POST | `/api/admin/review/posts[/{id}]` | 审核队列 / 审核（approve/reject） |
| GET/POST | `/api/admin/reports[/{id}/review]` | 举报处理（approve/confirm） |
| GET/POST | `/api/admin/users[/{id}]` | 用户管理（ban/unban/set_level/grant_quota） |
| GET/POST/DELETE | `/api/admin/emails` | 告警邮箱管理 |
| GET/POST | `/api/admin/trust[/{id}/clear]` | 管理员可信 IP/指纹 |
| POST | `/api/admin/shield` | 保护盾手动开关 |
| GET/POST | `/api/admin/weekly[/generate]` | 周报查看 / 手动生成 |
| GET | `/api/admin/banned-words` | 违禁词库 |
| GET | `/api/admin/logs` | 审核/安全操作日志 |

## 安全设计要点

- Argon2id 密码哈希、JWT（HMAC-SHA256）、登录/注册/发帖/上传分级限流（令牌桶）
- CSP / X-Frame-Options / nosniff 等安全响应头；前端 Markdown 渲染器默认转义 HTML 防 XSS
- SQL 全部参数化查询；上传文件随机存储名 + 危险扩展名黑名单 + 路径穿越防护 + 配额双重校验
- 封禁用户在中间件层拦截全部写操作；管理员敏感操作全程审计日志
- 盾模式开启后仅白名单 IP 与登录/健康检查端点可访问，其余请求返回 503

## 用户等级（LV1–LV10）

成长分 = 注册天数×10 + 发帖×50 + 评论×20 + 确认举报贡献×150
阈值：0 / 400 / 1200 / 2600 / 4800 / 8000 / 12500 / 18500 / 26500 / 37000
（管理员可在后台手动调整等级；10 级享 15GB 上传配额）
