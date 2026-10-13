package memory

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Row map[string]interface{}
type Store struct {
	Path string
	mu   sync.Mutex
}

func New(directory string) *Store { return &Store{Path: filepath.Join(directory, "memory.sqlite3")} }

const schema = `CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS memories(id TEXT PRIMARY KEY,scope TEXT NOT NULL,subject_id TEXT NOT NULL,field TEXT NOT NULL,value TEXT NOT NULL,source_type TEXT NOT NULL,source_ref TEXT NOT NULL DEFAULT '',occurred_at TEXT,observed_at TEXT NOT NULL,confidence TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'active',created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_memories_lookup ON memories(scope,subject_id,status,updated_at);
CREATE TABLE IF NOT EXISTS operations(op_id TEXT PRIMARY KEY,created_at TEXT NOT NULL,action TEXT NOT NULL,before_json TEXT NOT NULL,after_json TEXT NOT NULL);`

func (s *Store) open(create bool) (*sql.DB, error) {
	if _, err := os.Stat(s.Path); err != nil && !create {
		return nil, errors.New("尚未启用长期记忆")
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", s.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", schema} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

type queryer interface {
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}
type executor interface {
	Exec(string, ...interface{}) (sql.Result, error)
}

func setting(q queryer, key string) (string, error) {
	var value string
	err := q.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}
func setSetting(q executor, key, value string) error {
	_, err := q.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}
func enabled(q queryer) error {
	consent, err := setting(q, "consent_enabled")
	if err != nil {
		return err
	}
	if consent != "true" {
		return errors.New("长期记忆尚未获得用户同意")
	}
	paused, err := setting(q, "paused")
	if err != nil {
		return err
	}
	if paused == "true" {
		return errors.New("长期记忆当前已暂停")
	}
	return nil
}

func queryRows(q queryer, statement string, args ...interface{}) ([]Row, error) {
	rows, err := q.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := []Row{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := Row{}
		for i, column := range columns {
			row[column] = values[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) Status() (Row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Path); errors.Is(err, os.ErrNotExist) {
		return Row{"exists": false, "consent_enabled": false, "paused": false, "policy_version": "1", "path": s.Path}, nil
	}
	db, err := s.open(false)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	result := Row{"exists": true, "path": s.Path}
	for _, key := range []string{"consent_enabled", "paused", "consent_at", "policy_version"} {
		value, err := setting(db, key)
		if err != nil {
			return nil, err
		}
		switch key {
		case "consent_enabled", "paused":
			result[key] = value == "true"
		case "consent_at":
			result[key] = nil
			if value != "" {
				result[key] = value
			}
		default:
			if value == "" {
				value = "1"
			}
			result[key] = value
		}
	}
	for key, table := range map[string]string{"memory_count": "memories", "undo_count": "operations"} {
		var count int
		if err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return nil, err
		}
		result[key] = count
	}
	return result, nil
}

func (s *Store) Enable(confirm bool) error {
	if !confirm {
		return errors.New("需明确同意保存精简关系档案")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open(true)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	consentAt, err := setting(tx, "consent_at")
	if err != nil {
		return err
	}
	if consentAt == "" {
		if err = setSetting(tx, "consent_at", time.Now().UTC().Format("2006-01-02T15:04:05+00:00")); err != nil {
			return err
		}
	}
	for key, value := range map[string]string{"consent_enabled": "true", "paused": "false", "policy_version": "1"} {
		if err = setSetting(tx, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Pause(paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open(false)
	if err != nil {
		return err
	}
	defer db.Close()
	consent, err := setting(db, "consent_enabled")
	if err != nil {
		return err
	}
	if consent != "true" {
		return errors.New("长期记忆尚未启用")
	}
	value := "false"
	if paused {
		value = "true"
	}
	return setSetting(db, "paused", value)
}

func list(q queryer, subject string) ([]Row, error) {
	if subject == "" {
		return queryRows(q, "SELECT * FROM memories WHERE status='active' ORDER BY updated_at DESC")
	}
	return queryRows(q, "SELECT * FROM memories WHERE subject_id IN ('user',?) AND status='active' ORDER BY CASE scope WHEN 'user' THEN 0 WHEN 'object' THEN 1 WHEN 'relationship' THEN 2 WHEN 'event' THEN 3 ELSE 4 END,updated_at DESC", subject)
}

func (s *Store) Rows(subject string) ([]Row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Path); errors.Is(err, os.ErrNotExist) {
		return []Row{}, nil
	}
	db, err := s.open(false)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return list(db, subject)
}

func (s *Store) Context(subject string, maxChars int) (Row, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open(false)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err = enabled(db); err != nil {
		return nil, err
	}
	rows, err := list(db, subject)
	if err != nil {
		return nil, err
	}
	if maxChars < 500 || maxChars > 8000 {
		return nil, errors.New("记忆上下文长度无效")
	}
	ordered := []Row{}
	events := []Row{}
	for _, row := range rows {
		if subject != "" && row["scope"] == "event" {
			if len(events) < 8 {
				events = append(events, row)
			}
		} else {
			ordered = append(ordered, row)
		}
	}
	ordered = append(ordered, events...)
	compact := []Row{}
	for _, row := range ordered {
		value := Row{}
		for _, field := range []string{"id", "scope", "subject_id", "field", "value", "occurred_at", "observed_at", "confidence", "source_type", "source_ref"} {
			value[field] = row[field]
		}
		compact = append(compact, value)
	}
	for len(compact) > 0 {
		encoded, _ := json.Marshal(compact) // Python's default JSON includes separator spaces.
		spaces := 0
		for _, row := range compact {
			spaces += len(row)*2 - 1
		}
		spaces += len(compact) - 1
		if len([]rune(string(encoded)))+spaces <= maxChars {
			break
		}
		compact = compact[:len(compact)-1]
	}
	return Row{"count": len(compact), "max_chars": maxChars, "memories": compact}, nil
}

func id() string { return strings.ReplaceAll(uuid.NewString(), "-", "") }
