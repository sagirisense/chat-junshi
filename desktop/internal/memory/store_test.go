package memory

import (
	"fmt"
	"os"
	"testing"
)

func delta(scope, subject, field, value string) Row {
	return Row{"scope": scope, "subject_id": subject, "field": field, "value": value, "source_type": "user_explicit", "source_ref": "虚构测试", "confidence": "high"}
}

func TestConsentIsolationUndoAndRevoke(t *testing.T) {
	s := New(t.TempDir())
	status, err := s.Status()
	if err != nil || status["exists"] != false {
		t.Fatal(status, err)
	}
	if err = s.Enable(false); err == nil {
		t.Fatal("enabled without consent")
	}
	if err = s.Enable(true); err != nil {
		t.Fatal(err)
	}
	first, err := s.Apply(delta("object", "a", "label", "虚构对象A"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(delta("object", "b", "label", "虚构对象B")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(delta("user", "user", "boundary", "不接受强迫")); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Rows("a")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row["subject_id"] == "b" {
			t.Fatal("cross-object leak")
		}
	}
	if err = s.Pause(true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(delta("event", "a", "key_event", "测试")); err == nil {
		t.Fatal("wrote while paused")
	}
	if _, err = s.Context("a", 4000); err == nil {
		t.Fatal("read context while paused")
	}
	if err = s.Pause(false); err != nil {
		t.Fatal(err)
	}
	changed, err := s.Apply(delta("object", "a", "label", "新名称"))
	if err != nil {
		t.Fatal(err)
	}
	if first["memory"].(Row)["id"] != changed["memory"].(Row)["id"] {
		t.Fatal("changed stable ID")
	}
	if _, err = s.Undo(""); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Rows("a")
	if err != nil || rows[1]["value"] != "虚构对象A" {
		t.Fatal(rows, err)
	}
	if err = s.Forget("a", true); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Rows("a")
	if err != nil || len(rows) != 1 || rows[0]["scope"] != "user" {
		t.Fatal(rows, err)
	}
	if _, err = s.Undo(""); err == nil {
		t.Fatal("forget can be undone")
	}
	if err = s.Revoke(true, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Context("b", 4000); err == nil {
		t.Fatal("used memory after revoked consent")
	}
	if err = s.Revoke(true, true); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(s.Path); !os.IsNotExist(err) {
		t.Fatal("database not removed")
	}
}

func TestPruningAndQuotaRollback(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Enable(true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 21; i++ {
		if _, err := s.Apply(delta("event", "a", fmt.Sprintf("event-%d", i), "虚构事件")); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.Rows("a")
	if err != nil || len(rows) != 20 {
		t.Fatal(len(rows), err)
	}
	status, err := s.Status()
	if err != nil || status["undo_count"] != 20 {
		t.Fatal(status, err)
	}
	if _, err = s.Undo(""); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Rows("a")
	if err != nil || len(rows) != 20 {
		t.Fatal("pruned event not restored", len(rows), err)
	}
	for i := 0; i < 15; i++ {
		if _, err = s.Apply(delta("object", "b", fmt.Sprintf("field-%d", i), "虚构档案")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Apply(delta("object", "b", "overflow", "超限")); err == nil {
		t.Fatal("accepted quota overflow")
	}
	rows, err = s.Rows("b")
	if err != nil || len(rows) != 15 {
		t.Fatal("quota failure not rolled back", len(rows), err)
	}
}

func TestSourceEligibilityAndContextBudget(t *testing.T) {
	for _, scope := range []string{"user", "object", "relationship", "event"} {
		row := delta(scope, "a", "field", "测试")
		row["source_type"] = "assistant_inference"
		if _, err := ValidateDelta(row); err == nil {
			t.Fatal("inference accepted as fact", scope)
		}
	}
	s := New(t.TempDir())
	if err := s.Enable(true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := s.Apply(delta("event", "a", fmt.Sprintf("event-%d", i), "虚构事件")); err != nil {
			t.Fatal(err)
		}
	}
	context, err := s.Context("a", 8000)
	if err != nil || context["count"] != 8 {
		t.Fatal(context, err)
	}
	context, err = s.Context("a", 500)
	if err != nil || context["count"].(int) > 1 {
		t.Fatal(context, err)
	}
}
