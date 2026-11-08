package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/auth"
	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/middleware"
	"github.com/Garl404/IT_Forum/backend/internal/models"
	"github.com/Garl404/IT_Forum/backend/internal/moderation"
)

type createPostReq struct {
	CategoryID int64  `json:"category_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

// CreatePost 发帖：违禁词自动检测，无异常直接发布，命中则进入人工审核队列
func CreatePost(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	var req createPostReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || len([]rune(req.Title)) > config.Cfg.Content.TitleMaxChars {
		jsonErr(w, 400, "invalid_title", fmt.Sprintf("标题不能为空且不超过 %d 字", config.Cfg.Content.TitleMaxChars))
		return
	}
	if req.Content == "" || len([]rune(req.Content)) > config.Cfg.Content.PostMaxChars {
		jsonErr(w, 400, "invalid_content", fmt.Sprintf("内容不能为空且不超过 %d 字", config.Cfg.Content.PostMaxChars))
		return
	}
	var catOK int
	database.DB.QueryRow(`SELECT COUNT(*) FROM categories WHERE id=?`, req.CategoryID).Scan(&catOK)
	if catOK == 0 {
		jsonErr(w, 400, "invalid_category", "分类不存在")
		return
	}

	res := moderation.Default.Check(req.Title + "\n" + req.Content)
	status := models.PostPublished
	severity := 0
	reason := ""
	if res.Severity > 0 {
		status = models.PostPendingReview
		severity = res.Severity
		reason = strings.Join(res.Matched, ", ")
	}

	now := time.Now().Unix()
	out, err := database.DB.Exec(`INSERT INTO posts(user_id, category_id, title, content, status, flag_severity, flag_reason, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		u.ID, req.CategoryID, req.Title, req.Content, status, severity, reason, now, now)
	if err != nil {
		jsonErr(w, 500, "internal", "发布失败")
		return
	}
	id, _ := out.LastInsertId()
	auth.UpdateLevel(u.ID)
	if status == models.PostPendingReview {
		jsonCreated(w, map[string]interface{}{
			"id": id, "status": status, "message": "内容触发违禁词检测，已提交人工审核，审核通过后自动发布",
		})
		return
	}
	jsonCreated(w, map[string]interface{}{"id": id, "status": status})
}

func ListPosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	category := q.Get("category")
	search := strings.TrimSpace(q.Get("q"))
	sortBy := q.Get("sort")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 20
	where := []string{"p.status='published'"}
	args := []interface{}{}
	if category != "" && category != "all" {
		where = append(where, "c.slug=?")
		args = append(args, category)
	}
	if search != "" {
		where = append(where, "(p.title LIKE ? OR p.content LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	order := "p.created_at DESC"
	switch sortBy {
	case "hot":
		order = "p.views DESC, p.created_at DESC"
	case "new":
		order = "p.created_at DESC"
	}
	var total int
	countSQL := `SELECT COUNT(*) FROM posts p JOIN categories c ON c.id=p.category_id WHERE ` + strings.Join(where, " AND ")
	database.DB.QueryRow(countSQL, args...).Scan(&total)
	sqlStr := fmt.Sprintf(`SELECT p.id, p.user_id, u.username, u.level, p.category_id, c.slug, p.title, p.content, p.status,
		p.flag_severity, p.views, p.created_at, p.updated_at,
		(SELECT COUNT(*) FROM comments cm WHERE cm.post_id=p.id AND cm.status='published')
		FROM posts p JOIN users u ON u.id=p.user_id JOIN categories c ON c.id=p.category_id
		WHERE %s ORDER BY %s LIMIT ? OFFSET ?`, strings.Join(where, " AND "), order)
	args = append(args, perPage, (page-1)*perPage)
	rows, err := database.DB.Query(sqlStr, args...)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	posts := []models.Post{}
	for rows.Next() {
		var p models.Post
		if rows.Scan(&p.ID, &p.UserID, &p.Username, &p.UserLevel, &p.CategoryID, &p.CategorySlug, &p.Title, &p.Content,
			&p.Status, &p.FlagSeverity, &p.Views, &p.CreatedAt, &p.UpdatedAt, &p.CommentCount) == nil {
			posts = append(posts, p)
		}
	}
	jsonOK(w, map[string]interface{}{"posts": posts, "total": total, "page": page, "pages": (total + perPage - 1) / perPage})
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var p models.Post
	err = database.DB.QueryRow(`SELECT p.id, p.user_id, u.username, u.level, p.category_id, c.slug, p.title, p.content, p.status,
		p.flag_severity, p.flag_reason, p.views, p.created_at, p.updated_at,
		(SELECT COUNT(*) FROM comments cm WHERE cm.post_id=p.id AND cm.status='published')
		FROM posts p JOIN users u ON u.id=p.user_id JOIN categories c ON c.id=p.category_id WHERE p.id=?`, id).Scan(
		&p.ID, &p.UserID, &p.Username, &p.UserLevel, &p.CategoryID, &p.CategorySlug, &p.Title, &p.Content, &p.Status,
		&p.FlagSeverity, &p.FlagReason, &p.Views, &p.CreatedAt, &p.UpdatedAt, &p.CommentCount)
	if err == sql.ErrNoRows {
		jsonErr(w, 404, "not_found", "帖子不存在")
		return
	}
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	u := middleware.UserFrom(r.Context())
	own := u != nil && (u.ID == p.UserID || u.Role == models.RoleAdmin)
	if p.Status != models.PostPublished && !own {
		jsonErr(w, 404, "not_found", "帖子不存在")
		return
	}
	database.DB.Exec(`UPDATE posts SET views=views+1 WHERE id=?`, id)
	p.Views++
	jsonOK(w, map[string]interface{}{"post": p})
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req createPostReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	var ownerID int64
	err = database.DB.QueryRow(`SELECT user_id FROM posts WHERE id=?`, id).Scan(&ownerID)
	if err != nil || (ownerID != u.ID && u.Role != models.RoleAdmin) {
		jsonErr(w, 403, "forbidden", "无权限操作")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || len([]rune(req.Title)) > config.Cfg.Content.TitleMaxChars ||
		req.Content == "" || len([]rune(req.Content)) > config.Cfg.Content.PostMaxChars {
		jsonErr(w, 400, "invalid", "标题或内容不符合要求")
		return
	}
	res := moderation.Default.Check(req.Title + "\n" + req.Content)
	status := models.PostPublished
	severity := 0
	reason := ""
	if res.Severity > 0 {
		status = models.PostPendingReview
		severity = res.Severity
		reason = strings.Join(res.Matched, ", ")
	}
	database.DB.Exec(`UPDATE posts SET category_id=?, title=?, content=?, status=?, flag_severity=?, flag_reason=?, updated_at=? WHERE id=?`,
		req.CategoryID, req.Title, req.Content, status, severity, reason, time.Now().Unix(), id)
	jsonOK(w, map[string]interface{}{"id": id, "status": status})
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var ownerID int64
	err = database.DB.QueryRow(`SELECT user_id FROM posts WHERE id=?`, id).Scan(&ownerID)
	if err != nil || (ownerID != u.ID && u.Role != models.RoleAdmin) {
		jsonErr(w, 403, "forbidden", "无权限操作")
		return
	}
	database.DB.Exec(`DELETE FROM comments WHERE post_id=?`, id)
	database.DB.Exec(`DELETE FROM posts WHERE id=?`, id)
	jsonOK(w, map[string]string{"ok": "已删除"})
}

// ---- 评论 ----

type createCommentReq struct {
	Content string `json:"content"`
}

func CreateComment(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req createCommentReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" || len([]rune(req.Content)) > config.Cfg.Content.CommentMaxChars {
		jsonErr(w, 400, "invalid", fmt.Sprintf("评论不能为空且不超过 %d 字", config.Cfg.Content.CommentMaxChars))
		return
	}
	var postStatus string
	if err := database.DB.QueryRow(`SELECT status FROM posts WHERE id=?`, postID).Scan(&postStatus); err != nil {
		jsonErr(w, 404, "not_found", "帖子不存在")
		return
	}
	if postStatus != models.PostPublished {
		jsonErr(w, 403, "forbidden", "帖子不可评论")
		return
	}
	res := moderation.Default.Check(req.Content)
	status := "published"
	if res.Severity > 0 {
		status = models.PostPendingReview
	}
	out, err := database.DB.Exec(`INSERT INTO comments(post_id, user_id, content, status, created_at) VALUES(?,?,?,?,?)`,
		postID, u.ID, req.Content, status, time.Now().Unix())
	if err != nil {
		jsonErr(w, 500, "internal", "评论失败")
		return
	}
	id, _ := out.LastInsertId()
	if status != "published" {
		jsonCreated(w, map[string]interface{}{"id": id, "status": status, "message": "评论触发违禁词检测，待人工审核"})
		return
	}
	auth.UpdateLevel(u.ID)
	jsonCreated(w, map[string]interface{}{"id": id, "status": status})
}

func ListComments(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	rows, err := database.DB.Query(`SELECT c.id, c.post_id, c.user_id, u.username, u.level, c.content, c.status, c.created_at
		FROM comments c JOIN users u ON u.id=c.user_id WHERE c.post_id=? AND c.status='published' ORDER BY c.created_at ASC`, postID)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	comments := []models.Comment{}
	for rows.Next() {
		var c models.Comment
		if rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &c.UserLevel, &c.Content, &c.Status, &c.CreatedAt) == nil {
			comments = append(comments, c)
		}
	}
	jsonOK(w, map[string]interface{}{"comments": comments})
}

// ---- 举报 ----

type reportReq struct {
	Reason    string `json:"reason"`
	Anonymous bool   `json:"anonymous"`
}

func ReportPost(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req reportReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		jsonErr(w, 400, "invalid", "请填写举报理由")
		return
	}
	reporterID := int64(0)
	if u := middleware.UserFrom(r.Context()); u != nil && !req.Anonymous {
		reporterID = u.ID
	}
	var exists int
	database.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE post_id=? AND status='pending'`, postID).Scan(&exists)
	if exists > 0 {
		jsonErr(w, 409, "exists", "该帖已有待处理举报")
		return
	}
	var severity int
	database.DB.QueryRow(`SELECT flag_severity FROM posts WHERE id=?`, postID).Scan(&severity)
	_, err = database.DB.Exec(`INSERT INTO reports(post_id, reporter_id, reason, status, severity, created_at) VALUES(?,?,?,?,?,?)`,
		postID, reporterID, strings.TrimSpace(req.Reason), models.ReportPending, severity, time.Now().Unix())
	if err != nil {
		jsonErr(w, 500, "internal", "举报提交失败")
		return
	}
	jsonCreated(w, map[string]string{"ok": "举报已提交，管理员将进行人工审核"})
}

func ReportComment(w http.ResponseWriter, r *http.Request) {
	commentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "bad_request", "参数错误")
		return
	}
	var req reportReq
	if err := decode(r, &req); err != nil {
		jsonErr(w, 400, "bad_request", "请求格式错误")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		jsonErr(w, 400, "invalid", "请填写举报理由")
		return
	}
	reporterID := int64(0)
	if u := middleware.UserFrom(r.Context()); u != nil && !req.Anonymous {
		reporterID = u.ID
	}
	var exists int
	database.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE comment_id=? AND status='pending'`, commentID).Scan(&exists)
	if exists > 0 {
		jsonErr(w, 409, "exists", "该评论已有待处理举报")
		return
	}
	_, err = database.DB.Exec(`INSERT INTO reports(comment_id, reporter_id, reason, status, created_at) VALUES(?,?,?,?,?)`,
		commentID, reporterID, strings.TrimSpace(req.Reason), models.ReportPending, time.Now().Unix())
	if err != nil {
		jsonErr(w, 500, "internal", "举报提交失败")
		return
	}
	jsonCreated(w, map[string]string{"ok": "举报已提交，管理员将进行人工审核"})
}

// MyPosts 用户自己的帖子（含审核中状态）
func MyPosts(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	rows, err := database.DB.Query(`SELECT p.id, p.user_id, u.username, u.level, p.category_id, c.slug, p.title, p.content, p.status,
		p.flag_severity, p.flag_reason, p.views, p.created_at, p.updated_at
		FROM posts p JOIN users u ON u.id=p.user_id JOIN categories c ON c.id=p.category_id
		WHERE p.user_id=? ORDER BY p.created_at DESC LIMIT 100`, u.ID)
	if err != nil {
		jsonErr(w, 500, "internal", "查询失败")
		return
	}
	defer rows.Close()
	posts := []models.Post{}
	for rows.Next() {
		var p models.Post
		if rows.Scan(&p.ID, &p.UserID, &p.Username, &p.UserLevel, &p.CategoryID, &p.CategorySlug, &p.Title, &p.Content,
			&p.Status, &p.FlagSeverity, &p.FlagReason, &p.Views, &p.CreatedAt, &p.UpdatedAt) == nil {
			posts = append(posts, p)
		}
	}
	jsonOK(w, map[string]interface{}{"posts": posts})
}
