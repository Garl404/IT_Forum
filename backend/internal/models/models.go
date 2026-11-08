package models

type User struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email,omitempty"`
	PasswordHash    string `json:"-"`
	Role            string `json:"role"`
	Level           int    `json:"level"`
	Score           int64  `json:"score"`
	QuotaBonusBytes int64  `json:"quota_bonus_bytes"`
	BonusCount      int    `json:"bonus_count"`
	BannedUntil     int64  `json:"banned_until"`
	BanReason       string `json:"ban_reason,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	LastLoginAt     int64  `json:"last_login_at"`
	LastLoginIP     string `json:"last_login_ip,omitempty"`
	PostCount       int64  `json:"post_count,omitempty"`
	CommentCount    int64  `json:"comment_count,omitempty"`
	UploadUsedBytes int64  `json:"upload_used_bytes,omitempty"`
	UploadQuota     int64  `json:"upload_quota,omitempty"`
}

type PublicUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Level     int    `json:"level"`
	CreatedAt int64  `json:"created_at"`
	PostCount int64  `json:"post_count"`
}

type Category struct {
	ID      int64  `json:"id"`
	Slug    string `json:"slug"`
	NameKey string `json:"name_key"`
	Sort    int    `json:"sort"`
}

type Post struct {
	ID           int64       `json:"id"`
	UserID       int64       `json:"user_id"`
	Username     string      `json:"username"`
	UserLevel    int         `json:"user_level"`
	CategoryID   int64       `json:"category_id"`
	CategorySlug string      `json:"category_slug,omitempty"`
	Title        string      `json:"title"`
	Content      string      `json:"content"`
	Status       string      `json:"status"`
	FlagSeverity int         `json:"flag_severity"`
	FlagReason   string      `json:"flag_reason,omitempty"`
	Views        int64       `json:"views"`
	CommentCount int64       `json:"comment_count"`
	CreatedAt    int64       `json:"created_at"`
	UpdatedAt    int64       `json:"updated_at"`
}

type Comment struct {
	ID        int64  `json:"id"`
	PostID    int64  `json:"post_id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	UserLevel int    `json:"user_level"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

type Upload struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	PostID     int64  `json:"post_id"`
	Filename   string `json:"filename"`
	StoredName string `json:"-"`
	Size       int64  `json:"size"`
	Mime       string `json:"mime"`
	Sha256     string `json:"sha256,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

type Report struct {
	ID         int64  `json:"id"`
	PostID     int64  `json:"post_id"`
	CommentID  int64  `json:"comment_id"`
	ReporterID int64  `json:"reporter_id"`
	Reporter   string `json:"reporter,omitempty"`
	Reason     string `json:"reason"`
	Status     string `json:"status"`
	Severity   int    `json:"severity"`
	ReviewedBy int64  `json:"reviewed_by,omitempty"`
	ReviewedAt int64  `json:"reviewed_at,omitempty"`
	Note       string `json:"note,omitempty"`
	CreatedAt  int64  `json:"created_at"`
	PostTitle  string `json:"post_title,omitempty"`
	TargetUser string `json:"target_user,omitempty"`
}

type AdminTrust struct {
	UserID       int64    `json:"user_id"`
	IPs          []string `json:"ips"`
	Fingerprints []string `json:"fingerprints"`
	UpdatedAt    int64    `json:"updated_at"`
}

type AlertEmail struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"created_at"`
}

type WeeklyReport struct {
	ID        int64  `json:"id"`
	WeekStart string `json:"week_start"`
	WeekEnd   string `json:"week_end"`
	Data      string `json:"data"`
	CreatedAt int64  `json:"created_at"`
}

type ModerationLog struct {
	ID         int64  `json:"id"`
	AdminID    int64  `json:"admin_id"`
	AdminName  string `json:"admin_name,omitempty"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Detail     string `json:"detail"`
	CreatedAt  int64  `json:"created_at"`
}

type BannedWord struct {
	Word     string `json:"word"`
	Severity int    `json:"severity"`
	Lang     string `json:"lang"`
	Note     string `json:"note"`
}

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	PostPublished     = "published"
	PostPendingReview = "pending_review"
	PostRemoved       = "removed"

	ReportPending   = "pending"
	ReportApproved  = "approved"
	ReportConfirmed = "confirmed"
)
