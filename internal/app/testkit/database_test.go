package testkit

import (
	"context"
	"testing"
	"time"
)

func TestOpenFileWaitsForOverlappingWriters(t *testing.T) {
	database := OpenFile(t)
	first, err := database.Conn.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback()
	if _, err := first.ExecContext(context.Background(), `INSERT INTO projects (name, total_hours, start_date, end_date) VALUES ('first', 1, '2026-01-01', '2026-01-01')`); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		second, err := database.Conn.BeginTx(context.Background(), nil)
		if err != nil {
			done <- err
			return
		}
		_, err = second.ExecContext(context.Background(), `INSERT INTO projects (name, total_hours, start_date, end_date) VALUES ('second', 1, '2026-01-01', '2026-01-01')`)
		if err == nil {
			err = second.Commit()
		} else {
			_ = second.Rollback()
		}
		done <- err
	}()

	<-started
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second writer did not finish")
	}
}
