package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/database"
)

const (
	StateShield   = "shield_mode"   // v: auto|manual|off
	StateShieldBy = "shield_by"     // v: who enabled
	StateShieldAt = "shield_at"     // v: unix ts
)

type Stats struct {
	RPS           int            `json:"rps"`
	RPS1Min       int            `json:"rps_1min"`
	Baseline10Min int            `json:"baseline_10min"`
	TotalToday    int64          `json:"total_today"`
	UniqueIPToday int            `json:"unique_ip_today"`
	Shielded      bool           `json:"shielded"`
	ShieldBy      string         `json:"shield_by"`
	BlockedIPs    int            `json:"blocked_ips"`
	TopIPs        []TopIP        `json:"top_ips"`
	RPSHistory    []int          `json:"rps_history"`
}

type TopIP struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

type Monitor struct {
	cfg *config.Config
	mu  sync.Mutex

	// 全局每秒计数环形缓冲（最近 120 秒）
	ring    [120]int64
	ringPos int64

	// 每 IP 每分钟计数
	ipMinute map[string]*ipCounter
	// 被封禁 IP
	blocked map[string]time.Time

	// 访问日志缓冲
	visitCh chan Visit

	shielded bool
	shieldBy string
	shieldAt time.Time
	// 自动恢复计时
	lowSince time.Time

	emailMu sync.Mutex
}

type ipCounter struct {
	minute int64 // 当前分钟（unix/60）
	count  int
	total  int64
}

type Visit struct {
	IP   string
	Path string
	UA   string
	Ts   int64
}

var M *Monitor

func New(cfg *config.Config) *Monitor {
	m := &Monitor{
		cfg:      cfg,
		ipMinute: make(map[string]*ipCounter),
		blocked:  make(map[string]time.Time),
		visitCh:  make(chan Visit, 65536),
	}
	// 恢复持久化的盾状态
	var v string
	if err := database.DB.QueryRow(`SELECT v FROM site_state WHERE k=?`, StateShield).Scan(&v); err == nil {
		if v == "auto" || v == "manual" {
			m.shielded = true
			m.shieldBy = v
		}
	}
	M = m
	return m
}

func (m *Monitor) Start() {
	go m.evaluateLoop()
	go m.visitFlusher()
	go m.weeklyLoop()
	go m.cleanupLoop()
}

// Record 记录一次请求（由中间件调用）
func (m *Monitor) Record(ip, path, ua string) {
	now := time.Now()
	sec := now.Unix()
	m.mu.Lock()
	pos := sec % 120
	m.ring[pos]++ // 近似计数（环形覆盖旧数据）
	m.ringPos = sec
	min := sec / 60
	c := m.ipMinute[ip]
	if c == nil {
		c = &ipCounter{minute: min}
		m.ipMinute[ip] = c
	}
	if c.minute != min {
		c.minute = min
		c.count = 0
	}
	c.count++
	c.total++
	m.mu.Unlock()
	select {
	case m.visitCh <- Visit{IP: ip, Path: path, UA: ua, Ts: sec}:
	default:
	}
}

func (m *Monitor) IsShielded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shielded
}

func (m *Monitor) ShieldBy() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shieldBy
}

func (m *Monitor) IsIPBlocked(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.blocked[ip]; ok {
		if time.Now().Before(t) {
			return true
		}
		delete(m.blocked, ip)
	}
	return false
}

func (m *Monitor) BlockIP(ip string, minutes int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocked[ip] = time.Now().Add(time.Duration(minutes) * time.Minute)
}

func (m *Monitor) UnblockIP(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blocked, ip)
}

// SetShield 手动/自动开关盾（自动关站）
func (m *Monitor) SetShield(on bool, by string) {
	m.mu.Lock()
	changed := m.shielded != on
	m.shielded = on
	if on {
		m.shieldBy = by
		m.shieldAt = time.Now()
		_, _ = database.DB.Exec(`INSERT INTO site_state(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, StateShield, by)
		_, _ = database.DB.Exec(`INSERT INTO site_state(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, StateShieldBy, by)
		_, _ = database.DB.Exec(`INSERT INTO site_state(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, StateShieldAt, fmt.Sprint(time.Now().Unix()))
	} else {
		m.shieldBy = ""
		_, _ = database.DB.Exec(`INSERT INTO site_state(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, StateShield, "off")
	}
	m.mu.Unlock()
	if changed {
		go m.notifyShield(on, by)
	}
}

// GetStats 汇总实时统计（供管理后台）
func (m *Monitor) GetStats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	var last1, last60 int64
	for i := int64(0); i < 60; i++ {
		v := m.ring[(now-i+240)%120]
		if i < 5 {
			last1 += v
		}
		last60 += v
	}
	// 10 分钟基线
	var base10 int64
	for i := int64(0); i < 600; i++ {
		base10 += m.ring[(now-i+1200)%120]
	}
	type kv struct {
		ip  string
		cnt int
	}
	var list []kv
	min := now / 60
	for ip, c := range m.ipMinute {
		cnt := c.count
		if c.minute != min {
			cnt = 0
		}
		if cnt > 0 {
			list = append(list, kv{ip, cnt})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].cnt > list[j].cnt })
	if len(list) > 10 {
		list = list[:10]
	}
	top := make([]TopIP, 0, len(list))
	for _, k := range list {
		top = append(top, TopIP{IP: k.ip, Count: k.cnt})
	}
	hist := make([]int, 0, 60)
	for i := int64(59); i >= 0; i-- {
		hist = append(hist, int(m.ring[(now-i+240)%120]))
	}
	var totalToday int64
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM visits WHERE ts >= ?`, todayStart()).Scan(&totalToday)
	var uniq int
	_ = database.DB.QueryRow(`SELECT COUNT(DISTINCT ip) FROM visits WHERE ts >= ?`, todayStart()).Scan(&uniq)
	return Stats{
		RPS:           int(last1 / 5),
		RPS1Min:       int(last60 / 60),
		Baseline10Min: int(base10 / 600),
		TotalToday:    totalToday,
		UniqueIPToday: uniq,
		Shielded:      m.shielded,
		ShieldBy:      m.shieldBy,
		BlockedIPs:    len(m.blocked),
		TopIPs:        top,
		RPSHistory:    hist,
	}
}

// evaluateLoop DDoS 检测主循环
func (m *Monitor) evaluateLoop() {
	interval := time.Duration(m.cfg.DDOS.CheckIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		m.evaluate()
	}
}

func (m *Monitor) evaluate() {
	if !m.cfg.DDOS.Enabled {
		return
	}
	m.mu.Lock()
	now := time.Now().Unix()
	var last10 int64
	for i := int64(0); i < 10; i++ {
		last10 += m.ring[(now-i+240)%120]
	}
	// 1 秒瞬时峰值（当前秒 + 上一秒）
	peak := m.ring[(now+240)%120] + m.ring[(now-1+240)%120]
	var base int64
	for i := int64(0); i < 600; i++ {
		base += m.ring[(now-i+1200)%120]
	}
	rps := float64(last10) / 10.0
	baseRPS := float64(base) / 600.0
	peakRPS := float64(peak)

	// 单 IP 高频检测
	min := now / 60
	blockedAny := false
	for ip, c := range m.ipMinute {
		cnt := c.count
		if c.minute != min {
			cnt = 0
		}
		if cnt > m.cfg.DDOS.PerIPThresholdPerMin && !m.isWhitelisted(ip) {
			m.blocked[ip] = time.Now().Add(time.Duration(m.cfg.DDOS.IPBlockMinutes) * time.Minute)
			blockedAny = true
			log.Printf("[ddos] 封禁高频 IP: %s (%d req/min)", ip, cnt)
		}
	}
	m.mu.Unlock()
	if blockedAny {
		go m.notifyIPBlocked()
	}

	trigger := float64(m.cfg.DDOS.TriggerRPS)
	anomaly := rps > trigger && (baseRPS < 1 || rps > baseRPS*m.cfg.DDOS.SpikeFactor)
	// 突刺检测：1 秒峰值显著超过阈值且远超基线
	if !anomaly && peakRPS > trigger*2 && (baseRPS < 1 || peakRPS > baseRPS*m.cfg.DDOS.SpikeFactor*2) {
		anomaly = true
	}
	if anomaly {
		if !m.IsShielded() {
			log.Printf("[ddos] 检测到异常流量: rps=%.1f peak=%.0f 基线=%.1f，自动关闭网站", rps, peakRPS, baseRPS)
			m.SetShield(true, "auto")
		}
		m.resetLowSince()
		return
	}
	if m.IsShielded() && m.ShieldBy() == "auto" {
		if rps < float64(m.cfg.DDOS.RecoverThresholdRPS) {
			m.mu.Lock()
			if m.lowSince.IsZero() {
				m.lowSince = time.Now()
			}
			elapsed := time.Since(m.lowSince)
			m.mu.Unlock()
			if elapsed >= time.Duration(m.cfg.DDOS.AutoRecoverMin)*time.Minute {
				log.Printf("[ddos] 流量恢复正常 (rps=%.1f)，自动恢复网站", rps)
				m.SetShield(false, "")
				m.mu.Lock()
				m.lowSince = time.Time{}
				m.mu.Unlock()
			}
		} else {
			m.resetLowSince()
		}
	}
}

func (m *Monitor) resetLowSince() {
	m.mu.Lock()
	m.lowSince = time.Time{}
	m.mu.Unlock()
}

func (m *Monitor) isWhitelisted(ip string) bool {
	for _, w := range m.cfg.DDOS.WhitelistIPs {
		if w == ip {
			return true
		}
	}
	// 管理员登记的可信 IP 也放行
	rows, err := database.DB.Query(`SELECT ips FROM admin_trust`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		var list []string
		if json.Unmarshal([]byte(raw), &list) == nil {
			for _, w := range list {
				if w == ip {
					return true
				}
			}
		}
	}
	return false
}

// WhitelistedIP 供中间件检查 IP 是否在白名单
func WhitelistedIP(ip string) bool {
	if M == nil {
		return false
	}
	return M.isWhitelisted(ip)
}

// notifyShield 邮件通知管理员盾状态变化
func (m *Monitor) notifyShield(on bool, by string) {
	m.emailMu.Lock()
	defer m.emailMu.Unlock()
	emails := getAlertEmails()
	if len(emails) == 0 {
		return
	}
	s := m.GetStats()
	state := "已关闭（恢复正常）"
	if on {
		state = "已自动关闭"
	}
	body := fmt.Sprintf(`<div style="font-family:sans-serif">
<h2>CALUSE 安全告警</h2>
<p>网站保护盾状态：<strong style="color:%s">%s</strong></p>
<ul>
<li>触发方式：%s</li>
<li>当前 RPS：%d</li>
<li>10 分钟基线 RPS：%d</li>
<li>今日访问总量：%d</li>
<li>今日独立 IP：%d</li>
</ul>
<h4>访问量最高的 IP：</h4>
<table border="1" cellpadding="6" cellspacing="0">
<tr><th>IP</th><th>请求数/分钟</th></tr>`,
		func() string { if on { return "#dc2626" }; return "#16a34a" }(), state, by, s.RPS, s.Baseline10Min, s.TotalToday, s.UniqueIPToday)
	for _, t := range s.TopIPs {
		body += fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", t.IP, t.Count)
	}
	body += "</table><p>时间：" + time.Now().Format("2006-01-02 15:04:05 MST") + "</p></div>"
	_ = SendEmail(m.cfg, emails, "【CALUSE】网站保护盾"+state, body)
}

func (m *Monitor) notifyIPBlocked() {
	m.emailMu.Lock()
	defer m.emailMu.Unlock()
	emails := getAlertEmails()
	if len(emails) == 0 {
		return
	}
	body := "<h2>CALUSE 安全告警</h2><p>检测到单个 IP 请求频率异常，已临时封禁相关 IP（" + fmt.Sprint(m.cfg.DDOS.IPBlockMinutes) + " 分钟）。</p>"
	_ = SendEmail(m.cfg, emails, "【CALUSE】高频 IP 已自动封禁", body)
}

func getAlertEmails() []string {
	rows, err := database.DB.Query(`SELECT email FROM alert_emails`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if rows.Scan(&e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func todayStart() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

// visitFlusher 批量写入访问日志
func (m *Monitor) visitFlusher() {
	buf := make([]Visit, 0, 256)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case v := <-m.visitCh:
			buf = append(buf, v)
			if len(buf) >= 256 {
				m.flushVisits(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				m.flushVisits(buf)
				buf = buf[:0]
			}
		}
	}
}

func (m *Monitor) flushVisits(buf []Visit) {
	tx, err := database.DB.Begin()
	if err != nil {
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO visits(ip, path, ua, ts) VALUES(?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return
	}
	defer stmt.Close()
	for _, v := range buf {
		if len(v.UA) > 400 {
			v.UA = v.UA[:400]
		}
		if len(v.Path) > 400 {
			v.Path = v.Path[:400]
		}
		_, _ = stmt.Exec(v.IP, v.Path, v.UA, v.Ts)
	}
	tx.Commit()
}

func (m *Monitor) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for ip, t := range m.blocked {
			if now.After(t) {
				delete(m.blocked, ip)
			}
		}
		cutoff := now.Unix()/60 - 5
		for ip, c := range m.ipMinute {
			if c.minute < cutoff {
				delete(m.ipMinute, ip)
			}
		}
		m.mu.Unlock()
		// 访问日志保留策略：删除 90 天前的明细
		_, _ = database.DB.Exec(`DELETE FROM visits WHERE ts < ?`, time.Now().Add(-90*24*time.Hour).Unix())
	}
}

// ---- 每周数据报告 ----

// weeklyLoop 每周一（ISO）自动生成上一周网站数据报告
func (m *Monitor) weeklyLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		m.maybeGenerateWeekly()
	}
}

func (m *Monitor) maybeGenerateWeekly() {
	now := time.Now()
	if int(now.Weekday()) == 0 {
		now = now.AddDate(0, 0, -1) // 周日早上也补生成
	}
	if int(now.Weekday()) != m.cfg.Report.WeeklyISODay {
		return
	}
	prev := now.AddDate(0, 0, -7)
	start := time.Date(prev.Year(), prev.Month(), prev.Day(), 0, 0, 0, 0, prev.Location())
	end := start.AddDate(0, 0, 7)
	ws := start.Format("2006-01-02")
	var exists int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM weekly_reports WHERE week_start=?`, ws).Scan(&exists); err == nil && exists > 0 {
		return
	}
	rep, err := m.GenerateWeekly(start, end)
	if err != nil {
		log.Printf("[report] 生成周报失败: %v", err)
		return
	}
	data, _ := json.MarshalIndent(rep, "", "  ")
	if _, err := database.DB.Exec(`INSERT OR IGNORE INTO weekly_reports(week_start, week_end, data, created_at) VALUES(?,?,?,?)`,
		ws, end.Add(-time.Second).Format("2006-01-02"), string(data), time.Now().Unix()); err != nil {
		log.Printf("[report] 周报入库失败: %v", err)
		return
	}
	_ = os.MkdirAll(m.cfg.Report.Dir, 0o755)
	fname := fmt.Sprintf("%s/weekly_%s.json", m.cfg.Report.Dir, ws)
	_ = os.WriteFile(fname, data, 0o644)
	log.Printf("[report] 已生成周报 %s → %s", ws, fname)
	m.emailWeekly(ws, rep)
}

type WeeklyData struct {
	WeekStart     string            `json:"week_start"`
	WeekEnd       string            `json:"week_end"`
	Visits        int64             `json:"visits"`
	UniqueIPs     int               `json:"unique_ips"`
	NewUsers      int               `json:"new_users"`
	NewPosts      int               `json:"new_posts"`
	NewComments   int               `json:"new_comments"`
	Uploads       int               `json:"uploads"`
	UploadBytes   int64             `json:"upload_bytes"`
	Reports       int               `json:"reports"`
	ReportsDone   int               `json:"reports_resolved"`
	Bans          int               `json:"bans"`
	ActiveUsers   int               `json:"active_users"`
	ShieldEvents  int               `json:"shield_events"`
	TopIPs        []TopIP           `json:"top_ips"`
	DailyVisits   map[string]int64  `json:"daily_visits"`
	DailyPosts    map[string]int64  `json:"daily_posts"`
	CategoryPosts map[string]int64  `json:"category_posts"`
}

func (m *Monitor) GenerateWeekly(start, end time.Time) (*WeeklyData, error) {
	s, e := start.Unix(), end.Unix()
	rep := &WeeklyData{
		WeekStart:     start.Format("2006-01-02"),
		WeekEnd:       end.Add(-time.Second).Format("2006-01-02"),
		DailyVisits:   map[string]int64{},
		DailyPosts:    map[string]int64{},
		CategoryPosts: map[string]int64{},
	}
	q := func(dest interface{}, sql string, args ...interface{}) {
		if err := database.DB.QueryRow(sql, args...).Scan(dest); err != nil {
			log.Printf("[report] query error: %v", err)
		}
	}
	q(&rep.Visits, `SELECT COUNT(*) FROM visits WHERE ts>=? AND ts<?`, s, e)
	q(&rep.UniqueIPs, `SELECT COUNT(DISTINCT ip) FROM visits WHERE ts>=? AND ts<?`, s, e)
	q(&rep.NewUsers, `SELECT COUNT(*) FROM users WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.NewPosts, `SELECT COUNT(*) FROM posts WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.NewComments, `SELECT COUNT(*) FROM comments WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.Uploads, `SELECT COUNT(*) FROM uploads WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.UploadBytes, `SELECT COALESCE(SUM(size),0) FROM uploads WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.Reports, `SELECT COUNT(*) FROM reports WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.ReportsDone, `SELECT COUNT(*) FROM reports WHERE reviewed_at>=? AND reviewed_at<?`, s, e)
	q(&rep.Bans, `SELECT COUNT(*) FROM moderation_logs WHERE action IN ('ban_user','confirm_report') AND created_at>=? AND created_at<?`, s, e)
	q(&rep.ActiveUsers, `SELECT COUNT(DISTINCT user_id) FROM posts WHERE created_at>=? AND created_at<?`, s, e)
	q(&rep.ShieldEvents, `SELECT COUNT(*) FROM moderation_logs WHERE action LIKE '%shield%' AND created_at>=? AND created_at<?`, s, e)

	rows, err := database.DB.Query(`SELECT ip, COUNT(*) c FROM visits WHERE ts>=? AND ts<? GROUP BY ip ORDER BY c DESC LIMIT 10`, s, e)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t TopIP
			if rows.Scan(&t.IP, &t.Count) == nil {
				rep.TopIPs = append(rep.TopIPs, t)
			}
		}
	}
	rows, err = database.DB.Query(`SELECT (ts/86400)*86400 d, COUNT(*) c FROM visits WHERE ts>=? AND ts<? GROUP BY d`, s, e)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d, c int64
			if rows.Scan(&d, &c) == nil {
				rep.DailyVisits[time.Unix(d, 0).Format("2006-01-02")] = c
			}
		}
	}
	rows, err = database.DB.Query(`SELECT (created_at/86400)*86400 d, COUNT(*) c FROM posts WHERE created_at>=? AND created_at<? GROUP BY d`, s, e)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d, c int64
			if rows.Scan(&d, &c) == nil {
				rep.DailyPosts[time.Unix(d, 0).Format("2006-01-02")] = c
			}
		}
	}
	rows, err = database.DB.Query(`SELECT c.name_key, COUNT(*) n FROM posts p JOIN categories c ON c.id=p.category_id WHERE p.created_at>=? AND p.created_at<? GROUP BY p.category_id`, s, e)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var n int64
			if rows.Scan(&name, &n) == nil {
				rep.CategoryPosts[name] = n
			}
		}
	}
	return rep, nil
}

func (m *Monitor) emailWeekly(ws string, rep *WeeklyData) {
	m.emailMu.Lock()
	defer m.emailMu.Unlock()
	emails := getAlertEmails()
	if len(emails) == 0 {
		return
	}
	var topRows string
	for _, t := range rep.TopIPs {
		topRows += fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", t.IP, t.Count)
	}
	body := fmt.Sprintf(`<div style="font-family:sans-serif">
<h2>CALUSE 周度数据报告（%s ~ %s）</h2>
<table border="1" cellpadding="6" cellspacing="0">
<tr><th>指标</th><th>数值</th></tr>
<tr><td>总访问量</td><td>%d</td></tr>
<tr><td>独立访客 IP</td><td>%d</td></tr>
<tr><td>新增用户</td><td>%d</td></tr>
<tr><td>新发帖</td><td>%d</td></tr>
<tr><td>新评论</td><td>%d</td></tr>
<tr><td>文件上传数</td><td>%d</td></tr>
<tr><td>上传总字节</td><td>%d (%.2f GB)</td></tr>
<tr><td>举报数 / 已处理</td><td>%d / %d</td></tr>
<tr><td>封禁操作</td><td>%d</td></tr>
<tr><td>活跃发帖用户</td><td>%d</td></tr>
<tr><td>盾事件</td><td>%d</td></tr>
</table>
<h4>Top 10 访问 IP</h4>
<table border="1" cellpadding="6" cellspacing="0"><tr><th>IP</th><th>请求数</th></tr>%s</table>
<p>完整数据请登录管理后台查看。CALUSE 安全团队</p></div>`,
		rep.WeekStart, rep.WeekEnd, rep.Visits, rep.UniqueIPs, rep.NewUsers, rep.NewPosts,
		rep.NewComments, rep.Uploads, rep.UploadBytes, float64(rep.UploadBytes)/(1<<30),
		rep.Reports, rep.ReportsDone, rep.Bans, rep.ActiveUsers, rep.ShieldEvents, topRows)
	_ = SendEmail(m.cfg, emails, fmt.Sprintf("【CALUSE】周度数据报告 %s", ws), body)
}
