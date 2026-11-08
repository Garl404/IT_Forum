package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/handlers"
	"github.com/Garl404/IT_Forum/backend/internal/middleware"
)

func New(cfg *config.Config) http.Handler {
	middleware.Setup(cfg)
	mux := http.NewServeMux()

	// 公开 API
	mux.HandleFunc("GET /api/health", handlers.Health)
	mux.HandleFunc("GET /api/categories", handlers.Categories)
	mux.HandleFunc("GET /api/stats", handlers.SiteStats)
	mux.HandleFunc("GET /api/posts", handlers.ListPosts)
	mux.HandleFunc("GET /api/posts/{id}", handlers.GetPost)
	mux.HandleFunc("GET /api/posts/{id}/comments", handlers.ListComments)
	mux.HandleFunc("GET /api/user/{id}", handlers.UserProfile)

	// 认证
	mux.HandleFunc("POST /api/auth/register", handlers.Register)
	mux.HandleFunc("POST /api/auth/login", handlers.Login)
	mux.HandleFunc("POST /api/auth/verify-otp", handlers.VerifyOTP)
	mux.Handle("GET /api/auth/me", middleware.RequireAuth(http.HandlerFunc(handlers.Me)))
	mux.Handle("POST /api/auth/change-password", middleware.RequireAuth(middleware.BanCheck(http.HandlerFunc(handlers.ChangePassword))))

	// 用户写操作（需登录 + 封禁检查 + 用户限流）
	mux.Handle("POST /api/posts", middleware.RequireAuth(middleware.BanCheck(middleware.UserRateLimit("post")(http.HandlerFunc(handlers.CreatePost)))))
	mux.Handle("PUT /api/posts/{id}", middleware.RequireAuth(middleware.BanCheck(middleware.UserRateLimit("post")(http.HandlerFunc(handlers.UpdatePost)))))
	mux.Handle("DELETE /api/posts/{id}", middleware.RequireAuth(middleware.BanCheck(http.HandlerFunc(handlers.DeletePost))))
	mux.Handle("POST /api/posts/{id}/comments", middleware.RequireAuth(middleware.BanCheck(middleware.UserRateLimit("comment")(http.HandlerFunc(handlers.CreateComment)))))
	mux.Handle("POST /api/posts/{id}/report", middleware.RequireAuth(http.HandlerFunc(handlers.ReportPost)))
	mux.Handle("POST /api/comments/{id}/report", middleware.RequireAuth(http.HandlerFunc(handlers.ReportComment)))
	mux.Handle("GET /api/my/posts", middleware.RequireAuth(http.HandlerFunc(handlers.MyPosts)))

	// 上传
	mux.Handle("POST /api/uploads", middleware.RequireAuth(middleware.BanCheck(middleware.UserRateLimit("upload")(http.HandlerFunc(handlers.Upload)))))
	mux.Handle("GET /api/uploads", middleware.RequireAuth(http.HandlerFunc(handlers.MyUploads)))
	mux.Handle("GET /api/uploads/{id}/download", middleware.RequireAuth(http.HandlerFunc(handlers.Download)))
	mux.Handle("DELETE /api/uploads/{id}", middleware.RequireAuth(middleware.BanCheck(http.HandlerFunc(handlers.DeleteUpload))))

	// 管理后台
	admin := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(middleware.RequireAdmin(h))
	}
	mux.Handle("GET /api/admin/dashboard", admin(handlers.AdminDashboard))
	mux.Handle("GET /api/admin/review/posts", admin(handlers.AdminReviewPosts))
	mux.Handle("POST /api/admin/review/posts/{id}", admin(handlers.AdminReviewPost))
	mux.Handle("GET /api/admin/reports", admin(handlers.AdminReports))
	mux.Handle("POST /api/admin/reports/{id}/review", admin(handlers.AdminReviewReport))
	mux.Handle("GET /api/admin/users", admin(handlers.AdminUsers))
	mux.Handle("POST /api/admin/users/{id}", admin(handlers.AdminUserAction))
	mux.Handle("GET /api/admin/emails", admin(handlers.AdminEmails))
	mux.Handle("POST /api/admin/emails", admin(handlers.AdminAddEmail))
	mux.Handle("DELETE /api/admin/emails/{id}", admin(handlers.AdminDeleteEmail))
	mux.Handle("GET /api/admin/trust", admin(handlers.AdminTrustDevices))
	mux.Handle("POST /api/admin/trust/{id}/clear", admin(handlers.AdminClearTrust))
	mux.Handle("POST /api/admin/shield", admin(handlers.AdminShield))
	mux.Handle("POST /api/admin/unblock-ip", admin(handlers.AdminUnblockIP))
	mux.Handle("GET /api/admin/weekly", admin(handlers.AdminWeeklyReports))
	mux.Handle("POST /api/admin/weekly/generate", admin(handlers.AdminGenerateWeekly))
	mux.Handle("GET /api/admin/logs", admin(handlers.AdminLogs))
	mux.Handle("GET /api/admin/banned-words", admin(handlers.AdminBannedWords))
	mux.Handle("POST /api/admin/banned-words/reload", admin(handlers.AdminReloadBannedWords))

	// 静态前端（SPA fallback）
	feDir := cfg.Frontend.Dir
	if abs, err := filepath.Abs(feDir); err == nil {
		feDir = abs
	}
	mux.HandleFunc("/", spaHandler(feDir))

	// 中间件链（执行顺序：SecurityHeaders → Monitor → BlockedIPCheck → ShieldCheck → RateLimit）
	// Monitor 必须位于盾检查之前，以便注入客户端 IP
	var h http.Handler = mux
	h = middleware.RateLimit("general")(h)
	h = middleware.ShieldCheck(h)
	h = middleware.BlockedIPCheck(h)
	h = middleware.Monitor(h)
	h = middleware.SecurityHeaders(h)
	return h
}

func spaHandler(dir string) http.HandlerFunc {
	index := filepath.Join(dir, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, index)
	}
}
