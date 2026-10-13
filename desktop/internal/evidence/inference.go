package evidence

import "time"

// Message retains unknown fields so legacy exports survive a read/write cycle.
type Message map[string]interface{}

type timeAnchor struct {
	start, end time.Time
	id         interface{}
}

func parseTimestamp(value interface{}) (time.Time, bool) {
	s, ok := value.(string)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, minuteFormat} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func CleanContext(value interface{}) *TimeContext {
	var c TimeContext
	switch v := value.(type) {
	case *TimeContext:
		if v == nil {
			return nil
		}
		c = *v
	case map[string]interface{}:
		c.Label, _ = v["label"].(string)
		c.AnchorTime, _ = v["anchor_time"].(string)
		c.Precision, _ = v["precision"].(string)
		c.DateBasis, _ = v["date_basis"].(string)
	default:
		return nil
	}
	if c.Precision != "group" && c.Precision != "day" {
		return nil
	}
	t, ok := parseTimestamp(c.AnchorTime)
	if !ok || len([]rune(c.Label)) > 80 {
		return nil
	}
	c.AnchorTime = t.In(China).Format(minuteFormat)
	c.Source = "ocr"
	switch c.DateBasis {
	case "today_inferred", "relative_to_capture_date", "weekday_inferred", "explicit_date", "year_inferred":
	default:
		c.DateBasis = "unknown"
	}
	return &c
}

func InferMissingTimes(messages []Message) []Message {
	anchors := make([]*timeAnchor, len(messages))
	following := make([]*timeAnchor, len(messages))
	for i, row := range messages {
		t, ok := parseTimestamp(row["message_time"])
		end := t
		if !ok {
			if c := CleanContext(row["time_context"]); c != nil {
				t, ok = parseTimestamp(c.AnchorTime)
				end = t
				if c.Precision == "day" {
					end = t.Add(24*time.Hour - time.Minute)
				}
			}
		}
		if ok {
			anchors[i] = &timeAnchor{t, end, row["id"]}
		}
	}
	var right, left *timeAnchor
	for i := len(messages) - 1; i >= 0; i-- {
		following[i] = right
		if anchors[i] != nil {
			right = anchors[i]
		}
	}
	result := make([]Message, len(messages))
	for i, original := range messages {
		row := Message{}
		for k, v := range original {
			if k != "inferred_time" {
				row[k] = v
			}
		}
		right = following[i]
		if anchors[i] == nil && row["time_inference_disabled"] != true && (left != nil || right != nil) && !(left != nil && right != nil && left.start.After(right.end)) {
			var earliest, latest interface{}
			ids := []interface{}{}
			precision := "range"
			if left != nil {
				earliest = left.start.Format(minuteFormat)
				ids = append(ids, left.id)
			} else {
				precision = "before"
			}
			if right != nil {
				latest = right.end.Format(minuteFormat)
				ids = append(ids, right.id)
			} else {
				precision = "after"
			}
			row["inferred_time"] = map[string]interface{}{"earliest": earliest, "latest": latest, "source_ids": ids, "precision": precision, "source": "neighbor_estimate", "note": "按已采集的相邻时间推算，不代表精确发送时间；历史可能不完整或采集顺序不同"}
		}
		result[i] = row
		if anchors[i] != nil {
			left = anchors[i]
		}
	}
	return result
}
