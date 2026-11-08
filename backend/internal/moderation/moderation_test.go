package moderation

import (
	"testing"

	"github.com/Garl404/IT_Forum/backend/internal/models"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"毒品", "毒品"},
		{"毒 品", "毒品"},
		{"毒-品", "毒品"},
		{"毒。品", "毒品"},
		{"ＤＲＵＧ", "drug"},     // 全角转半角
		{"dr0g", "drog"},        // leet 0→o
		{"dru9", "drug"},        // leet 9→g
		{"ｆｕｃｋ", "fuck"},   // 全角
		{"f.u.c.k", "fuck"},     // 分隔符绕过
		{"fu©k", "fuck"},        // 符号剥离
		{"Хакер", "xakep"},      // 西里尔形近字
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func newEngine() *Engine {
	return &Engine{normed: map[string]models.BannedWord{
		"毒品":   {Word: "毒品", Severity: 5},
		"fuck":   {Word: "fuck", Severity: 2},
		"heroin": {Word: "heroin", Severity: 5},
	}}
}

func TestCheckPlain(t *testing.T) {
	e := newEngine()
	if r := e.Check("这里有毒品交易"); r.Severity != 5 {
		t.Errorf("plain match: severity=%d, want 5", r.Severity)
	}
	if r := e.Check("fuck you"); r.Severity != 2 {
		t.Errorf("en match: severity=%d, want 2", r.Severity)
	}
	if r := e.Check("正常的技术讨论"); r.Severity != 0 || len(r.Matched) != 0 {
		t.Errorf("clean text flagged: %+v", r)
	}
}

func TestCheckVariantBypass(t *testing.T) {
	e := newEngine()
	cases := []string{
		"毒 品", "毒·品", "毒_品", "毒【品】", "毒\t品", // 分隔符
		"ｆｕｃｋ", "f.u.c.k", "F_U_C_K",        // 全角/分隔/大小写
		"h3r01n",                                 // leet
	}
	for _, c := range cases {
		if r := e.Check(c); r.Severity == 0 {
			t.Errorf("变体未检出: %q", c)
		}
	}
}

func TestBanDuration(t *testing.T) {
	cases := map[int]int64{
		1: 86400,
		2: 3 * 86400,
		3: 7 * 86400,
		4: 30 * 86400,
		5: -1,
	}
	for sev, want := range cases {
		if got := BanDuration(sev); got != want {
			t.Errorf("BanDuration(%d) = %d, want %d", sev, got, want)
		}
	}
}
