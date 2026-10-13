package memory

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var scopeLimits = map[string]int{"user": 30, "object": 15, "relationship": 10, "event": 20, "hypothesis": 5}

func text(raw Row, field string, limit int, required bool) (string, error) {
	value := ""
	if raw[field] != nil {
		var ok bool
		value, ok = raw[field].(string)
		if !ok {
			return "", fmt.Errorf("%s 必须是字符串", field)
		}
	}
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", fmt.Errorf("%s 不能为空", field)
	}
	if len([]rune(value)) > limit {
		return "", fmt.Errorf("%s 超过 %d 字符", field, limit)
	}
	for _, char := range value {
		if char < 32 && char != '\t' && char != '\n' {
			return "", fmt.Errorf("%s 包含控制字符", field)
		}
	}
	return value, nil
}

func ValidateDelta(raw Row) (Row, error) {
	result := Row{}
	for field, limit := range map[string]int{"scope": 32, "subject_id": 64, "field": 64, "value": 200, "source_type": 32, "source_ref": 200, "occurred_at": 64, "confidence": 16} {
		if field == "confidence" && raw[field] == nil {
			result[field] = "medium"
			continue
		}
		value, err := text(raw, field, limit, field != "source_ref" && field != "occurred_at")
		if err != nil {
			return nil, err
		}
		result[field] = value
	}
	scope := result["scope"].(string)
	source := result["source_type"].(string)
	if scopeLimits[scope] == 0 {
		return nil, errors.New("不支持的 scope")
	}
	if !oneOf(source, "user_explicit", "user_report", "chatlab", "tool", "assistant_inference") {
		return nil, errors.New("不支持的 source_type")
	}
	if !oneOf(result["confidence"].(string), "high", "medium", "low") {
		return nil, errors.New("不支持的 confidence")
	}
	if scope == "user" && source != "user_explicit" {
		return nil, errors.New("用户稳定档案只接受用户明确陈述")
	}
	if (scope == "object" || scope == "relationship") && !oneOf(source, "user_explicit", "user_report") {
		return nil, errors.New("对象事实和关系快照只接受用户明确陈述或转述")
	}
	if oneOf(source, "chatlab", "tool", "assistant_inference") && !oneOf(scope, "event", "hypothesis") {
		return nil, errors.New("外部工具和模型推断只能写入事件或假设")
	}
	if source == "assistant_inference" && scope != "hypothesis" {
		return nil, errors.New("模型推断只能写入带置信度的假设")
	}
	if result["occurred_at"] == "" {
		result["occurred_at"] = nil
	}
	return result, nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

const insertRow = `INSERT OR REPLACE INTO memories(id,scope,subject_id,field,value,source_type,source_ref,occurred_at,observed_at,confidence,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`

func restore(q executor, row Row) error {
	_, err := q.Exec(insertRow, row["id"], row["scope"], row["subject_id"], row["field"], row["value"], row["source_type"], row["source_ref"], row["occurred_at"], row["observed_at"], row["confidence"], row["status"], row["created_at"], row["updated_at"])
	return err
}

func upsert(q executor, row Row) error {
	statement := strings.Replace(insertRow, "INSERT OR REPLACE", "INSERT", 1) + ` ON CONFLICT(id) DO UPDATE SET value=excluded.value,source_type=excluded.source_type,source_ref=excluded.source_ref,occurred_at=excluded.occurred_at,observed_at=excluded.observed_at,confidence=excluded.confidence,status=excluded.status,updated_at=excluded.updated_at`
	_, err := q.Exec(statement, row["id"], row["scope"], row["subject_id"], row["field"], row["value"], row["source_type"], row["source_ref"], row["occurred_at"], row["observed_at"], row["confidence"], row["status"], row["created_at"], row["updated_at"])
	return err
}

func prune(tx *sql.Tx, delta Row) ([]Row, error) {
	removed := []Row{}
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM memories WHERE scope=? AND subject_id=? AND status='active'", delta["scope"], delta["subject_id"]).Scan(&count); err != nil {
		return nil, err
	}
	scope := delta["scope"].(string)
	excess := count - scopeLimits[scope]
	if excess > 0 {
		if scope != "event" && scope != "hypothesis" {
			return nil, fmt.Errorf("%s 档案已达 %d 条上限，请先合并或删除旧字段", scope, scopeLimits[scope])
		}
		rows, err := queryRows(tx, "SELECT * FROM memories WHERE scope=? AND subject_id=? AND status='active' ORDER BY updated_at ASC LIMIT ?", scope, delta["subject_id"], excess)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if _, err = tx.Exec("DELETE FROM memories WHERE id=?", row["id"]); err != nil {
				return nil, err
			}
			removed = append(removed, row)
		}
	}
	if err := tx.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count); err != nil {
		return nil, err
	}
	if count > 200 {
		rows, err := queryRows(tx, "SELECT * FROM memories WHERE scope IN ('event','hypothesis') ORDER BY updated_at ASC LIMIT ?", count-200)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if _, err = tx.Exec("DELETE FROM memories WHERE id=?", row["id"]); err != nil {
				return nil, err
			}
			removed = append(removed, row)
		}
		if err = tx.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count); err != nil {
			return nil, err
		}
		if count > 200 {
			return nil, errors.New("长期记忆已达上限，请先归档或删除旧对象")
		}
	}
	return removed, nil
}

func (s *Store) Apply(raw Row) (Row, error) {
	delta, err := ValidateDelta(raw)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open(false)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = enabled(tx); err != nil {
		return nil, err
	}
	var before []Row
	if delta["scope"] == "event" {
		before, err = queryRows(tx, "SELECT * FROM memories WHERE scope='event' AND subject_id=? AND field=? AND occurred_at IS ? AND status='active' ORDER BY updated_at DESC,rowid DESC", delta["subject_id"], delta["field"], delta["occurred_at"])
	} else {
		before, err = queryRows(tx, "SELECT * FROM memories WHERE scope=? AND subject_id=? AND field=? AND status='active' LIMIT 1", delta["scope"], delta["subject_id"], delta["field"])
	}
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05+00:00")
	row := Row{}
	for k, v := range delta {
		row[k] = v
	}
	row["id"] = id()
	row["created_at"] = timestamp
	wasUpdate := len(before) > 0
	if wasUpdate {
		row["id"] = before[0]["id"]
		row["created_at"] = before[0]["created_at"]
		for _, duplicate := range before[1:] {
			if _, err = tx.Exec("DELETE FROM memories WHERE id=?", duplicate["id"]); err != nil {
				return nil, err
			}
		}
	}
	row["observed_at"] = timestamp
	row["updated_at"] = timestamp
	row["status"] = "active"
	if err = upsert(tx, row); err != nil {
		return nil, err
	}
	removed, err := prune(tx, delta)
	if err != nil {
		return nil, err
	}
	before = append(before, removed...)
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal([]Row{row})
	operation := id()
	if _, err = tx.Exec("INSERT INTO operations(op_id,created_at,action,before_json,after_json) VALUES(?,?,'apply',?,?)", operation, timestamp, string(beforeJSON), string(afterJSON)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec("DELETE FROM operations WHERE op_id NOT IN (SELECT op_id FROM operations ORDER BY created_at DESC,rowid DESC LIMIT 20)"); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	pruned := len(before)
	if wasUpdate {
		pruned--
	}
	return Row{"ok": true, "op_id": operation, "memory": row, "pruned": pruned}, nil
}
