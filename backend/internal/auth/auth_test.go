package auth

import (
	"testing"

	"github.com/Garl404/IT_Forum/backend/internal/config"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Security.JWTSecret = "test-secret-0123456789"
	cfg.Security.JWTExpireHours = 1
	cfg.Security.Argon2MemoryKB = 4096
	cfg.Security.Argon2Iterations = 1
	cfg.Security.Argon2Parallelism = 1
	return cfg
}

func TestHashVerifyPassword(t *testing.T) {
	cfg := testConfig()
	hash, err := HashPassword("caluse_665544Xx", cfg)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !VerifyPassword(hash, "caluse_665544Xx") {
		t.Error("正确密码校验失败")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("错误密码竟然校验通过")
	}
	if VerifyPassword(hash, "") {
		t.Error("空密码竟然校验通过")
	}
	if VerifyPassword("not-a-valid-hash", "x") {
		t.Error("非法哈希格式竟然通过")
	}
}

func TestJWTIssueParse(t *testing.T) {
	cfg := testConfig()
	token, err := IssueToken(cfg, 42, "tester", "user", 3)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := ParseToken(cfg, token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "tester" || claims.Role != "user" || claims.Level != 3 {
		t.Errorf("claims 不匹配: %+v", claims)
	}
	// 篡改后的 token 必须被拒绝
	if _, err := ParseToken(cfg, token+"x"); err == nil {
		t.Error("篡改 token 未被拒绝")
	}
	// 其他密钥签发的 token 必须被拒绝
	other := testConfig()
	other.Security.JWTSecret = "another-secret"
	token2, _ := IssueToken(other, 1, "x", "user", 1)
	if _, err := ParseToken(cfg, token2); err == nil {
		t.Error("错误密钥签发的 token 未被拒绝")
	}
}

func TestOTP(t *testing.T) {
	store := &OTPStore{store: make(map[string]otpEntry)}
	code, err := store.Generate("admin@example.com")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("验证码长度 = %d, want 6", len(code))
	}
	if !store.Verify("ADMIN@example.com", code) { // 大小写不敏感
		t.Error("正确验证码（大写邮箱）校验失败")
	}
	if store.Verify("admin@example.com", code) {
		t.Error("同一验证码重复使用竟然通过")
	}
	if store.Verify("admin@example.com", "000000") {
		t.Error("错误验证码竟然通过")
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint("device-raw-fingerprint", "secret")
	b := Fingerprint("device-raw-fingerprint", "secret")
	c := Fingerprint("device-raw-fingerprint", "other-secret")
	if a != b {
		t.Error("同输入同密钥指纹不一致")
	}
	if a == c {
		t.Error("不同密钥指纹竟然相同")
	}
}
