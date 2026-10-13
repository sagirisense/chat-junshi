package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// BackupBeforeMigration is called once before the Go service opens legacy data.
// VACUUM INTO copies a consistent SQLite snapshot, including committed WAL data.
func BackupBeforeMigration(root string) error {
	dataRoot := filepath.Join(root, "data")
	marker := filepath.Join(dataRoot, "go-migration-v1.json")
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dataRoot, "*.json"))
	if err != nil {
		return err
	}
	database := filepath.Join(dataRoot, "relationship-memory", "memory.sqlite3")
	_, dbErr := os.Stat(database)
	if dbErr != nil && !errors.Is(dbErr, os.ErrNotExist) {
		return dbErr
	}
	if len(files) == 0 && errors.Is(dbErr, os.ErrNotExist) {
		return nil
	}
	backup := filepath.Join(dataRoot, "migration-backup-go-v1", time.Now().UTC().Format("20060102T150405.000000000"))
	if err = os.MkdirAll(backup, 0700); err != nil {
		return err
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(backup, filepath.Base(path)), data, 0600); err != nil {
			return err
		}
	}
	if dbErr == nil {
		absolute, err := filepath.Abs(database)
		if err != nil {
			return err
		}
		uriPath := filepath.ToSlash(absolute)
		if filepath.VolumeName(absolute) != "" {
			uriPath = "/" + uriPath
		}
		uri := url.URL{Scheme: "file", Path: uriPath}
		query := url.Values{"mode": []string{"ro"}}
		uri.RawQuery = query.Encode()
		db, err := sql.Open("sqlite", uri.String())
		if err != nil {
			return err
		}
		_, err = db.Exec("VACUUM INTO ?", filepath.Join(backup, "memory.sqlite3"))
		closeErr := db.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	encoded, _ := json.Marshal(map[string]interface{}{"version": 1, "backup": backup, "created_at": time.Now().UTC().Format(time.RFC3339)})
	return os.WriteFile(marker, encoded, 0600)
}
