package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tgdrive/teldrive/internal/api"
)

func TestStreamFile_MissingReturns404(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	token := loginAndGetToken(t, s, 9101, "stream-user-9101")

	nonexistentUUID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// HEAD missing file => 404
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.server.URL+"/files/"+nonexistentUUID.String()+"/content", nil)
	if err != nil {
		t.Fatalf("new HEAD request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", "access_token="+token)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("HEAD request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing file HEAD, got %d", resp.StatusCode)
	}

	// GET missing file => 404
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, s.server.URL+"/files/"+nonexistentUUID.String()+"/content", nil)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", "access_token="+token)

	resp, err = s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing file GET, got %d", resp.StatusCode)
	}
}

func TestStreamFile_InvalidRangeReturns416(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	token := loginAndGetToken(t, s, 9102, "stream-user-9102")
	client := s.newClientWithToken(token)

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910082), ChannelName: api.NewOptString("stream-range")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "range-test.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910082),
		Size:      api.NewOptInt64(100),
	})
	if err != nil {
		t.Fatalf("FilesCreate file failed: %v", err)
	}

	// GET with range beyond file size => 416 before streaming begins.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.server.URL+"/files/"+uuid.UUID(file.ID.Value).String()+"/content", nil)
	if err != nil {
		t.Fatalf("new HEAD request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", "access_token="+token)
	req.Header.Set("Range", "bytes=1000-2000")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("HEAD request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected 416 for invalid range, got %d", resp.StatusCode)
	}

	// Valid suffix range passes range validation. The fixture file has no backing
	// Telegram parts, so streaming setup now fails before response headers are sent.
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, s.server.URL+"/files/"+uuid.UUID(file.ID.Value).String()+"/content", nil)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", "access_token="+token)
	req.Header.Set("Range", "bytes=-5")

	resp, err = s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for fixture without streamable content, got %d", resp.StatusCode)
	}
}

func TestStreamFileHead_RangeErrors(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	token := loginAndGetToken(t, s, 9104, "stream-user-9104")
	client := s.newClientWithToken(token)
	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910084), ChannelName: api.NewOptString("stream-head-range")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	for _, size := range []int64{0, 100} {
		file, err := client.FilesCreate(ctx, &api.File{
			Name:      fmt.Sprintf("head-range-%d.txt", size),
			Type:      api.FileTypeFile,
			Path:      api.NewOptString("/"),
			MimeType:  api.NewOptString("text/plain"),
			ChannelId: api.NewOptInt64(910084),
			Size:      api.NewOptInt64(size),
		})
		if err != nil {
			t.Fatalf("FilesCreate size %d failed: %v", size, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.server.URL+"/files/"+uuid.UUID(file.ID.Value).String()+"/content", nil)
		if err != nil {
			t.Fatalf("new HEAD request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Cookie", "access_token="+token)
		req.Header.Set("Range", "bytes=1000-2000")

		resp, err := s.httpCli.Do(req)
		if err != nil {
			t.Fatalf("HEAD request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
			t.Fatalf("size %d status = %d, want 416", size, resp.StatusCode)
		}
		want := fmt.Sprintf("bytes */%d", size)
		if got := resp.Header.Get("Content-Range"); got != want {
			t.Fatalf("size %d Content-Range = %q, want %q", size, got, want)
		}
	}
}

func TestStreamFile_MultipleRangesReturn400RegardlessOfOrder(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	token := loginAndGetToken(t, s, 9105, "stream-user-9105")
	client := s.newClientWithToken(token)
	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910085), ChannelName: api.NewOptString("stream-multi-range")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "multi-range.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910085),
		Size:      api.NewOptInt64(100),
	})
	if err != nil {
		t.Fatalf("FilesCreate failed: %v", err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, rawRange := range []string{"bytes=0-1,1000-2000", "bytes=1000-2000,0-1"} {
			req, err := http.NewRequestWithContext(ctx, method, s.server.URL+"/files/"+uuid.UUID(file.ID.Value).String()+"/content", nil)
			if err != nil {
				t.Fatalf("new %s request: %v", method, err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Cookie", "access_token="+token)
			req.Header.Set("Range", rawRange)

			resp, err := s.httpCli.Do(req)
			if err != nil {
				t.Fatalf("%s request failed: %v", method, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s %q status = %d, want 400", method, rawRange, resp.StatusCode)
			}
		}
	}
}

func TestStreamFile_ValidRangeAccepted(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	token := loginAndGetToken(t, s, 9103, "stream-user-9103")
	client := s.newClientWithToken(token)

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910083), ChannelName: api.NewOptString("stream-valid")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "valid-range.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910083),
		Size:      api.NewOptInt64(100),
	})
	if err != nil {
		t.Fatalf("FilesCreate file failed: %v", err)
	}

	// HEAD is not generated for this route. The fixture file has metadata but no
	// backing Telegram parts, so streaming setup should fail before writing 206.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.server.URL+"/files/"+uuid.UUID(file.ID.Value).String()+"/content", nil)
	if err != nil {
		t.Fatalf("new HEAD request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", "access_token="+token)
	req.Header.Set("Range", "bytes=0-49")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("HEAD request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for fixture without streamable content, got %d", resp.StatusCode)
	}
}
