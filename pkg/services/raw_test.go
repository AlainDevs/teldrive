package services

import (
	"net/http"
	"testing"
	"time"
)

type deadlineResponseWriter struct {
	header    http.Header
	deadlines []time.Time
}

func (w *deadlineResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *deadlineResponseWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *deadlineResponseWriter) WriteHeader(int) {}

func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func TestStreamResponseWriterRefreshesWriteDeadline(t *testing.T) {
	underlying := &deadlineResponseWriter{}
	writer := &streamResponseWriter{
		ResponseWriter: underlying,
		controller:     http.NewResponseController(underlying),
		timeout:        time.Minute,
	}

	beforeFirstWrite := time.Now()
	if _, err := writer.Write([]byte("first")); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	firstDeadline := underlying.deadlines[0]
	if firstDeadline.Before(beforeFirstWrite.Add(writer.timeout)) {
		t.Fatalf("first deadline %v is earlier than expected", firstDeadline)
	}

	time.Sleep(time.Millisecond)
	if _, err := writer.Write([]byte("second")); err != nil {
		t.Fatalf("second write failed: %v", err)
	}
	secondDeadline := underlying.deadlines[1]
	if !secondDeadline.After(firstDeadline) {
		t.Fatalf("second deadline %v did not advance beyond %v", secondDeadline, firstDeadline)
	}
}
