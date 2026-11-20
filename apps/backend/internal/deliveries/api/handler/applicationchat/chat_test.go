package applicationchat

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type unwrappingWriter struct {
	http.ResponseWriter
}

func (w *unwrappingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type deadlineWriter struct {
	http.ResponseWriter
	deadline time.Time
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestExtendWriteDeadline(t *testing.T) {
	inner := &deadlineWriter{ResponseWriter: httptest.NewRecorder()}
	outer := &unwrappingWriter{ResponseWriter: inner}

	extendWriteDeadline(outer, 45*time.Second)

	if inner.deadline.IsZero() {
		t.Fatal("expected the write deadline to be set through the Unwrap chain")
	}
	remaining := time.Until(inner.deadline)
	if remaining < 44*time.Second || remaining > 46*time.Second {
		t.Fatalf("unexpected deadline: %s from now", remaining)
	}
}
