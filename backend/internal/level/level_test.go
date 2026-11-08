package level

import "testing"

func TestScore(t *testing.T) {
	s := Score(30, 10, 20, 2) // 300 + 500 + 400 + 300
	if s != 1500 {
		t.Errorf("Score = %d, want 1500", s)
	}
	if Score(0, 0, 0, 0) != 0 {
		t.Error("空账号分数应为 0")
	}
}

func TestFromScore(t *testing.T) {
	cases := []struct {
		score int64
		want  int
	}{
		{0, 1}, {399, 1}, {400, 2}, {1199, 2}, {1200, 3}, {2599, 3},
		{2600, 4}, {4799, 4}, {4800, 5}, {7999, 5}, {8000, 6},
		{12499, 6}, {12500, 7}, {18499, 7}, {18500, 8},
		{26499, 8}, {26500, 9}, {36999, 9}, {37000, 10}, {999999, 10},
		{-5, 1},
	}
	for _, c := range cases {
		if got := FromScore(c.score); got != c.want {
			t.Errorf("FromScore(%d) = %d, want %d", c.score, got, c.want)
		}
	}
}

func TestName(t *testing.T) {
	if Name(10) != "LV10 · 一代宗师" {
		t.Errorf("Name(10) = %q", Name(10))
	}
	if Name(0) != "LV1 · 新手上路" {
		t.Errorf("Name(0) 应回退到 LV1")
	}
	if Name(99) != "LV10 · 一代宗师" {
		t.Errorf("Name(99) 应封顶 LV10")
	}
}
