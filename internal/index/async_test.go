package index_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/srerickson/ocfl-index/internal/index"
)

func TestScheduler(t *testing.T) {
	task := func(_ context.Context, w io.Writer) error {
		time.Sleep(25 * time.Millisecond)
		return nil
	}
	sch := index.NewAsync(context.Background())
	if err := sch.TryNow("sleeping", task); err != nil {
		t.Fatal(err)
	}
	if err := sch.TryNow("snoozing", task); !errors.Is(err, index.ErrAsyncNotReady) {
		t.Fatal("expected ErrBusy, got", err)
	}
	sch.Close()
	sch.Wait()
}
