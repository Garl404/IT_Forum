package auth

import (
	"fmt"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/level"
	"github.com/Garl404/IT_Forum/backend/internal/models"
)

// AuthUser 认证上下文中的用户（含统计字段）
type AuthUser struct {
	models.User
}

func ClaimsKey(uid int64) string { return fmt.Sprintf("u%d", uid) }

// LoadUser 从数据库加载用户及其统计
func LoadUser(id int64) *AuthUser {
	u := &AuthUser{}
	err := database.DB.QueryRow(`SELECT id, username, email, password_hash, role, level, score,
		quota_bonus_bytes, bonus_count, banned_until, ban_reason, created_at, last_login_at, last_login_ip
		FROM users WHERE id=?`, id).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.Level, &u.Score,
		&u.QuotaBonusBytes, &u.BonusCount, &u.BannedUntil, &u.BanReason, &u.CreatedAt, &u.LastLoginAt, &u.LastLoginIP)
	if err != nil {
		return nil
	}
	database.DB.QueryRow(`SELECT COUNT(*) FROM posts WHERE user_id=? AND status='published'`, id).Scan(&u.PostCount)
	database.DB.QueryRow(`SELECT COUNT(*) FROM comments WHERE user_id=?`, id).Scan(&u.CommentCount)
	database.DB.QueryRow(`SELECT COALESCE(SUM(size),0) FROM uploads WHERE user_id=?`, id).Scan(&u.UploadUsedBytes)
	return u
}

// UpdateLevel 重算用户等级与成长分
func UpdateLevel(uid int64) {
	u := LoadUser(uid)
	if u == nil {
		return
	}
	regDays := int64(0)
	if u.CreatedAt > 0 {
		regDays = int64(time.Since(time.Unix(u.CreatedAt, 0)).Hours() / 24)
	}
	if regDays < 0 {
		regDays = 0
	}
	var confirmed int64
	database.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE reporter_id=? AND status='confirmed'`, uid).Scan(&confirmed)
	score := level.Score(regDays, u.PostCount, u.CommentCount, confirmed)
	lv := level.FromScore(score)
	database.DB.Exec(`UPDATE users SET score=?, level=? WHERE id=?`, score, lv, uid)
}
