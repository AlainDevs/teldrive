package http_range

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name   string
		header string
		size   int64
		start  int64
		end    int64
	}{
		{name: "bounded", header: "bytes=10-19", size: 100, start: 10, end: 19},
		{name: "open ended", header: "bytes=90-", size: 100, start: 90, end: 99},
		{name: "suffix", header: "bytes=-10", size: 100, start: 90, end: 99},
		{name: "bounded clamped to size", header: "bytes=90-200", size: 100, start: 90, end: 99},
		{name: "suffix clamped to size", header: "bytes=-200", size: 100, start: 0, end: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, err := Parse(tt.header, tt.size)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if len(ranges) != 1 || ranges[0].Start != tt.start || ranges[0].End != tt.end {
				t.Fatalf("Parse(%q) = %+v, want %d-%d", tt.header, ranges, tt.start, tt.end)
			}
		})
	}
}

func TestParseRejectsMalformedRanges(t *testing.T) {
	for _, header := range []string{
		"items=0-1",
		"bytes=1",
		"bytes=",
		"bytes=--",
		"bytes=1-2-3",
		"bytes=abc-5",
		"bytes=5-abc",
		"bytes=10-5",
		"bytes=-0",
		"bytes=,0-1",
		"bytes=0-1,",
		"bytes=9223372036854775808-",
	} {
		t.Run(header, func(t *testing.T) {
			_, err := Parse(header, 100)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Parse(%q) error = %v, want ErrInvalid", header, err)
			}
		})
	}
}

func TestParseReturnsNoOverlap(t *testing.T) {
	_, err := Parse("bytes=100-200", 100)
	if !errors.Is(err, ErrNoOverlap) {
		t.Fatalf("Parse error = %v, want ErrNoOverlap", err)
	}
}

func TestParseMultipleRanges(t *testing.T) {
	ranges, err := Parse("bytes=0-1,10-19", 100)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("Parse returned %d ranges, want 2", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 1 || ranges[1].Start != 10 || ranges[1].End != 19 {
		t.Fatalf("Parse returned %+v, want 0-1 and 10-19", ranges)
	}
}
