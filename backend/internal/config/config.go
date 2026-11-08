package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Host           string `yaml:"host"`
		Port           int    `yaml:"port"`
		ReadTimeoutSec int    `yaml:"read_timeout_sec"`
		WriteTimeout   int    `yaml:"write_timeout_sec"`
		IdleTimeoutSec int    `yaml:"idle_timeout_sec"`
		// 仅当部署在反向代理后且代理会覆盖 X-Forwarded-For 时才开启
		TrustProxy bool `yaml:"trust_proxy"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Security struct {
		JWTSecret               string `yaml:"jwt_secret"`
		JWTExpireHours          int    `yaml:"jwt_expire_hours"`
		Argon2MemoryKB          uint32 `yaml:"argon2_memory_kb"`
		Argon2Iterations        uint32 `yaml:"argon2_iterations"`
		Argon2Parallelism       uint8  `yaml:"argon2_parallelism"`
		AdminRequireOTPNewDev   bool   `yaml:"admin_require_otp_new_device"`
		RateLimit               struct {
			GeneralPerMin int `yaml:"general_per_min"`
			LoginPerMin   int `yaml:"login_per_min"`
			RegisterPerHour int `yaml:"register_per_hour"`
			PostPerMin    int `yaml:"post_per_min"`
			CommentPerMin int `yaml:"comment_per_min"`
			UploadPerMin  int `yaml:"upload_per_min"`
		} `yaml:"rate_limit"`
	} `yaml:"security"`
	DDOS struct {
		Enabled              bool     `yaml:"enabled"`
		TriggerRPS           int      `yaml:"trigger_rps"`
		SpikeFactor          float64  `yaml:"spike_factor"`
		CheckIntervalSec     int      `yaml:"check_interval_sec"`
		AutoRecoverMin       int      `yaml:"auto_recover_min"`
		RecoverThresholdRPS  int      `yaml:"recover_threshold_rps"`
		PerIPThresholdPerMin int      `yaml:"per_ip_threshold_per_min"`
		IPBlockMinutes       int      `yaml:"ip_block_minutes"`
		WhitelistIPs         []string `yaml:"whitelist_ips"`
	} `yaml:"ddos"`
	Content struct {
		PostMaxChars    int `yaml:"post_max_chars"`
		CommentMaxChars int `yaml:"comment_max_chars"`
		TitleMaxChars   int `yaml:"title_max_chars"`
		Upload          struct {
			BaseQuotaBytes    int64  `yaml:"base_quota_bytes"`
			Level10QuotaBytes int64  `yaml:"level10_quota_bytes"`
			MaxFileBytes      int64  `yaml:"max_file_bytes"`
			Dir               string `yaml:"dir"`
		} `yaml:"upload"`
		BannedWordsFile string `yaml:"banned_words_file"`
		Compensation    struct {
			QuotaBonusBytes int64 `yaml:"quota_bonus_bytes"`
			MaxBonuses      int   `yaml:"max_bonuses"`
		} `yaml:"compensation"`
	} `yaml:"content"`
	Email struct {
		Enabled  bool   `yaml:"enabled"`
		SMTPHost string `yaml:"smtp_host"`
		SMTPPort int    `yaml:"smtp_port"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		From     string `yaml:"from"`
		UseTLS   bool   `yaml:"use_tls"`
	} `yaml:"email"`
	Report struct {
		WeeklyISODay int    `yaml:"weekly_iso_day"`
		Dir          string `yaml:"dir"`
	} `yaml:"report"`
	Frontend struct {
		Dir string `yaml:"dir"`
	} `yaml:"frontend"`
}

var Cfg *Config

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Security.JWTSecret == "" {
		return nil, fmt.Errorf("security.jwt_secret is required")
	}
	if cfg.Security.Argon2MemoryKB == 0 {
		cfg.Security.Argon2MemoryKB = 65536
	}
	if cfg.Security.Argon2Iterations == 0 {
		cfg.Security.Argon2Iterations = 3
	}
	if cfg.Security.Argon2Parallelism == 0 {
		cfg.Security.Argon2Parallelism = 2
	}
	if cfg.DDOS.CheckIntervalSec <= 0 {
		cfg.DDOS.CheckIntervalSec = 5
	}
	Cfg = cfg
	return cfg, nil
}
