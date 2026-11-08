package moderation

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/Garl404/IT_Forum/backend/internal/models"
)

type Engine struct {
	mu     sync.RWMutex
	words  []models.BannedWord
	normed map[string]models.BannedWord
}

var Default *Engine

func Load(path string) (*Engine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var words []models.BannedWord
	if err := json.Unmarshal(data, &words); err != nil {
		return nil, err
	}
	e := &Engine{normed: make(map[string]models.BannedWord)}
	for _, w := range words {
		e.words = append(e.words, w)
		e.normed[Normalize(w.Word)] = w
	}
	Default = e
	return e, nil
}

func (e *Engine) List() []models.BannedWord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]models.BannedWord, len(e.words))
	copy(out, e.words)
	return out
}

func (e *Engine) Reload(path string) error {
	ne, err := Load(path)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.words, e.normed = ne.words, ne.normed
	e.mu.Unlock()
	Default = e
	return nil
}

// 形近字/变体映射（规避用相似字符绕过检测）
var homoglyphs = map[rune]rune{
	// 西里尔字母
	'а': 'a', 'в': 'b', 'с': 'c', 'е': 'e', 'к': 'k', 'м': 'm', 'н': 'h',
	'о': 'o', 'р': 'p', 'т': 't', 'у': 'y', 'х': 'x', 'г': 'r',
	// 希腊字母
	'α': 'a', 'β': 'b', 'γ': 'y', 'ε': 'e', 'η': 'n', 'ι': 'i', 'κ': 'k',
	'ν': 'v', 'ο': 'o', 'ρ': 'p', 'τ': 't', 'υ': 'u', 'χ': 'x',
	// 其他形近字符
	'і': 'i', 'ѕ': 's', 'ԁ': 'd', 'ј': 'j', 'ԝ': 'w', 'ӏ': 'l',
	'ⓐ': 'a', 'ⓑ': 'b', 'ⓒ': 'c', 'ⓓ': 'd', 'ⓔ': 'e',
	'©': 'c', '®': 'r',
	'①': '1', '②': '2', '③': '3', '④': '4', '⑤': '5',
}

var leet = map[rune]rune{
	'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't',
	'@': 'a', '$': 's', '!': 'i', '+': 't', '8': 'b', '9': 'g', '6': 'g',
}

// 全角字符转半角
func halfwidth(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	if r == 0x3000 {
		return ' '
	}
	return r
}

// Normalize 归一化文本：小写、全角转半角、形近字/leet 还原、去除非字母数字
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		r = halfwidth(r)
		if h, ok := homoglyphs[r]; ok {
			r = h
		} else if l, ok := leet[r]; ok {
			r = l
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type Result struct {
	Severity    int      `json:"severity"`
	Matched     []string `json:"matched"`
	MatchedNote string   `json:"matched_note"`
}

// Check 检测文本是否命中违禁词，返回最高严重等级与命中词
func (e *Engine) Check(text string) Result {
	e.mu.RLock()
	defer e.mu.RUnlock()
	res := Result{}
	seen := map[string]bool{}
	// 通道1：去空白/符号的归一化文本（防分隔符绕过）
	plain := Normalize(text)
	for norm, w := range e.normed {
		if norm == "" {
			continue
		}
		if strings.Contains(plain, norm) {
			if w.Severity > res.Severity {
				res.Severity = w.Severity
			}
			if !seen[w.Word] {
				seen[w.Word] = true
				res.Matched = append(res.Matched, w.Word+"(s"+itoa(w.Severity)+")")
			}
		}
	}
	return res
}

// 数字转字符串小工具
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [12]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// BanDuration 按违禁程度（严重等级）确定封禁时长（秒）
func BanDuration(severity int) int64 {
	switch {
	case severity >= 5:
		return -1 // 永久
	case severity == 4:
		return 30 * 24 * 3600
	case severity == 3:
		return 7 * 24 * 3600
	case severity == 2:
		return 3 * 24 * 3600
	default:
		return 24 * 3600
	}
}

func BanDurationText(severity int) string {
	switch {
	case severity >= 5:
		return "permanent"
	case severity == 4:
		return "30d"
	case severity == 3:
		return "7d"
	case severity == 2:
		return "3d"
	default:
		return "1d"
	}
}
