package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This optional integration test exercises the bundled legacy implementation
// against a temporary database only. The regular Go tests need no Python.
func TestLegacyPythonReadWriteUndo(t *testing.T) {
	python := os.Getenv("JUNSHI_TEST_PYTHON")
	if python == "" {
		t.Skip("set JUNSHI_TEST_PYTHON to run legacy compatibility test")
	}
	script, err := filepath.Abs("../../../packaging/vendor/goutoujunshi/scripts/memory_store.py")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	s := New(directory)
	run := func(action string) {
		t.Helper()
		program := `import importlib.util,json,pathlib,sys,types
spec=importlib.util.spec_from_file_location('legacy_memory',sys.argv[1])
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
m.memory_dir=lambda:pathlib.Path(sys.argv[2]);m.emit=lambda value:None
if sys.argv[3]=='create':
 m.command_enable(types.SimpleNamespace(confirm=True))
 m.command_apply(types.SimpleNamespace(file=None,json=json.dumps({'scope':'object','subject_id':'synthetic','field':'label','value':'legacy value','source_type':'user_explicit','confidence':'high'})))
elif sys.argv[3]=='undo':
 m.command_undo(types.SimpleNamespace(op_id=None))
with m.connect() as db:
 row=db.execute("SELECT value FROM memories WHERE subject_id='synthetic'").fetchone()
 assert row['value']=='legacy value',row
`
		cmd := exec.Command(python, "-c", program, script, directory, action)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("legacy %s: %v\n%s", action, err, output)
		}
	}
	run("create")
	rows, err := s.Rows("synthetic")
	if err != nil || len(rows) != 1 || rows[0]["value"] != "legacy value" {
		t.Fatal(rows, err)
	}
	if _, err = s.Apply(delta("object", "synthetic", "label", "Go replacement")); err != nil {
		t.Fatal(err)
	}
	run("undo")
	rows, err = s.Rows("synthetic")
	if err != nil || rows[0]["value"] != "legacy value" {
		t.Fatal(rows, err)
	}
	if _, err = s.Undo(""); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Rows("synthetic")
	if err != nil || len(rows) != 0 {
		t.Fatal("Go failed to undo legacy operation", rows, err)
	}
}
