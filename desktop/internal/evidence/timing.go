package evidence

import (
	"math"
	"time"
)

func roundMinutes(duration time.Duration) float64 {
	return math.RoundToEven(duration.Minutes()*10) / 10
}

func TimingEvidence(messages []Message, now time.Time) map[string]interface{} {
	now = now.In(China)
	period := "深夜"
	switch {
	case now.Hour() < 6:
		period = "凌晨"
	case now.Hour() < 12:
		period = "上午"
	case now.Hour() < 18:
		period = "下午"
	case now.Hour() < 23:
		period = "晚上"
	}
	intervals := []map[string]interface{}{}
	for i := 1; i < len(messages); i++ {
		a, okA := parseTimestamp(messages[i-1]["message_time"])
		b, okB := parseTimestamp(messages[i]["message_time"])
		if okA && okB && !b.Before(a) && !b.After(now) {
			intervals = append(intervals, map[string]interface{}{"from_id": messages[i-1]["id"], "to_id": messages[i]["id"], "minutes": roundMinutes(b.Sub(a))})
		}
	}
	result := map[string]interface{}{"current_time": now.Format(time.RFC3339), "timezone": "Asia/Shanghai", "current_period": period, "source": "本机时钟，生成请求时读取", "exact_intervals": intervals}
	if len(messages) > 0 {
		if last, ok := parseTimestamp(messages[len(messages)-1]["message_time"]); ok && !last.After(now) {
			result["latest_message_age_minutes"] = roundMinutes(now.Sub(last))
		}
	}
	return result
}
