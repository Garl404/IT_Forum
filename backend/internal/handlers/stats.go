package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/models"
	"github.com/Garl404/IT_Forum/backend/internal/version"
)

// Categories 公开分类列表
func Categories(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, slug, name_key, sort FROM categories ORDER BY sort`)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	list := []models.Category{}
	for rows.Next() {
		var c models.Category
		if rows.Scan(&c.ID, &c.Slug, &c.NameKey, &c.Sort) == nil {
			list = append(list, c)
		}
	}
	jsonOK(w, map[string]interface{}{"categories": list})
}

// SiteStats 公开站点统计
func SiteStats(w http.ResponseWriter, r *http.Request) {
	dayStart := time.Now().AddDate(0, 0, -6)
	dayStart = time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, dayStart.Location())
	var totalUsers, totalPosts, totalComments int
	var todayVisits int64
	database.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	database.DB.QueryRow(`SELECT COUNT(*) FROM posts WHERE status='published'`).Scan(&totalPosts)
	database.DB.QueryRow(`SELECT COUNT(*) FROM comments WHERE status='published'`).Scan(&totalComments)
	database.DB.QueryRow(`SELECT COUNT(*) FROM visits WHERE ts>=?`, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location()).Unix()).Scan(&todayVisits)

	var online int
	database.DB.QueryRow(`SELECT COUNT(DISTINCT ip) FROM visits WHERE ts>=?`, time.Now().Add(-15*time.Minute).Unix()).Scan(&online)

	visits7 := []map[string]interface{}{}
	for i := 0; i < 7; i++ {
		d := dayStart.AddDate(0, 0, i)
		var v int64
		database.DB.QueryRow(`SELECT COUNT(*) FROM visits WHERE ts>=? AND ts<?`, d.Unix(), d.AddDate(0, 0, 1).Unix()).Scan(&v)
		visits7 = append(visits7, map[string]interface{}{"date": d.Format("01-02"), "count": v})
	}
	jsonOK(w, map[string]interface{}{
		"total_users":   totalUsers,
		"total_posts":   totalPosts,
		"total_comments": totalComments,
		"today_visits":  todayVisits,
		"online_now":    online,
		"visits_7d":     visits7,
	})
}

// UserProfile 公开用户信息
func UserProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var u models.PublicUser
	err = database.DB.QueryRow(`SELECT id, username, role, level, created_at FROM users WHERE id=?`, id).Scan(
		&u.ID, &u.Username, &u.Role, &u.Level, &u.CreatedAt)
	if err == sql.ErrNoRows {
		jsonErr(w, 404, "not_found", "用户不存在")
		return
	}
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	database.DB.QueryRow(`SELECT COUNT(*) FROM posts WHERE user_id=? AND status='published'`, id).Scan(&u.PostCount)
	jsonOK(w, map[string]interface{}{"user": u})
}

func Health(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{"ok": true, "name": "CALUSE", "version": version.Version, "time": time.Now().Unix()})
}
