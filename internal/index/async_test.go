package index_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/srerickson/ocfl-index/internal/index"
)

func TestScheduler(t *testing.T) {
	var slept bool
	task := func(_ context.Context, w io.Writer) error {
		time.Sleep(25 * time.Millisecond)
		slept = true

		return nil
	}
	sch := index.NewAsync(context.Background())
	added, doneErr := sch.TryNow("sleeping", task)
	if !added {
		t.Fatal("expected task to be added")
	}
	if added, _ := sch.TryNow("snoozing", task); added {
		t.Fatal("expected task to not be added")
	}
	// block until running task is complete
	err := <-doneErr
	if err != nil {
		t.Fatal("expected no error")
	}
	if !slept {
		t.Fatal("should have slept")
	}
	sch.Close()
	sch.Wait()
}
