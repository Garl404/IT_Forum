package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/middleware"
	"github.com/Garl404/IT_Forum/backend/internal/models"
	"github.com/Garl404/IT_Forum/backend/internal/monitor"
)

func adminLog(adminID int64, action, targetType string, targetID int64, detail string) {
	database.DB.Exec(`INSERT INTO moderation_logs(admin_id, action, target_type, target_id, detail, created_at) VALUES(?,?,?,?,?,?)`,
		adminID, action, targetType, targetID, detail, time.Now().Unix())
}

// ---- 仪表盘 ----

func AdminDashboard(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	s := monitor.M.GetStats()
	var pendingPosts, pendingReports, activeBans, totalUsers, totalPosts, totalComments int
	var totalUploadBytes int64
	database.DB.QueryRow(`SELECT COUNT(*) FROM posts WHERE status='pending_review'`).Scan(&pendingPosts)
	database.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE status='pending'`).Scan(&pendingReports)
	database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE banned_until=-1 OR banned_until>?`, time.Now().Unix()).Scan(&activeBans)
	database.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	database.DB.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&totalPosts)
	database.DB.QueryRow(`SELECT COUNT(*) FROM comments`).Scan(&totalComments)
	database.DB.QueryRow(`SELECT COALESCE(SUM(size),0) FROM uploads`).Scan(&totalUploadBytes)

	// 近 7 天访问与发帖
	dayStart := time.Now().AddDate(0, 0, -6)
	dayStart = time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, dayStart.Location())
	visits7 := make([]map[string]interface{}, 0, 7)
	posts7 := make([]map[string]interface{}, 0, 7)
	for i := 0; i < 7; i++ {
		d := dayStart.AddDate(0, 0, i)
		var v, p int64
		database.DB.QueryRow(`SELECT COUNT(*) FROM visits WHERE ts>=? AND ts<?`, d.Unix(), d.AddDate(0, 0, 1).Unix()).Scan(&v)
		database.DB.QueryRow(`SELECT COUNT(*) FROM posts WHERE created_at>=? AND created_at<?`, d.Unix(), d.AddDate(0, 0, 1).Unix()).Scan(&p)
		visits7 = append(visits7, map[string]interface{}{"date": d.Format("01-02"), "count": v})
		posts7 = append(posts7, map[string]interface{}{"date": d.Format("01-02"), "count": p})
	}
	jsonOK(w, map[string]interface{}{
		"stats":             s,
		"pending_posts":     pendingPosts,
		"pending_reports":   pendingReports,
		"active_bans":       activeBans,
		"total_users":       totalUsers,
		"total_posts":       totalPosts,
		"total_comments":    totalComments,
		"total_upload_bytes": totalUploadBytes,
		"visits_7d":         visits7,
		"posts_7d":          posts7,
		"admin_name":        admin.Username,
	})
}

// ---- 审核队列 ----

// AdminReviewPosts 待审核帖子列表
func AdminReviewPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT p.id, p.user_id, u.username, p.category_id, p.title, p.content, p.status,
		p.flag_severity, p.flag_reason, p.created_at
		FROM posts p JOIN users u ON u.id=p.user_id WHERE p.status='pending_review' ORDER BY p.created_at ASC LIMIT 100`)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var id, uid, cat, sev, created int64
		var uname, title, content, status, reason string
		if rows.Scan(&id, &uid, &uname, &cat, &title, &content, &status, &sev, &reason, &created) == nil {
			list = append(list, map[string]interface{}{
				"id": id, "user_id": uid, "username": uname, "category_id": cat,
				"title": title, "content": content, "status": status,
				"flag_severity": sev, "flag_reason": reason, "created_at": created,
			})
		}
	}
	jsonOK(w, map[string]interface{}{"items": list})
}

type reviewPostReq struct {
	Action string `json:"action"` // approve | reject
	Note   string `json:"note"`
}

// AdminReviewPost 审核帖子：approve 发布并补偿；reject 下架并按严重程度封禁
func AdminReviewPost(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req reviewPostReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	var uid, sev int64
	var title string
	err = database.DB.QueryRow(`SELECT user_id, title, flag_severity FROM posts WHERE id=? AND status='pending_review'`, id).Scan(&uid, &title, &sev)
	if err != nil {
		jsonErr(w, 404, "not_found", "帖子不在审核队列")
		return
	}
	switch req.Action {
	case "approve":
		database.DB.Exec(`UPDATE posts SET status='published', flag_severity=0, flag_reason='', updated_at=? WHERE id=?`, time.Now().Unix(), id)
		grantCompensation(uid, "post_review_approved", id)
		adminLog(admin.ID, "approve_post", "post", id, "审核通过并发布："+title)
		jsonOK(w, map[string]string{"ok": "已发布，并向作者发放误判补偿"})
	case "reject":
		database.DB.Exec(`UPDATE posts SET status='removed', updated_at=? WHERE id=?`, time.Now().Unix(), id)
		sevInt := int(sev)
		if sevInt < 1 {
			sevInt = 1
		}
		banUser(uid, sevInt, "帖子审核未通过："+req.Note)
		adminLog(admin.ID, "reject_post", "post", id, fmt.Sprintf("下架并封禁(severity=%d)：%s %s", sevInt, title, req.Note))
		jsonOK(w, map[string]string{"ok": "已下架，作者账号已按违禁程度封禁"})
	default:
		jsonErr(w, 400, "bad_action", "action 必须为 approve 或 reject")
	}
}

// grantCompensation 误判补偿：增加上传额度
func grantCompensation(uid int64, reason string, targetID int64) {
	var bonusCount int
	database.DB.QueryRow(`SELECT bonus_count FROM users WHERE id=?`, uid).Scan(&bonusCount)
	if bonusCount < config.Cfg.Content.Compensation.MaxBonuses {
		database.DB.Exec(`UPDATE users SET quota_bonus_bytes=quota_bonus_bytes+?, bonus_count=bonus_count+1 WHERE id=?`,
			config.Cfg.Content.Compensation.QuotaBonusBytes, uid)
		adminLog(0, "compensation", "user", uid, fmt.Sprintf("%s 发放上传额度补偿 +%.0fGB (target=%d)",
			reason, float64(config.Cfg.Content.Compensation.QuotaBonusBytes)/(1<<30), targetID))
	}
}

func banUser(uid int64, severity int, reason string) {
	now := time.Now().Unix()
	var until int64 = -1
	durText := "永久"
	if severity < 5 {
		until = now + banDurationSec(severity)
		durText = fmt.Sprintf("%d 天", banDurationSec(severity)/86400)
	}
	database.DB.Exec(`UPDATE users SET banned_until=?, ban_reason=? WHERE id=?`, until, reason, uid)
	adminLog(0, "ban_user", "user", uid, fmt.Sprintf("封禁 %s，原因：%s", durText, reason))
}

func banDurationSec(severity int) int64 {
	switch severity {
	case 1:
		return 86400
	case 2:
		return 3 * 86400
	case 3:
		return 7 * 86400
	case 4:
		return 30 * 86400
	default:
		return -1
	}
}

// ---- 举报处理 ----

func AdminReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	sqlStr := `SELECT r.id, r.post_id, r.comment_id, r.reporter_id, u.username, r.reason, r.status, r.severity,
		r.reviewed_by, r.reviewed_at, r.note, r.created_at, COALESCE(p.title,'')
		FROM reports r LEFT JOIN users u ON u.id=r.reporter_id LEFT JOIN posts p ON p.id=r.post_id`
	if status != "" {
		sqlStr += " WHERE r.status='" + status + "'"
	}
	sqlStr += " ORDER BY r.created_at DESC LIMIT 200"
	rows, err := database.DB.Query(sqlStr)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var r models.Report
		var uname, ptitle sql.NullString
		if rows.Scan(&r.ID, &r.PostID, &r.CommentID, &r.ReporterID, &uname, &r.Reason, &r.Status, &r.Severity,
			&r.ReviewedBy, &r.ReviewedAt, &r.Note, &r.CreatedAt, &ptitle) == nil {
			list = append(list, map[string]interface{}{
				"id": r.ID, "post_id": r.PostID, "comment_id": r.CommentID, "reporter_id": r.ReporterID,
				"reporter": uname.String, "reason": r.Reason, "status": r.Status, "severity": r.Severity,
				"reviewed_by": r.ReviewedBy, "reviewed_at": r.ReviewedAt, "note": r.Note,
				"created_at": r.CreatedAt, "post_title": ptitle.String,
			})
		}
	}
	jsonOK(w, map[string]interface{}{"reports": list})
}

type reviewReportReq struct {
	Action    string `json:"action"` // approve | confirm
	Severity  int    `json:"severity"`
	Note      string `json:"note"`
	BanDays   int    `json:"ban_days"`
}

// AdminReviewReport 审核举报：approve=内容无异常（保留并补偿被举报者）；confirm=确认违禁（下架+按程度封禁）
func AdminReviewReport(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req reviewReportReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	var rep models.Report
	err = database.DB.QueryRow(`SELECT id, post_id, comment_id, reporter_id, status FROM reports WHERE id=? AND status='pending'`, id).Scan(
		&rep.ID, &rep.PostID, &rep.CommentID, &rep.ReporterID, &rep.Status)
	if err != nil {
		jsonErr(w, 404, "not_found", "举报不存在或已处理")
		return
	}
	now := time.Now().Unix()
	switch req.Action {
	case "approve":
		// 内容无异常：帖子保留，向被举报用户发放精神损失补偿（如更大上传额度）
		targetUID := postOwnerID(rep.PostID)
		if targetUID > 0 {
			grantCompensation(targetUID, "report_approved", rep.ID)
		}
		database.DB.Exec(`UPDATE reports SET status='approved', reviewed_by=?, reviewed_at=?, note=? WHERE id=?`,
			admin.ID, now, req.Note, id)
		adminLog(admin.ID, "approve_report", "report", id, "举报审核通过（内容无异常），已向被举报者发放补偿")
		jsonOK(w, map[string]string{"ok": "审核通过：帖子保留，已向作者发放精神损失补偿"})
	case "confirm":
		sev := req.Severity
		if sev < 1 {
			sev = 1
		}
		if sev > 5 {
			sev = 5
		}
		if rep.PostID > 0 {
			var uid int64
			database.DB.QueryRow(`SELECT user_id FROM posts WHERE id=?`, rep.PostID).Scan(&uid)
			database.DB.Exec(`UPDATE posts SET status='removed', flag_severity=?, updated_at=? WHERE id=?`, sev, now, rep.PostID)
			database.DB.Exec(`UPDATE comments SET status='removed' WHERE post_id=?`, rep.PostID)
			duration := banDurationSec(sev)
			if req.BanDays > 0 {
				duration = int64(req.BanDays) * 86400
			}
			var until int64 = now + duration
			if duration < 0 {
				until = -1
			}
			database.DB.Exec(`UPDATE users SET banned_until=?, ban_reason=? WHERE id=?`, until, "帖子确认违禁："+req.Note, uid)
			adminLog(admin.ID, "confirm_report", "post", rep.PostID, fmt.Sprintf("确认违禁 severity=%d，封禁用户 %d：%s", sev, uid, req.Note))
		} else if rep.CommentID > 0 {
			var uid int64
			database.DB.QueryRow(`SELECT user_id FROM comments WHERE id=?`, rep.CommentID).Scan(&uid)
			database.DB.Exec(`UPDATE comments SET status='removed' WHERE id=?`, rep.CommentID)
			if uid > 0 {
				duration := banDurationSec(sev)
				if req.BanDays > 0 {
					duration = int64(req.BanDays) * 86400
				}
				var until int64 = now + duration
				if duration < 0 {
					until = -1
				}
				database.DB.Exec(`UPDATE users SET banned_until=?, ban_reason=? WHERE id=?`, until, "评论确认违禁："+req.Note, uid)
			}
			adminLog(admin.ID, "confirm_report", "comment", rep.CommentID, fmt.Sprintf("确认违禁 severity=%d：%s", sev, req.Note))
		}
		database.DB.Exec(`UPDATE reports SET status='confirmed', severity=?, reviewed_by=?, reviewed_at=?, note=? WHERE id=?`,
			sev, admin.ID, now, req.Note, id)
		jsonOK(w, map[string]string{"ok": "已下架内容，账号已按违禁程度封禁"})
	default:
		jsonErr(w, 400, "bad_action", "action 必须为 approve 或 confirm")
	}
}

func postOwnerID(postID int64) int64 {
	if postID <= 0 {
		return 0
	}
	var uid int64
	database.DB.QueryRow(`SELECT user_id FROM posts WHERE id=?`, postID).Scan(&uid)
	return uid
}

// ---- 用户管理 ----

func AdminUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	sqlStr := `SELECT u.id, u.username, u.email, u.role, u.level, u.score, u.banned_until, u.ban_reason, u.created_at, u.last_login_at, u.last_login_ip,
		(SELECT COUNT(*) FROM posts p WHERE p.user_id=u.id),
		(SELECT COALESCE(SUM(size),0) FROM uploads f WHERE f.user_id=u.id),
		u.quota_bonus_bytes
		FROM users u`
	args := []interface{}{}
	if q != "" {
		sqlStr += " WHERE u.username LIKE ? OR u.email LIKE ?"
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	sqlStr += " ORDER BY u.id DESC LIMIT 200"
	rows, err := database.DB.Query(sqlStr, args...)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var u models.User
		var email sql.NullString
		if rows.Scan(&u.ID, &u.Username, &email, &u.Role, &u.Level, &u.Score, &u.BannedUntil, &u.BanReason,
			&u.CreatedAt, &u.LastLoginAt, &u.LastLoginIP, &u.PostCount, &u.UploadUsedBytes, &u.QuotaBonusBytes) == nil {
			list = append(list, map[string]interface{}{
				"id": u.ID, "username": u.Username, "email": email.String, "role": u.Role,
				"level": u.Level, "score": u.Score, "banned_until": u.BannedUntil, "ban_reason": u.BanReason,
				"created_at": u.CreatedAt, "last_login_at": u.LastLoginAt, "last_login_ip": u.LastLoginIP,
				"post_count": u.PostCount, "upload_used_bytes": u.UploadUsedBytes,
				"quota_bonus_bytes": u.QuotaBonusBytes,
			})
		}
	}
	jsonOK(w, map[string]interface{}{"users": list})
}

type adminUserReq struct {
	Action    string `json:"action"` // ban | unban | set_level | grant_quota
	Severity  int    `json:"severity"`
	BanDays   int    `json:"ban_days"`
	Reason    string `json:"reason"`
	Level     int    `json:"level"`
	QuotaMB   int64  `json:"quota_mb"`
}

func AdminUserAction(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req adminUserReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	var uname, role string
	if err := database.DB.QueryRow(`SELECT username, role FROM users WHERE id=?`, id).Scan(&uname, &role); err != nil {
		jsonErr(w, 404, "not_found", "用户不存在")
		return
	}
	if role == models.RoleAdmin && id == admin.ID {
		jsonErr(w, 403, "forbidden", "不能操作自己的账号")
		return
	}
	switch req.Action {
	case "ban":
		duration := banDurationSec(req.Severity)
		if req.BanDays > 0 {
			duration = int64(req.BanDays) * 86400
		}
		var until int64 = time.Now().Unix() + duration
		if duration < 0 {
			until = -1
		}
		database.DB.Exec(`UPDATE users SET banned_until=?, ban_reason=? WHERE id=?`, until, req.Reason, id)
		adminLog(admin.ID, "ban_user", "user", id, fmt.Sprintf("手动封禁 %s：%s", uname, req.Reason))
	case "unban":
		database.DB.Exec(`UPDATE users SET banned_until=0, ban_reason='' WHERE id=?`, id)
		adminLog(admin.ID, "unban_user", "user", id, "解封 "+uname)
	case "set_level":
		lv := req.Level
		if lv < 1 {
			lv = 1
		}
		if lv > 10 {
			lv = 10
		}
		database.DB.Exec(`UPDATE users SET level=? WHERE id=?`, lv, id)
		adminLog(admin.ID, "set_level", "user", id, fmt.Sprintf("调整 %s 等级为 LV%d", uname, lv))
	case "grant_quota":
		if req.QuotaMB > 0 {
			bytes := req.QuotaMB * 1024 * 1024
			database.DB.Exec(`UPDATE users SET quota_bonus_bytes=quota_bonus_bytes+? WHERE id=?`, bytes, id)
			adminLog(admin.ID, "grant_quota", "user", id, fmt.Sprintf("为 %s 增加上传额度 %d MB", uname, req.QuotaMB))
		}
	default:
		jsonErr(w, 400, "bad_action", "未知操作")
		return
	}
	jsonOK(w, map[string]string{"ok": "操作成功"})
}

// ---- 管理员邮箱 / 可信设备 ----

func AdminEmails(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, email, note, created_at FROM alert_emails ORDER BY id`)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []models.AlertEmail{}
	for rows.Next() {
		var e models.AlertEmail
		if rows.Scan(&e.ID, &e.Email, &e.Note, &e.CreatedAt) == nil {
			list = append(list, e)
		}
	}
	jsonOK(w, map[string]interface{}{"emails": list})
}

type emailReq struct {
	Email string `json:"email"`
	Note  string `json:"note"`
}

func AdminAddEmail(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	var req emailReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !emailRe.MatchString(req.Email) {
		jsonErr(w, 400, "invalid_email", "邮箱格式不正确")
		return
	}
	_, err := database.DB.Exec(`INSERT INTO alert_emails(email, note, created_at) VALUES(?,?,?)`, req.Email, req.Note, time.Now().Unix())
	if err != nil {
		jsonErr(w, 409, "exists", "该邮箱已存在")
		return
	}
	adminLog(admin.ID, "add_email", "alert_email", 0, "添加告警邮箱 "+req.Email)
	jsonCreated(w, map[string]string{"ok": "已添加"})
}

func AdminDeleteEmail(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	res, _ := database.DB.Exec(`DELETE FROM alert_emails WHERE id=?`, id)
	if n, _ := res.RowsAffected(); n == 0 {
		jsonErr(w, 404, "not_found", "不存在")
		return
	}
	adminLog(admin.ID, "del_email", "alert_email", id, "删除告警邮箱")
	jsonOK(w, map[string]string{"ok": "已删除"})
}

func AdminTrustDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT t.user_id, u.username, t.ips, t.fingerprints, t.updated_at
		FROM admin_trust t JOIN users u ON u.id=t.user_id`)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var uid, upd int64
		var uname, ipsRaw, fpRaw string
		if rows.Scan(&uid, &uname, &ipsRaw, &fpRaw, &upd) == nil {
			var ips, fps []string
			json.Unmarshal([]byte(ipsRaw), &ips)
			json.Unmarshal([]byte(fpRaw), &fps)
			list = append(list, map[string]interface{}{
				"user_id": uid, "username": uname, "ips": ips, "fingerprints": fps, "updated_at": upd,
			})
		}
	}
	jsonOK(w, map[string]interface{}{"devices": list})
}

// AdminClearTrust 清除某管理员的信任设备（强制下次 OTP）
func AdminClearTrust(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	database.DB.Exec(`UPDATE admin_trust SET ips='[]', fingerprints='[]', updated_at=? WHERE user_id=?`, time.Now().Unix(), id)
	adminLog(admin.ID, "clear_trust", "admin_trust", id, "清除管理员可信设备记录")
	jsonOK(w, map[string]string{"ok": "已清除，该管理员下次登录需邮箱验证"})
}

// ---- 盾模式控制 ----

type shieldReq struct {
	Enable bool `json:"enable"`
}

func AdminShield(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	var req shieldReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	by := "manual"
	if !req.Enable {
		by = ""
	}
	monitor.M.SetShield(req.Enable, by)
	adminLog(admin.ID, "shield_toggle", "site", 0, fmt.Sprintf("手动%s保护盾", map[bool]string{true: "开启", false: "关闭"}[req.Enable]))
	jsonOK(w, map[string]interface{}{"shielded": monitor.M.IsShielded()})
}

func AdminUnblockIP(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	var req struct {
		IP string `json:"ip"`
	}
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	monitor.M.UnblockIP(strings.TrimSpace(req.IP))
	adminLog(admin.ID, "unblock_ip", "ip", 0, "解封 IP "+req.IP)
	jsonOK(w, map[string]string{"ok": "已解封"})
}

// ---- 周报 ----

func AdminWeeklyReports(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, week_start, week_end, data, created_at FROM weekly_reports ORDER BY id DESC LIMIT 50`)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var r models.WeeklyReport
		if rows.Scan(&r.ID, &r.WeekStart, &r.WeekEnd, &r.Data, &r.CreatedAt) == nil {
			var data interface{}
			json.Unmarshal([]byte(r.Data), &data)
			list = append(list, map[string]interface{}{
				"id": r.ID, "week_start": r.WeekStart, "week_end": r.WeekEnd, "data": data, "created_at": r.CreatedAt,
			})
		}
	}
	jsonOK(w, map[string]interface{}{"reports": list})
}

// AdminGenerateWeekly 手动生成上周报告
func AdminGenerateWeekly(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	now := time.Now()
	prev := now.AddDate(0, 0, -7)
	start := time.Date(prev.Year(), prev.Month(), prev.Day(), 0, 0, 0, 0, prev.Location())
	end := start.AddDate(0, 0, 7)
	rep, err := monitor.M.GenerateWeekly(start, end)
	if err != nil {
		jsonErr(w, 500, "internal", "生成失败")
		return
	}
	data, _ := json.MarshalIndent(rep, "", "  ")
	ws := start.Format("2006-01-02")
	database.DB.Exec(`INSERT OR REPLACE INTO weekly_reports(week_start, week_end, data, created_at) VALUES(?,?,?,?)`,
		ws, end.Add(-time.Second).Format("2006-01-02"), string(data), time.Now().Unix())
	_ = os.MkdirAll(config.Cfg.Report.Dir, 0o755)
	_ = os.WriteFile(fmt.Sprintf("%s/weekly_%s.json", config.Cfg.Report.Dir, ws), data, 0o644)
	adminLog(admin.ID, "gen_weekly", "report", 0, "手动生成周报 "+ws)
	jsonOK(w, map[string]interface{}{"week": ws, "data": rep})
}

// ---- 审核日志 ----

func AdminLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT l.id, l.admin_id, u.username, l.action, l.target_type, l.target_id, l.detail, l.created_at
		FROM moderation_logs l LEFT JOIN users u ON u.id=l.admin_id ORDER BY l.id DESC LIMIT 200`)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var l models.ModerationLog
		var uname sql.NullString
		if rows.Scan(&l.ID, &l.AdminID, &uname, &l.Action, &l.TargetType, &l.TargetID, &l.Detail, &l.CreatedAt) == nil {
			list = append(list, map[string]interface{}{
				"id": l.ID, "admin_id": l.AdminID, "admin_name": uname.String, "action": l.Action,
				"target_type": l.TargetType, "target_id": l.TargetID, "detail": l.Detail, "created_at": l.CreatedAt,
			})
		}
	}
	jsonOK(w, map[string]interface{}{"logs": list})
}

// ---- 违禁词库 ----

func AdminBannedWords(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{"words": moderation_List(), "total": moderation_Count()})
}

func AdminReloadBannedWords(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	if err := moderation_Reload(config.Cfg.Content.BannedWordsFile); err != nil {
		jsonErr(w, 500, "internal", "重新加载失败："+err.Error())
		return
	}
	adminLog(admin.ID, "reload_words", "banned_words", 0, "重新加载离线违禁词库")
	jsonOK(w, map[string]string{"ok": "词库已重新加载"})
}
