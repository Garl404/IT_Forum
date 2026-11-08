package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/auth"
	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/monitor"
)

type ctxKey string

const (
	CtxUser  ctxKey = "user"
	CtxIP    ctxKey = "ip"
	CtxAdmin ctxKey = "admin"
)

// SecurityHeaders 安全响应头
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		if r.Method == http.MethodGet {
			h.Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

// Monitor 记录访问指标（供 DDoS 检测与统计）
func Monitor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := monitor.ClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Real-IP"), config.Cfg.Server.TrustProxy)
		ctx := context.WithValue(r.Context(), CtxIP, ip)
		monitor.M.Record(ip, r.URL.Path, r.Header.Get("User-Agent"))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ShieldCheck 盾模式：非白名单 IP 一律 503（保留登录接口供管理员进入）
func ShieldCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !monitor.M.IsShielded() {
			next.ServeHTTP(w, r)
			return
		}
		ip, _ := r.Context().Value(CtxIP).(string)
		path := r.URL.Path
		allowed := strings.HasPrefix(path, "/api/health") ||
			strings.HasPrefix(path, "/api/auth/login") ||
			strings.HasPrefix(path, "/api/auth/verify-otp") ||
			strings.HasPrefix(path, "/api/auth/me")
		if allowed || adminIPWhitelisted(ip) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"site_shielded","message":"网站正遭受异常流量攻击，已启动自动保护。请稍后再试。"}`))
	})
}

func adminIPWhitelisted(ip string) bool {
	return monitor.WhitelistedIP(ip)
}

// BlockedIPCheck 临时封禁 IP
func BlockedIPCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _ := r.Context().Value(CtxIP).(string)
		if ip != "" && monitor.M.IsIPBlocked(ip) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"ip_blocked","message":"请求过于频繁，IP 已被临时限制"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- 令牌桶限流 ----

type bucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	mu   sync.Mutex
	m    map[string]*bucket
	rate float64 // 每秒补充
	cap  float64
}

func newLimiter(rate, cap float64) *limiter {
	return &limiter{m: make(map[string]*bucket), rate: rate, cap: cap}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.m[key]
	if !ok {
		b = &bucket{tokens: l.cap, last: now}
		l.m[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.cap {
		b.tokens = l.cap
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (l *limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for k, b := range l.m {
		if b.last.Before(cutoff) {
			delete(l.m, k)
		}
	}
}

var (
	generalLimiter  = newLimiter(0, 0)
	loginLimiter    = newLimiter(0, 0)
	registerLimiter = newLimiter(0, 0)
	userLimiters    = map[string]*limiter{
		"post":    newLimiter(0, 0),
		"comment": newLimiter(0, 0),
		"upload":  newLimiter(0, 0),
	}
	initOnce sync.Once
)

func initLimiters(cfg *config.Config) {
	generalLimiter = newLimiter(float64(cfg.Security.RateLimit.GeneralPerMin)/60.0, float64(cfg.Security.RateLimit.GeneralPerMin))
	loginLimiter = newLimiter(float64(cfg.Security.RateLimit.LoginPerMin)/60.0, float64(cfg.Security.RateLimit.LoginPerMin))
	registerLimiter = newLimiter(float64(cfg.Security.RateLimit.RegisterPerHour)/3600.0, float64(cfg.Security.RateLimit.RegisterPerHour))
	userLimiters["post"] = newLimiter(float64(cfg.Security.RateLimit.PostPerMin)/60.0, float64(cfg.Security.RateLimit.PostPerMin))
	userLimiters["comment"] = newLimiter(float64(cfg.Security.RateLimit.CommentPerMin)/60.0, float64(cfg.Security.RateLimit.CommentPerMin))
	userLimiters["upload"] = newLimiter(float64(cfg.Security.RateLimit.UploadPerMin)/60.0, float64(cfg.Security.RateLimit.UploadPerMin))
	go func() {
		t := time.NewTicker(5 * time.Minute)
		for range t.C {
			generalLimiter.cleanup()
			loginLimiter.cleanup()
			registerLimiter.cleanup()
		}
	}()
}

func Setup(cfg *config.Config) {
	initOnce.Do(func() { initLimiters(cfg) })
}

func tooMany(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":"rate_limited","message":"操作过于频繁，请稍后再试"}`))
}

func RateLimit(kind string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _ := r.Context().Value(CtxIP).(string)
			var l *limiter
			switch kind {
			case "general":
				l = generalLimiter
			case "login":
				l = loginLimiter
			case "register":
				l = registerLimiter
			default:
				l = generalLimiter
			}
			if !l.allow(ip) {
				tooMany(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserRateLimit 按用户限流（发帖/评论/上传）
func UserRateLimit(kind string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFrom(r.Context())
			if u == nil {
				next.ServeHTTP(w, r)
				return
			}
			l := userLimiters[kind]
			if l == nil {
				next.ServeHTTP(w, r)
				return
			}
			key := auth.ClaimsKey(u.ID) + ":" + kind
			if !l.allow(key) {
				tooMany(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth 解析 JWT 并注入用户
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ah := r.Header.Get("Authorization")
		if !strings.HasPrefix(ah, "Bearer ") {
			unauthorized(w, "未登录")
			return
		}
		claims, err := auth.ParseToken(config.Cfg, strings.TrimPrefix(ah, "Bearer "))
		if err != nil {
			unauthorized(w, "登录已过期，请重新登录")
			return
		}
		u := auth.LoadUser(claims.UserID)
		if u == nil {
			unauthorized(w, "账号不存在")
			return
		}
		ctx := context.WithValue(r.Context(), CtxUser, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","message":"` + msg + `"}`))
}

// RequireAdmin 管理员权限
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil || u.Role != "admin" {
			forbidden(w)
			return
		}
		ctx := context.WithValue(r.Context(), CtxAdmin, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func forbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"forbidden","message":"权限不足"}`))
}

// BanCheck 封禁用户禁止写操作
func BanCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		u := UserFrom(r.Context())
		if u != nil && u.BannedUntil != 0 {
			now := time.Now().Unix()
			if u.BannedUntil == -1 || u.BannedUntil > now {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				msg := "账号已被封禁"
				if u.BanReason != "" {
					msg += "：" + u.BanReason
				}
				if u.BannedUntil > now {
					msg += "，解封时间 " + time.Unix(u.BannedUntil, 0).Format("2006-01-02 15:04")
				}
				w.Write([]byte(`{"error":"banned","message":"` + msg + `"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func UserFrom(ctx context.Context) *auth.AuthUser {
	u, _ := ctx.Value(CtxUser).(*auth.AuthUser)
	return u
}

func IPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(CtxIP).(string)
	return ip
}
