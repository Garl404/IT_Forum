# 贡献指南

感谢你对 CALUSE 的关注！所有用户都可以参与贡献：报告 Bug、提出建议、提交补丁。

## 报告 Bug

1. 前往 [Issues](https://github.com/Garl404/IT_Forum/issues/new/choose)，选择 **Bug 报告** 模板
2. 尽量提供：问题描述、最小复现步骤、期望行为、运行环境（系统/Go 版本/浏览器）、相关日志
3. 确认没有重复的 Issue

> 安全漏洞请勿公开提交，请按 [SECURITY.md](SECURITY.md) 的流程报告。

## 提交补丁

1. Fork 本仓库并创建功能分支：
   ```bash
   git checkout -b fix/my-bug
   ```
2. 修改代码并确保本地验证通过：
   ```bash
   cd backend
   go build ./... && go vet ./... && go test ./...
   cd ../frontend && for f in js/*.js; do node --check "$f"; done
   sh -n ../scripts/caluse.sh
   ```
3. 提交（提交信息简明描述动机）并推送到你的 Fork
4. 发起 [Pull Request](https://github.com/Garl404/IT_Forum/pulls)，填写 PR 模板

## 代码规范

- Go：`gofmt` 格式化；新功能应包含单元测试；避免引入不必要的依赖
- 前端：原生 JS，无构建工具、无外部 CDN；改动需在明暗双主题与多语言下验证
- 内容审核相关改动需同步考虑 `backend/data/banned_words.json` 与测试用例
- 提交信息风格：`修复: ...` / `新增: ...` / `文档: ...`

## 项目结构

```
.
├── backend/          # Go 后端（net/http + SQLite，无框架依赖）
│   ├── cmd/server/   # 入口
│   ├── internal/     # config / database / models / auth / middleware
│   │                 # monitor / moderation / level / handlers / router / version
│   └── data/         # banned_words.json 违禁词库（运行时数据不入库）
├── frontend/         # 原生 JS SPA（无构建）
│   ├── css/          # 玻璃拟态设计系统（明暗双主题）
│   └── js/           # i18n / pages / markdown / charts / api / app / admin
├── scripts/          # caluse.sh 启动脚本（systemd/OpenRC/SysV）
└── .github/          # CI 与 Issue/PR 模板
```

## 行为准则

- 友善交流，禁止人身攻击与歧视言论
- 尊重他人劳动成果，引用需注明来源
