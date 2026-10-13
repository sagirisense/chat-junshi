package evidence

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var China = time.FixedZone("Asia/Shanghai", 8*60*60)

const minuteFormat = "2006-01-02T15:04Z07:00"

var labelPattern = regexp.MustCompile(`^(?:(今天|昨天|前天|星期[一二三四五六日天]|周[一二三四五六日天])|(?:(\d{4})[年/.-])?(\d{1,2})[月/.-](\d{1,2})日?)?(凌晨|早上|上午|中午|下午|晚上)?(\d{1,2}):(\d{2})$`)
var datePattern = regexp.MustCompile(`^(?:(今天|昨天|前天|星期[一二三四五六日天]|周[一二三四五六日天])|(?:(\d{4})[年/.-])?(\d{1,2})[月/.-](\d{1,2})日?)$`)

type TimeContext struct {
	Label      string `json:"label"`
	AnchorTime string `json:"anchor_time"`
	Precision  string `json:"precision"`
	Source     string `json:"source"`
	DateBasis  string `json:"date_basis"`
}

func ParseLabel(label string, now time.Time) *TimeContext {
	now = now.In(China)
	text := strings.ReplaceAll(strings.Join(strings.Fields(label), ""), "：", ":")
	m := labelPattern.FindStringSubmatch(text)
	dayOnly := false
	if m == nil {
		m = datePattern.FindStringSubmatch(text)
		dayOnly = true
	}
	if m == nil {
		return nil
	}
	year, month, day := now.Date()
	basis := "today_inferred"
	d := time.Date(year, month, day, 0, 0, 0, 0, China)
	if m[1] != "" {
		if offset, ok := map[string]int{"今天": 0, "昨天": 1, "前天": 2}[m[1]]; ok {
			d = d.AddDate(0, 0, -offset)
			basis = "relative_to_capture_date"
		} else {
			last := []rune(m[1])
			weekday := strings.Index("一二三四五六日", strings.ReplaceAll(string(last[len(last)-1]), "天", "日")) / 3
			d = d.AddDate(0, 0, -((int(d.Weekday())+6)%7-weekday+7)%7)
			basis = "weekday_inferred"
		}
	} else if m[3] != "" {
		if m[2] != "" {
			year, _ = strconv.Atoi(m[2])
		}
		mon, _ := strconv.Atoi(m[3])
		day, _ = strconv.Atoi(m[4])
		month = time.Month(mon)
		d = time.Date(year, month, day, 0, 0, 0, 0, China)
		if d.Year() != year || d.Month() != month || d.Day() != day || year < 1 || year > 9999 {
			return nil
		}
		basis = "explicit_date"
		if m[2] == "" {
			basis = "year_inferred"
			if d.After(now) {
				d = d.AddDate(-1, 0, 0)
				if d.Month() != month || d.Day() != day {
					return nil
				}
			}
		}
	}
	hour, minute := 0, 0
	precision := "day"
	if !dayOnly {
		precision = "group"
		hour, _ = strconv.Atoi(m[6])
		minute, _ = strconv.Atoi(m[7])
		if m[5] != "" {
			if hour < 1 || hour > 12 {
				return nil
			}
			if m[5] == "下午" || m[5] == "晚上" || m[5] == "中午" {
				if hour < 12 {
					hour += 12
				}
			} else if hour == 12 {
				hour = 0
			}
		}
		if hour > 23 || minute > 59 {
			return nil
		}
	}
	d = d.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	return &TimeContext{strings.TrimSpace(label), d.Format(minuteFormat), precision, "ocr", basis}
}

func NormalizeManual(value string) (*string, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > 80 {
		return nil, errors.New("请填写有效的消息日期和时间（北京时间），或留空表示未知")
	}
	for _, layout := range []string{time.RFC3339Nano, minuteFormat, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04Z07:00", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, China); err == nil {
			normalized := parsed.In(China).Format(minuteFormat)
			return &normalized, nil
		}
	}
	return nil, errors.New("请填写有效的消息日期和时间（北京时间），或留空表示未知")
}
