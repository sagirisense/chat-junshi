package service

import (
	"junshi-desktop/internal/memory"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationBackupPreservesLegacyData(t *testing.T) {
	root := t.TempDir()
	store := memory.New(filepath.Join(root, "data", "relationship-memory"))
	if err := store.Enable(true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply(memory.Row{"scope": "object", "subject_id": "synthetic", "field": "label", "value": "虚构旧数据", "source_type": "user_explicit", "confidence": "high"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "synthetic.json"), []byte(`{"version":1,"title":"synthetic","messages":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := BackupBeforeMigration(root); err != nil {
		t.Fatal(err)
	}
	if err := BackupBeforeMigration(root); err != nil {
		t.Fatal(err)
	}
	directories, err := filepath.Glob(filepath.Join(root, "data", "migration-backup-go-v1", "*"))
	if err != nil || len(directories) != 1 {
		t.Fatal(directories, err)
	}
	backed := memory.New(directories[0])
	rows, err := backed.Rows("synthetic")
	if err != nil || len(rows) != 1 || rows[0]["value"] != "虚构旧数据" {
		t.Fatal(rows, err)
	}
	if _, err = os.Stat(filepath.Join(directories[0], "synthetic.json")); err != nil {
		t.Fatal(err)
	}
}
