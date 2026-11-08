package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/models"
)

var DB *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  email TEXT UNIQUE NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  level INTEGER NOT NULL DEFAULT 1,
  score INTEGER NOT NULL DEFAULT 0,
  quota_bonus_bytes INTEGER NOT NULL DEFAULT 0,
  bonus_count INTEGER NOT NULL DEFAULT 0,
  banned_until INTEGER NOT NULL DEFAULT 0,
  ban_reason TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  last_login_at INTEGER NOT NULL DEFAULT 0,
  last_login_ip TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT UNIQUE NOT NULL,
  name_key TEXT NOT NULL,
  sort INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  category_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'published',
  flag_severity INTEGER NOT NULL DEFAULT 0,
  flag_reason TEXT NOT NULL DEFAULT '',
  views INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_posts_cat ON posts(category_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_posts_user ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);

CREATE TABLE IF NOT EXISTS comments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  post_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  content TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'published',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id);

CREATE TABLE IF NOT EXISTS uploads (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  post_id INTEGER NOT NULL DEFAULT 0,
  filename TEXT NOT NULL,
  stored_name TEXT NOT NULL,
  size INTEGER NOT NULL,
  mime TEXT NOT NULL DEFAULT '',
  sha256 TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_uploads_user ON uploads(user_id);

CREATE TABLE IF NOT EXISTS reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  post_id INTEGER NOT NULL DEFAULT 0,
  comment_id INTEGER NOT NULL DEFAULT 0,
  reporter_id INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  severity INTEGER NOT NULL DEFAULT 0,
  reviewed_by INTEGER NOT NULL DEFAULT 0,
  reviewed_at INTEGER NOT NULL DEFAULT 0,
  note TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);

CREATE TABLE IF NOT EXISTS admin_trust (
  user_id INTEGER PRIMARY KEY,
  ips TEXT NOT NULL DEFAULT '[]',
  fingerprints TEXT NOT NULL DEFAULT '[]',
  updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS alert_emails (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT UNIQUE NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS weekly_reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  week_start TEXT UNIQUE NOT NULL,
  week_end TEXT NOT NULL,
  data TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS visits (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  ua TEXT NOT NULL DEFAULT '',
  ts INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_visits_ts ON visits(ts);

CREATE TABLE IF NOT EXISTS moderation_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  admin_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL DEFAULT '',
  target_id INTEGER NOT NULL DEFAULT 0,
  detail TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS site_state (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL DEFAULT ''
);
`

func Open(cfg *config.Config) error {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-64000)", cfg.Database.Path)
	dir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	var err error
	DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	DB.SetMaxOpenConns(16)
	DB.SetMaxIdleConns(8)
	if _, err := DB.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if err := seed(); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}

// SeedAdmin 创建站长账号（密码哈希由调用方生成）
func SeedAdmin(passwordHash string) error {
	now := time.Now().Unix()
	var count int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	res, err := DB.Exec(`INSERT INTO users(username, email, password_hash, role, level, created_at) VALUES(?,?,?,?,?,?)`,
		"caluse_admin", "", passwordHash, models.RoleAdmin, 10, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if _, err := DB.Exec(`INSERT OR IGNORE INTO admin_trust(user_id, ips, fingerprints, updated_at) VALUES(?,?,?,?)`,
		id, "[]", "[]", now); err != nil {
		return err
	}
	if _, err := DB.Exec(`INSERT OR IGNORE INTO alert_emails(email, note, created_at) VALUES(?,?,?)`,
		"admin@caluse.local", "default", now); err != nil {
		return err
	}
	log.Println("[seed] 已创建站长账号 caluse_admin（请尽快在控制台设置邮箱并修改密码）")
	return nil
}

func seed() error {
	now := time.Now().Unix()
	cats := []struct {
		slug, key string
		sort      int
	}{
		{"linux", "Linux", 1},
		{"programming", "Programming", 2},
		{"network", "Network", 3},
		{"security", "Security", 4},
		{"hardware", "Hardware", 5},
		{"devops", "DevOps", 6},
		{"ai", "AI & Data", 7},
		{"news", "IT News", 8},
		{"help", "Help", 9},
		{"chat", "Lounge", 10},
	}
	for _, c := range cats {
		if _, err := DB.Exec(`INSERT OR IGNORE INTO categories(slug, name_key, sort) VALUES(?,?,?)`, c.slug, c.key, c.sort); err != nil {
			return err
		}
	}
	_ = now
	return nil
}

func Now() int64 { return time.Now().Unix() }
