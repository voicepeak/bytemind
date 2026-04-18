package runtime

import (
	"strings"
	"testing"
)

func TestTaskReportRecordsUniqueItemsInOrder(t *testing.T) {
	var report TaskReport
	report.RecordExecuted("list_files")
	report.RecordExecuted("read_file")
	report.RecordExecuted("list_files")
	report.RecordDenied("write_file")
	report.RecordPendingApproval("write_file")
	report.RecordPendingApproval("write_file")
	report.RecordSkippedDueToDependency("update_plan")

	if got, want := strings.Join(report.Executed, ","), "list_files,read_file"; got != want {
		t.Fatalf("unexpected executed tools: got=%q want=%q", got, want)
	}
	if got, want := strings.Join(report.Denied, ","), "write_file"; got != want {
		t.Fatalf("unexpected denied tools: got=%q want=%q", got, want)
	}
	if got, want := strings.Join(report.PendingApproval, ","), "write_file"; got != want {
		t.Fatalf("unexpected pending approval tools: got=%q want=%q", got, want)
	}
	if got, want := strings.Join(report.SkippedDueToDependency, ","), "update_plan"; got != want {
		t.Fatalf("unexpected skipped tools: got=%q want=%q", got, want)
	}
}

func TestTaskReportJSONAndEmpty(t *testing.T) {
	var report TaskReport
	if !report.IsEmpty() {
		t.Fatal("expected zero-value report to be empty")
	}
	if got := report.JSON(); got != "{}" {
		t.Fatalf("expected empty JSON object, got %q", got)
	}

	report.RecordExecuted("run_shell")
	if report.IsEmpty() {
		t.Fatal("expected non-empty report after recording an entry")
	}
	if got := report.JSON(); !strings.Contains(got, `"executed":["run_shell"]`) {
		t.Fatalf("expected executed payload in JSON, got %q", got)
	}
}
