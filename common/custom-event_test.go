package common

import (
	"errors"
	"io"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }
func (failingWriter) writeString(s string) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteDataReturnsWriteError(t *testing.T) {
	if err := writeData(failingWriter{}, "hello"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected writeData to return write error, got %v", err)
	}
}
