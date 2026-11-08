package level

// 10 级用户等级：综合注册时长（天）、发帖数、评论数、举报确认贡献计算成长分
var thresholds = [10]int64{0, 400, 1200, 2600, 4800, 8000, 12500, 18500, 26500, 37000}

var names = map[int]string{
	1:  "LV1 · 新手上路",
	2:  "LV2 · 初窥门径",
	3:  "LV3 · 渐入佳境",
	4:  "LV4 · 小有所成",
	5:  "LV5 · 驾轻就熟",
	6:  "LV6 · 融会贯通",
	7:  "LV7 · 炉火纯青",
	8:  "LV8 · 出类拔萃",
	9:  "LV9 · 登峰造极",
	10: "LV10 · 一代宗师",
}

func Score(regDays int64, posts, comments, confirmedReports int64) int64 {
	return regDays*10 + posts*50 + comments*20 + confirmedReports*150
}

func FromScore(score int64) int {
	lv := 1
	for i, th := range thresholds {
		if score >= th {
			lv = i + 1
		}
	}
	if lv > 10 {
		lv = 10
	}
	return lv
}

func Name(lv int) string {
	if lv < 1 {
		lv = 1
	}
	if lv > 10 {
		lv = 10
	}
	return names[lv]
}
