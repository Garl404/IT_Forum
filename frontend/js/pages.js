// CALUSE 静态内容页（页脚链接的落地页）
window.PAGES = {
  about: {
    "zh-CN": {
      title: "关于我们",
      blocks: [
        { p: "CALUSE 是一个面向 IT 从业者与技术爱好者的技术论坛。在这里，你可以讨论 Linux、编程、网络、安全、硬件、DevOps、AI 等话题，分享经验、解决问题、结识同好。" },
        { p: "站点内置多层安全防护：DDoS 攻击自动关站保护、异常流量邮件告警、访问数据全量统计，并在每周一自动生成上一周的网站数据报告。" },
        { p: "发布内容将自动经过离线违禁词库检测，违规内容由管理员人工审核处理，为每一位用户营造安全、友善的交流环境。" },
        { h: "开源", p: "CALUSE 基于 MIT 协议完全开源，所有用户均可自由使用、修改与分发，并欢迎通过 GitHub 参与开发。" },
        { links: [
          { text: "GitHub 仓库", href: "https://github.com/Garl404/IT_Forum" },
          { text: "提交 Bug", href: "https://github.com/Garl404/IT_Forum/issues/new" },
          { text: "提交补丁", href: "https://github.com/Garl404/IT_Forum/pulls" },
        ] },
      ],
    },
    "en-US": {
      title: "About us",
      blocks: [
        { p: "CALUSE is a technology forum for IT professionals and enthusiasts. Discuss Linux, programming, networking, security, hardware, DevOps, AI and more — share experience, solve problems and meet peers." },
        { p: "The site ships with multi-layer protection: automatic shutdown under DDoS attack, email alerts for traffic anomalies, full visit analytics, and automated weekly data reports every Monday." },
        { p: "All content is screened against an offline banned-word library; violations are handled by human moderators to keep the community safe and friendly." },
        { h: "Open source", p: "CALUSE is fully open source under the MIT license. Everyone is free to use, modify and redistribute it, and contributions on GitHub are welcome." },
        { links: [
          { text: "GitHub repository", href: "https://github.com/Garl404/IT_Forum" },
          { text: "Report a bug", href: "https://github.com/Garl404/IT_Forum/issues/new" },
          { text: "Submit a patch", href: "https://github.com/Garl404/IT_Forum/pulls" },
        ] },
      ],
    },
  },
  announce: {
    "zh-CN": {
      title: "站务公告",
      blocks: [
        { h: "2026-11-08 · CALUSE 正式上线", p: "欢迎各位技术爱好者注册交流。注册后即可发帖、评论与上传文件。" },
        { h: "社区规范", p: "发帖前请阅读发帖须知，并遵守社区准则。违规内容将被下架，账号按违禁程度封禁。" },
        { h: "数据报告", p: "站点每周一自动生成上一周的数据报告（访问量、独立 IP、发帖、上传、举报等），管理员可在后台查看。" },
      ],
    },
    "en-US": {
      title: "Announcements",
      blocks: [
        { h: "2026-11-08 · CALUSE launched", p: "Welcome! Register to post, comment and upload files." },
        { h: "Community rules", p: "Read the posting rules before posting and follow the community guidelines. Violations are removed and accounts banned by severity." },
        { h: "Data reports", p: "Weekly site reports (visits, unique IPs, posts, uploads, reports and more) are generated automatically every Monday and available in the admin console." },
      ],
    },
  },
  help: {
    "zh-CN": {
      title: "帮助中心",
      blocks: [
        { h: "如何发帖？", p: "登录后点击顶部「发新帖」，选择分类、填写标题与内容，勾选同意发帖须知后即可发布。正文支持 Markdown 语法，单帖不超过 65536 字。" },
        { h: "上传文件有什么限制？", p: "普通用户上传总配额 5GB，10 级用户 15GB，单文件不超过 2GB。可在「文件管理」中查看用量与已上传文件。" },
        { h: "内容审核是如何进行的？", p: "发布内容会自动经过离线违禁词库检测；命中违禁词的内容不会立即发布，将转入人工审核，审核通过后自动发布。经审核确认违禁的内容将被下架，账号按违禁程度封禁（1级=1天、2级=3天、3级=7天、4级=30天、5级=永久）。" },
        { h: "如何举报违规内容？", p: "在帖子或评论处点击「举报」按钮，填写理由后提交（支持匿名）。管理员将进行人工审核：内容无异常则保留，作者获得 +2GB 上传额度补偿；确认违禁则下架并封禁。" },
        { h: "用户等级如何计算？", p: "用户共 10 级，由注册时长、发帖频率、评论数与举报贡献综合评定。等级提升会带来更大上传配额等权益。" },
      ],
    },
    "en-US": {
      title: "Help center",
      blocks: [
        { h: "How do I post?", p: "Sign in, click New post, pick a category, fill in the title and content, agree to the posting rules and publish. Markdown is supported; posts are limited to 65536 characters." },
        { h: "Upload limits", p: "Regular users get a 5GB total upload quota; level 10 users get 15GB. Single files are limited to 2GB. Manage usage under Files." },
        { h: "How does moderation work?", p: "Content is screened against an offline banned-word library. Flagged content is held for manual review and published automatically once approved. Confirmed violations are removed and accounts banned by severity (L1=1d, L2=3d, L3=7d, L4=30d, L5=permanent)." },
        { h: "How do I report content?", p: "Click Report on any post or comment and submit a reason (anonymous reporting is supported). If content is found innocent, the author receives +2GB upload quota as compensation; confirmed violations are removed and banned." },
        { h: "How are user levels calculated?", p: "There are 10 levels based on account age, posting frequency, comments and report contributions. Higher levels unlock benefits such as larger upload quotas." },
      ],
    },
  },
  contact: {
    "zh-CN": {
      title: "联系管理员",
      blocks: [
        { h: "举报违规内容", p: "请使用帖子或评论内的「举报」按钮提交，支持匿名举报。管理员将进行人工审核，并依据结果执行保留+补偿或下架+封禁。" },
        { h: "其他事务", p: "请发送邮件至管理员邮箱（以管理后台「告警邮箱」中配置的地址为准）。邮件中请附上相关链接与必要说明。" },
        { h: "安全告警", p: "当站点遭遇异常流量（DDoS）时，系统会自动关闭网站并向管理员邮箱发送数据异常告警，无需人工干预。" },
      ],
    },
    "en-US": {
      title: "Contact admin",
      blocks: [
        { h: "Report abuse", p: "Use the Report button on any post or comment (anonymous reports are supported). Moderators review manually and either keep the content with compensation or remove it and ban the account." },
        { h: "Other matters", p: "Email the administrator address configured under Alert emails in the admin console. Please include relevant links and details." },
        { h: "Security alerts", p: "Under abnormal traffic (DDoS), the site shuts down automatically and sends an anomaly alert to the administrator's email — no manual action needed." },
      ],
    },
  },
  terms: {
    "zh-CN": {
      title: "用户协议",
      blocks: [
        { p: "欢迎使用 CALUSE（以下简称「本论坛」）。注册或使用本论坛即表示您已阅读并同意本协议与《社区准则》。" },
        { h: "1. 账号", p: "一个邮箱地址只能注册一个账号。请妥善保管账号与密码，因保管不善造成的损失由用户自行承担。" },
        { h: "2. 内容责任", p: "用户对其发布的帖子、评论、文件等内容负全部责任，不得发布违法违规内容。本论坛有权对违规内容进行下架处理。" },
        { h: "3. 违规处理", p: "经审核确认违禁的内容将被下架，账号按违禁程度封禁：1级=1天、2级=3天、3级=7天、4级=30天、5级=永久封禁。" },
        { h: "4. 误判补偿", p: "举报或审核被判定为无异常时，作者将获得 +2GB 上传额度作为精神损失补偿（最多 3 次）。" },
        { h: "5. 协议变更", p: "本协议可能随时更新，更新后继续使用本论坛即视为同意新协议。" },
      ],
    },
    "en-US": {
      title: "Terms of service",
      blocks: [
        { p: "Welcome to CALUSE. By registering or using this forum you agree to these terms and the Community Guidelines." },
        { h: "1. Accounts", p: "Each email address may register only one account. Keep your credentials safe; you are responsible for your account." },
        { h: "2. Content responsibility", p: "Users are fully responsible for posts, comments and files they publish. Illegal content is prohibited and will be removed." },
        { h: "3. Violations", p: "Confirmed violations are removed and accounts banned by severity: L1=1d, L2=3d, L3=7d, L4=30d, L5=permanent." },
        { h: "4. Compensation", p: "When a report or review finds content innocent, the author receives +2GB upload quota as compensation (max 3 times)." },
        { h: "5. Changes", p: "These terms may be updated at any time; continued use constitutes acceptance." },
      ],
    },
  },
  privacy: {
    "zh-CN": {
      title: "隐私政策",
      blocks: [
        { h: "我们收集什么", p: "账号信息（用户名、邮箱）、登录 IP、设备指纹（仅用于管理员账号安全验证）、访问日志（IP、User-Agent、访问路径，用于安全防护与统计）。" },
        { h: "如何使用", p: "数据仅用于：DDoS 攻击检测与防护、异常登录识别、站点访问统计与每周数据报告、内容审核。" },
        { h: "数据保留", p: "访问日志保留 90 天；周报数据长期保留用于站点运营分析。" },
        { h: "数据共享", p: "我们不会向第三方出售或共享您的个人信息，法律另有规定的除外。" },
        { h: "您的权利", p: "您可联系管理员申请查询、更正或删除您的账号数据。" },
      ],
    },
    "en-US": {
      title: "Privacy policy",
      blocks: [
        { h: "What we collect", p: "Account data (username, email), login IP, device fingerprint (admin account verification only), and access logs (IP, user agent, path — used for protection and analytics)." },
        { h: "How we use it", p: "Data is used solely for: DDoS detection and protection, abnormal login detection, site analytics and weekly reports, and content moderation." },
        { h: "Retention", p: "Access logs are kept for 90 days; weekly report data is kept long-term for operations analysis." },
        { h: "Sharing", p: "We do not sell or share personal information with third parties except as required by law." },
        { h: "Your rights", p: "Contact the administrator to request access, correction or deletion of your account data." },
      ],
    },
  },
  rules: {
    "zh-CN": {
      title: "社区准则",
      blocks: [
        { p: "为维护良好的交流环境，请所有用户遵守以下准则：" },
        { h: "1. 违禁内容", p: "禁止发布违法违规内容。所有内容自动经过离线违禁词库检测，命中即转入人工审核，不会立即发布。" },
        { h: "2. 违规处理", p: "经审核确认违禁：下架内容，账号按违禁程度封禁（1级=1天、2级=3天、3级=7天、4级=30天、5级=永久）。" },
        { h: "3. 误判补偿", p: "举报或审核被判定为无异常时，作者将获得 +2GB 上传额度补偿（最多 3 次）。" },
        { h: "4. 文明交流", p: "禁止人身攻击、歧视言论、恶意引战。请尊重每一位用户。" },
        { h: "5. 其他", p: "禁止广告、刷屏、灌水、发布垃圾信息。重复违规将从重处理。" },
      ],
    },
    "en-US": {
      title: "Community guidelines",
      blocks: [
        { p: "To keep CALUSE a friendly place, all users must follow these guidelines:" },
        { h: "1. Banned content", p: "Illegal content is prohibited. All content is screened against an offline banned-word library; flagged content is held for manual review." },
        { h: "2. Violations", p: "Confirmed violations are removed and accounts banned by severity (L1=1d, L2=3d, L3=7d, L4=30d, L5=permanent)." },
        { h: "3. Compensation", p: "When a report or review finds content innocent, the author receives +2GB upload quota (max 3 times)." },
        { h: "4. Be respectful", p: "No personal attacks, discrimination or deliberate provocation. Respect every user." },
        { h: "5. Miscellaneous", p: "No advertising, spamming or flooding. Repeat violations are punished more severely." },
      ],
    },
  },
};
