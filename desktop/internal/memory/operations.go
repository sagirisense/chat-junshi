package memory

import (
	"encoding/json"
	"errors"
	"os"
)

func (s *Store) Undo(operation string) (Row, error) {
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
	query := "SELECT * FROM operations ORDER BY created_at DESC,rowid DESC LIMIT 1"
	args := []interface{}{}
	if operation != "" {
		query = "SELECT * FROM operations WHERE op_id=?"
		args = append(args, operation)
	}
	rows, err := queryRows(tx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("没有可撤销的记忆更新")
	}
	var before, after []Row
	if err = json.Unmarshal([]byte(rows[0]["before_json"].(string)), &before); err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(rows[0]["after_json"].(string)), &after); err != nil {
		return nil, err
	}
	for _, row := range append(append([]Row{}, before...), after...) {
		if _, err = tx.Exec("DELETE FROM memories WHERE id=?", row["id"]); err != nil {
			return nil, err
		}
	}
	for _, row := range before {
		if err = restore(tx, row); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec("DELETE FROM operations WHERE op_id=?", rows[0]["op_id"]); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return Row{"ok": true, "undone_op_id": rows[0]["op_id"]}, nil
}

func (s *Store) Forget(subject string, confirm bool) error {
	if !confirm {
		return errors.New("请确认要忘记当前对象")
	}
	if _, err := text(Row{"subject_id": subject}, "subject_id", 64, true); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open(false)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM memories WHERE subject_id=?", subject); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM operations"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	_, err = db.Exec("VACUUM")
	return err
}

func (s *Store) Revoke(confirm, deleteAll bool) error {
	if !confirm {
		return errors.New("请明确确认撤回记忆同意")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if deleteAll {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Remove(s.Path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	}
	db, err := s.open(false)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = setSetting(tx, "consent_enabled", "false"); err != nil {
		return err
	}
	if err = setSetting(tx, "paused", "true"); err != nil {
		return err
	}
	return tx.Commit()
}
