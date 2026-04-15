package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryTaskManagerSubmitAndCancel(t *testing.T) {
	mgr := NewInMemoryTaskManager()
	id, err := mgr.Submit(context.Background(), TaskSpec{Name: "demo"})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected task id")
	}
	if err := mgr.Cancel(context.Background(), id, "test"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	task, err := mgr.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if task.Status != "killed" {
		t.Fatalf("expected killed status, got %s", task.Status)
	}
	if task.ErrorCode != ErrorCodeTaskCancelled {
		t.Fatalf("expected error code %q, got %q", ErrorCodeTaskCancelled, task.ErrorCode)
	}
}

func TestInMemoryTaskManagerWaitReturnsTerminalResult(t *testing.T) {
	mgr := NewInMemoryTaskManager()
	id, err := mgr.Submit(context.Background(), TaskSpec{Name: "demo"})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	results := make(chan TaskResult, 1)
	waitErrs := make(chan error, 1)
	go func() {
		result, err := mgr.Wait(context.Background(), id)
		if err != nil {
			waitErrs <- err
			return
		}
		results <- result
	}()

	time.Sleep(10 * time.Millisecond)
	if err := mgr.Cancel(context.Background(), id, "test"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	select {
	case err := <-waitErrs:
		t.Fatalf("Wait failed: %v", err)
	case result := <-results:
		if result.TaskID != id {
			t.Fatalf("expected task id %q, got %q", id, result.TaskID)
		}
		if result.Status != "killed" {
			t.Fatalf("expected killed status, got %s", result.Status)
		}
		if result.ErrorCode != ErrorCodeTaskCancelled {
			t.Fatalf("expected error code %q, got %q", ErrorCodeTaskCancelled, result.ErrorCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait timed out")
	}
}

func TestInMemoryTaskManagerWaitRespectsContextCancellation(t *testing.T) {
	mgr := NewInMemoryTaskManager()
	id, err := mgr.Submit(context.Background(), TaskSpec{Name: "demo"})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = mgr.Wait(ctx, id)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestInMemoryTaskManagerGetUnknownTaskReturnsTaskNotFound(t *testing.T) {
	mgr := NewInMemoryTaskManager()

	_, err := mgr.Get(context.Background(), "unknown-task")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
	if !hasErrorCode(err, ErrorCodeTaskNotFound) {
		t.Fatalf("expected error code %q, got %q", ErrorCodeTaskNotFound, errorCode(err))
	}
}
