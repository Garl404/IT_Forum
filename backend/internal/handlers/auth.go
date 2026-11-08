package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/auth"
	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/middleware"
	"github.com/Garl404/IT_Forum/backend/internal/models"
	"github.com/Garl404/IT_Forum/backend/internal/monitor"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_\-\x{4e00}-\x{9fa5}]{2,24}$`)
	emailRe    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !usernameRe.MatchString(req.Username) {
		jsonErr(w, 400, "invalid_username", "用户名需为 2-24 位字母、数字、下划线、连字符或中文")
		return
	}
	if req.Email == "" || !emailRe.MatchString(req.Email) {
		jsonErr(w, 400, "invalid_email", "邮箱格式不正确")
		return
	}
	if len(req.Password) < 8 || !regexp.MustCompile(`[a-zA-Z]`).MatchString(req.Password) || !regexp.MustCompile(`[0-9]`).MatchString(req.Password) {
		jsonErr(w, 400, "weak_password", "密码至少 8 位，且同时包含字母和数字")
		return
	}
	var exists int
	database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE username=? OR email=?`, req.Username, req.Email).Scan(&exists)
	if exists > 0 {
		jsonErr(w, 409, "exists", "用户名或邮箱已被注册")
		return
	}
	hash, err := auth.HashPassword(req.Password, config.Cfg)
	if err != nil {
		jsonErr(w, 500, "internal", "服务器错误")
		return
	}
	now := time.Now().Unix()
	res, err := database.DB.Exec(`INSERT INTO users(username, email, password_hash, role, level, created_at) VALUES(?,?,?,?,?,?)`,
		req.Username, req.Email, hash, models.RoleUser, 1, now)
	if err != nil {
		jsonErr(w, 500, "internal", "注册失败")
		return
	}
	id, _ := res.LastInsertId()
	token, _ := auth.IssueToken(config.Cfg, id, req.Username, models.RoleUser, 1)
	jsonCreated(w, map[string]interface{}{
		"token": token,
		"user":  userView(auth.LoadUser(id)),
	})
}

type loginReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Fingerprint string `json:"fingerprint"`
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	var u models.User
	err := database.DB.QueryRow(`SELECT id, username, email, password_hash, role, level, banned_until, ban_reason, created_at
		FROM users WHERE username=? OR email=?`, req.Username, strings.ToLower(req.Username)).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.Level, &u.BannedUntil, &u.BanReason, &u.CreatedAt)
	if err == sql.ErrNoRows || (err == nil && !auth.VerifyPassword(u.PasswordHash, req.Password)) {
		jsonErr(w, 401, "bad_credentials", "用户名或密码错误")
		return
	}
	if err != nil && err != sql.ErrNoRows {
		jsonErr(w, 500, "internal", "服务器错误")
		return
	}
	now := time.Now().Unix()
	if u.BannedUntil == -1 || u.BannedUntil > now {
		msg := "账号已被封禁"
		if u.BannedUntil > now {
			msg += "，解封时间 " + time.Unix(u.BannedUntil, 0).Format("2006-01-02 15:04")
		}
		jsonErr(w, 403, "banned", msg)
		return
	}
	ip := middleware.IPFrom(r.Context())
	database.DB.Exec(`UPDATE users SET last_login_at=?, last_login_ip=? WHERE id=?`, now, ip, u.ID)

	// 管理员严格登录：IP/指纹未登记 → 邮箱验证码
	if u.Role == models.RoleAdmin {
		fp := auth.Fingerprint(req.Fingerprint, config.Cfg.Security.JWTSecret)
		if !adminTrusted(u.ID, ip, fp) {
			if config.Cfg.Security.AdminRequireOTPNewDev && u.Email != "" {
				code, err := auth.OTP.Generate(u.Email)
				if err == nil {
					monitor.SendEmail(config.Cfg, []string{u.Email}, "【CALUSE】管理员登录验证码",
						"<p>检测到管理员账号 <b>"+u.Username+"</b> 在新设备/IP（"+ip+"）登录。</p><p>验证码：<b style='font-size:20px'>"+code+"</b>（10 分钟内有效）</p><p>如非本人操作，请立即修改密码。</p>")
				}
				jsonOK(w, map[string]interface{}{
					"need_otp": true, "email": maskEmail(u.Email), "pending_id": u.ID,
				})
				return
			}
			log.Printf("[security] 管理员 %s 从新 IP %s 登录（无邮箱绑定，放行并记录）", u.Username, ip)
		}
		trustAdmin(u.ID, ip, fp)
	}
	auth.UpdateLevel(u.ID)
	token, _ := auth.IssueToken(config.Cfg, u.ID, u.Username, u.Role, u.Level)
	jsonOK(w, map[string]interface{}{"token": token, "user": userView(auth.LoadUser(u.ID))})
}

type otpReq struct {
	PendingID   int64  `json:"pending_id"`
	Code        string `json:"code"`
	Fingerprint string `json:"fingerprint"`
}

func VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req otpReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	var u models.User
	err := database.DB.QueryRow(`SELECT id, username, email, role, level FROM users WHERE id=? AND role='admin'`, req.PendingID).Scan(
		&u.ID, &u.Username, &u.Email, &u.Role, &u.Level)
	if err != nil {
		jsonErr(w, 401, "unauthorized", "登录状态已失效，请重新登录")
		return
	}
	if !auth.OTP.Verify(u.Email, req.Code) {
		jsonErr(w, 401, "bad_code", "验证码错误或已过期")
		return
	}
	ip := middleware.IPFrom(r.Context())
	fp := auth.Fingerprint(req.Fingerprint, config.Cfg.Security.JWTSecret)
	trustAdmin(u.ID, ip, fp)
	log.Printf("[security] 管理员 %s 通过邮箱验证码登录（IP %s）", u.Username, ip)
	token, _ := auth.IssueToken(config.Cfg, u.ID, u.Username, u.Role, u.Level)
	jsonOK(w, map[string]interface{}{"token": token, "user": userView(auth.LoadUser(u.ID))})
}

func adminTrusted(uid int64, ip, fp string) bool {
	var ipsRaw, fpsRaw string
	err := database.DB.QueryRow(`SELECT ips, fingerprints FROM admin_trust WHERE user_id=?`, uid).Scan(&ipsRaw, &fpsRaw)
	if err != nil {
		return false
	}
	var ips, fps []string
	json.Unmarshal([]byte(ipsRaw), &ips)
	json.Unmarshal([]byte(fpsRaw), &fps)
	if len(ips) == 0 && len(fps) == 0 {
		return false
	}
	ipOK := false
	for _, v := range ips {
		if v == ip {
			ipOK = true
			break
		}
	}
	fpOK := fp != "" && len(fps) > 0
	if fpOK {
		fpOK = false
		for _, v := range fps {
			if v == fp {
				fpOK = true
				break
			}
		}
	}
	return ipOK && (fp == "" || fpOK)
}

func trustAdmin(uid int64, ip, fp string) {
	var ipsRaw, fpsRaw string
	_ = database.DB.QueryRow(`SELECT ips, fingerprints FROM admin_trust WHERE user_id=?`, uid).Scan(&ipsRaw, &fpsRaw)
	var ips, fps []string
	json.Unmarshal([]byte(ipsRaw), &ips)
	json.Unmarshal([]byte(fpsRaw), &fps)
	if ip != "" && !contains(ips, ip) {
		ips = append(ips, ip)
		if len(ips) > 20 {
			ips = ips[len(ips)-20:]
		}
	}
	if fp != "" && !contains(fps, fp) {
		fps = append(fps, fp)
		if len(fps) > 10 {
			fps = fps[len(fps)-10:]
		}
	}
	ni, _ := json.Marshal(ips)
	nf, _ := json.Marshal(fps)
	database.DB.Exec(`INSERT INTO admin_trust(user_id, ips, fingerprints, updated_at) VALUES(?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET ips=excluded.ips, fingerprints=excluded.fingerprints, updated_at=excluded.updated_at`,
		uid, string(ni), string(nf), time.Now().Unix())
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func maskEmail(e string) string {
	at := strings.Index(e, "@")
	if at <= 1 {
		return "***@" + e[at+1:]
	}
	return e[:2] + "***" + e[at:]
}

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	var req changePwdReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	if !auth.VerifyPassword(u.PasswordHash, req.OldPassword) {
		jsonErr(w, 401, "bad_credentials", "原密码错误")
		return
	}
	if len(req.NewPassword) < 8 || !regexp.MustCompile(`[a-zA-Z]`).MatchString(req.NewPassword) || !regexp.MustCompile(`[0-9]`).MatchString(req.NewPassword) {
		jsonErr(w, 400, "weak_password", "新密码至少 8 位，且同时包含字母和数字")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword, config.Cfg)
	if err != nil {
		jsonErr(w, 500, "internal", "服务器错误")
		return
	}
	database.DB.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, u.ID)
	jsonOK(w, map[string]string{"ok": "密码已修改"})
}

func Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	auth.UpdateLevel(u.ID)
	jsonOK(w, map[string]interface{}{"user": userView(auth.LoadUser(u.ID))})
}

func userView(u *auth.AuthUser) map[string]interface{} {
	if u == nil {
		return nil
	}
	return map[string]interface{}{
		"id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role,
		"level": u.Level, "score": u.Score, "created_at": u.CreatedAt,
		"banned_until": u.BannedUntil, "ban_reason": u.BanReason,
		"post_count": u.PostCount, "comment_count": u.CommentCount,
		"upload_used_bytes": u.UploadUsedBytes,
		"upload_quota":      uploadQuota(u),
		"quota_bonus_bytes": u.QuotaBonusBytes, "bonus_count": u.BonusCount,
	}
}

// uploadQuota 计算用户上传配额（基础 5GB；10 级 15GB；含误判补偿）
func uploadQuota(u *auth.AuthUser) int64 {
	base := config.Cfg.Content.Upload.BaseQuotaBytes
	if u.Level >= 10 {
		base = config.Cfg.Content.Upload.Level10QuotaBytes
	}
	return base + u.QuotaBonusBytes
}
