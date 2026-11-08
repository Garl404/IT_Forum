package monitor

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/config"
)

// SendEmail 发送 HTML 邮件；未配置 SMTP 时写入本地 outbox 目录
func SendEmail(cfg *config.Config, to []string, subject, htmlBody string) error {
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}
	if !cfg.Email.Enabled || cfg.Email.SMTPHost == "" {
		dir := "data/outbox"
		_ = os.MkdirAll(dir, 0o755)
		name := fmt.Sprintf("%s_%s.html", time.Now().Format("20060102_150405"), sanitize(subject))
		full := filepath.Join(dir, name)
		body := fmt.Sprintf("<h3>To: %s</h3><h2>%s</h2><hr>%s", strings.Join(to, ", "), subject, htmlBody)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			return err
		}
		return nil
	}

	from := cfg.Email.From
	if from == "" {
		from = cfg.Email.Username
	}
	msg := buildMime(from, to, subject, htmlBody)
	addr := fmt.Sprintf("%s:%d", cfg.Email.SMTPHost, cfg.Email.SMTPPort)
	if cfg.Email.UseTLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Email.SMTPHost, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, cfg.Email.SMTPHost)
		if err != nil {
			return err
		}
		defer c.Close()
		if cfg.Email.Username != "" {
			auth := smtp.PlainAuth("", cfg.Email.Username, cfg.Email.Password, cfg.Email.SMTPHost)
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		for _, rcpt := range to {
			if err := c.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			return err
		}
		return w.Close()
	}
	return smtp.SendMail(addr, smtp.PlainAuth("", cfg.Email.Username, cfg.Email.Password, cfg.Email.SMTPHost), from, to, msg)
}

func buildMime(from string, to []string, subject, html string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html)
	return []byte(b.String())
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ClientIP 从请求中提取真实 IP
// 仅当 trustProxy 开启时才信任 X-Forwarded-For/X-Real-IP（防止伪造头绕过防护）
func ClientIP(remoteAddr, xff, xRealIP string, trustProxy bool) string {
	if trustProxy {
		if xRealIP != "" {
			return strings.TrimSpace(strings.Split(xRealIP, ",")[0])
		}
		if xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
