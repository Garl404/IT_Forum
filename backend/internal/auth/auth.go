package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"

	"github.com/Garl404/IT_Forum/backend/internal/config"
)

const (
	argonSaltLen = 16
	argonKeyLen  = 32
)

func HashPassword(password string, cfg *config.Config) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, cfg.Security.Argon2Iterations, cfg.Security.Argon2MemoryKB, cfg.Security.Argon2Parallelism, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		cfg.Security.Argon2MemoryKB, cfg.Security.Argon2Iterations, cfg.Security.Argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	expect, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(expect)))
	return subtle.ConstantTimeCompare(got, expect) == 1
}

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"uname"`
	Role     string `json:"role"`
	Level    int    `json:"lv"`
	jwt.RegisteredClaims
}

func IssueToken(cfg *config.Config, uid int64, username, role string, level int) (string, error) {
	claims := Claims{
		UserID:   uid,
		Username: username,
		Role:     role,
		Level:    level,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.Security.JWTExpireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "caluse",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.Security.JWTSecret))
}

func ParseToken(cfg *config.Config, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(cfg.Security.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// Fingerprint 将客户端指纹做加盐哈希，便于安全存储与比对
func Fingerprint(raw, secret string) string {
	sum := sha256.Sum256([]byte(raw + "|" + secret))
	return hex.EncodeToString(sum[:])
}

// OTP 邮箱验证码（内存存储）
type OTPStore struct {
	mu     sync.Mutex
	store  map[string]otpEntry
	secret string
}

type otpEntry struct {
	code      string
	expiresAt time.Time
	attempts  int
}

var OTP = &OTPStore{store: make(map[string]otpEntry)}

func (s *OTPStore) Generate(email string) (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", int(buf[0])%10*100000+int(buf[1])%10*10000+int(buf[2])%10*1000+int(buf[3])%10*100+(int(buf[0]+buf[1])%10)*10+(int(buf[2]+buf[3])%10))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[strings.ToLower(email)] = otpEntry{code: code, expiresAt: time.Now().Add(10 * time.Minute), attempts: 0}
	return code, nil
}

func (s *OTPStore) Verify(email, code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.ToLower(email)
	e, ok := s.store[key]
	if !ok {
		return false
	}
	if e.attempts >= 5 || time.Now().After(e.expiresAt) {
		delete(s.store, key)
		return false
	}
	e.attempts++
	if subtle.ConstantTimeCompare([]byte(e.code), []byte(code)) == 1 {
		delete(s.store, key)
		return true
	}
	s.store[key] = e
	return false
}
