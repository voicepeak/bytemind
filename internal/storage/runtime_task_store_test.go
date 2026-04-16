package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corepkg "bytemind/internal/core"
	runtimepkg "bytemind/internal/runtime"
)

func TestRuntimeTaskEventAdapterPersistsEventsAndLogsToUnifiedTaskLog(t *testing.T) {
	store, err := NewFileTaskStore(t.TempDir(), NewInMemoryLocker())
	if err != nil {
		t.Fatal(err)
	}
	adapter, ok := NewRuntimeTaskEventAdapter(store).(*RuntimeTaskEventAdapter)
	if !ok {
		t.Fatal("expected runtime task event adapter")
	}

	taskID := corepkg.TaskID("task-unified")
	eventTime := time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC)
	logTime := eventTime.Add(2 * time.Second)

	event := runtimepkg.TaskEvent{
		Type:      runtimepkg.TaskEventStatus,
		Offset:    3,
		TaskID:    taskID,
		SessionID: corepkg.SessionID("session-1"),
		TraceID:   corepkg.TraceID("trace-1"),
		Status:    corepkg.TaskPending,
		Attempt:   2,
		Payload:   []byte(`{"step":1}`),
		Metadata: map[string]string{
			"owner": "runtime",
		},
		Timestamp: eventTime,
	}
	if err := adapter.AppendTaskEvent(context.Background(), event); err != nil {
		t.Fatalf("AppendTaskEvent failed: %v", err)
	}

	logEntry := runtimepkg.TaskLogEntry{
		TaskID:    taskID,
		Offset:    5,
		Payload:   []byte("status=pending"),
		Timestamp: logTime,
	}
	if err := adapter.AppendTaskLog(context.Background(), taskID, logEntry); err != nil {
		t.Fatalf("AppendTaskLog failed: %v", err)
	}

	records, _, err := store.ReadLogFrom(context.Background(), taskID, 0, 10)
	if err != nil {
		t.Fatalf("ReadLogFrom failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 persisted records, got %d", len(records))
	}
	if records[0].Type != TaskRecordTypeTaskEventStatus {
		t.Fatalf("expected first record type %q, got %q", TaskRecordTypeTaskEventStatus, records[0].Type)
	}
	if records[1].Type != TaskRecordTypeTaskLog {
		t.Fatalf("expected second record type %q, got %q", TaskRecordTypeTaskLog, records[1].Type)
	}
	if !records[0].CreatedAt.Equal(eventTime) {
		t.Fatalf("expected event timestamp %s, got %s", eventTime, records[0].CreatedAt)
	}
	if !records[1].CreatedAt.Equal(logTime) {
		t.Fatalf("expected log timestamp %s, got %s", logTime, records[1].CreatedAt)
	}

	var persistedEvent runtimepkg.TaskEvent
	if err := json.Unmarshal(records[0].Payload, &persistedEvent); err != nil {
		t.Fatalf("event payload unmarshal failed: %v", err)
	}
	if persistedEvent.TaskID != taskID {
		t.Fatalf("expected event task id %q, got %q", taskID, persistedEvent.TaskID)
	}
	if persistedEvent.Status != corepkg.TaskPending {
		t.Fatalf("expected event status %q, got %q", corepkg.TaskPending, persistedEvent.Status)
	}
	if got := persistedEvent.Metadata["owner"]; got != "runtime" {
		t.Fatalf("expected event metadata owner %q, got %q", "runtime", got)
	}

	var persistedLog runtimepkg.TaskLogEntry
	if err := json.Unmarshal(records[1].Payload, &persistedLog); err != nil {
		t.Fatalf("log payload unmarshal failed: %v", err)
	}
	if persistedLog.TaskID != taskID {
		t.Fatalf("expected log task id %q, got %q", taskID, persistedLog.TaskID)
	}
	if string(persistedLog.Payload) != "status=pending" {
		t.Fatalf("expected log payload %q, got %q", "status=pending", string(persistedLog.Payload))
	}
}

func TestRuntimeTaskEventAdapterSanitizesMissingAndUnsafeTaskIDs(t *testing.T) {
	store, err := NewFileTaskStore(t.TempDir(), NewInMemoryLocker())
	if err != nil {
		t.Fatal(err)
	}
	adapter, ok := NewRuntimeTaskEventAdapter(store).(*RuntimeTaskEventAdapter)
	if !ok {
		t.Fatal("expected runtime task event adapter")
	}

	if err := adapter.AppendTaskLog(context.Background(), "", runtimepkg.TaskLogEntry{
		Payload: []byte("fallback"),
	}); err != nil {
		t.Fatalf("AppendTaskLog fallback failed: %v", err)
	}
	if err := adapter.AppendTaskEvent(context.Background(), runtimepkg.TaskEvent{
		Type:   runtimepkg.TaskEventStatus,
		TaskID: corepkg.TaskID("task/with/slash"),
		Status: corepkg.TaskPending,
	}); err != nil {
		t.Fatalf("AppendTaskEvent sanitize failed: %v", err)
	}

	unknownRecords, _, err := store.ReadLogFrom(context.Background(), corepkg.TaskID(unknownTaskID), 0, 10)
	if err != nil {
		t.Fatalf("ReadLogFrom unknown task failed: %v", err)
	}
	if len(unknownRecords) != 1 {
		t.Fatalf("expected one fallback record for unknown task id, got %d", len(unknownRecords))
	}
	if unknownRecords[0].Type != TaskRecordTypeTaskLog {
		t.Fatalf("expected fallback record type %q, got %q", TaskRecordTypeTaskLog, unknownRecords[0].Type)
	}

	sanitizedRecords, _, err := store.ReadLogFrom(context.Background(), corepkg.TaskID("task_with_slash"), 0, 10)
	if err != nil {
		t.Fatalf("ReadLogFrom sanitized task failed: %v", err)
	}
	if len(sanitizedRecords) != 1 {
		t.Fatalf("expected one sanitized event record, got %d", len(sanitizedRecords))
	}
	if sanitizedRecords[0].Type != TaskRecordTypeTaskEventStatus {
		t.Fatalf("expected sanitized event type %q, got %q", TaskRecordTypeTaskEventStatus, sanitizedRecords[0].Type)
	}
}
