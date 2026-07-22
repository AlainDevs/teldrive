package api_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tgdrive/teldrive/internal/api"
	jetmodel "github.com/tgdrive/teldrive/internal/database/jet/gen/model"
	"github.com/tgdrive/teldrive/pkg/services"
)

func TestSharesStream(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, client, _ := loginWithClient(t, s, 8404, "user8404")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910084), ChannelName: api.NewOptString("stream-share-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	// Create a file with zero size — streamFile returns 200 immediately
	// without any Telegram interaction for zero-length files.
	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "stream-share.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910084),
		Size:      api.NewOptInt64(0),
	})
	if err != nil {
		t.Fatalf("FilesCreate file failed: %v", err)
	}

	// Create an unprotected share.
	if err := client.FilesCreateShare(ctx, &api.FileShareCreate{}, api.FilesCreateShareParams{ID: file.ID.Value}); err != nil {
		t.Fatalf("FilesCreateShare failed: %v", err)
	}

	shares, err := client.FilesListShares(ctx, api.FilesListSharesParams{ID: file.ID.Value})
	if err != nil || len(shares) == 0 {
		t.Fatalf("FilesListShares failed: %v len=%d", err, len(shares))
	}

	// Hit the raw SharesStream endpoint.
	shareID := uuid.UUID(shares[0].ID)
	fileID := uuid.UUID(file.ID.Value)
	reqURL := s.server.URL + "/shares/" + shareID.String() + "/files/" + fileID.String() + "/content"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSharesStreamNoBotUsesNewestOwnerSession(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	const userID int64 = 8407
	_, client, loginSessionID := loginWithClient(t, s, userID, "user8407")

	loginSession, err := s.repos.Sessions.GetByID(ctx, uuid.MustParse(loginSessionID))
	if err != nil {
		t.Fatalf("get login session: %v", err)
	}

	newestSession := "1BvXNhK1zA5P-NEWEST-SHARE-STREAM-SESSION-8407"
	newestCreatedAt := loginSession.CreatedAt.Add(time.Minute)
	if err := s.repos.Sessions.Create(ctx, &jetmodel.Sessions{
		ID:        uuid.New(),
		UserID:    userID,
		TgSession: newestSession,
		CreatedAt: newestCreatedAt,
		UpdatedAt: newestCreatedAt,
	}); err != nil {
		t.Fatalf("create newer owner session: %v", err)
	}

	bots, err := s.repos.Bots.GetTokensByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get owner bot tokens: %v", err)
	}
	if len(bots) != 0 {
		t.Fatalf("expected owner to have zero bot tokens, got %d", len(bots))
	}

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910087), ChannelName: api.NewOptString("share-newest-session-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "share-newest-session.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910087),
		Size:      api.NewOptInt64(12),
		Parts: []api.Part{
			{ID: 920087},
		},
	})
	if err != nil {
		t.Fatalf("FilesCreate file failed: %v", err)
	}

	if err := client.FilesCreateShare(ctx, &api.FileShareCreate{}, api.FilesCreateShareParams{ID: file.ID.Value}); err != nil {
		t.Fatalf("FilesCreateShare failed: %v", err)
	}

	shares, err := client.FilesListShares(ctx, api.FilesListSharesParams{ID: file.ID.Value})
	if err != nil || len(shares) == 0 {
		t.Fatalf("FilesListShares failed: %v len=%d", err, len(shares))
	}

	authClientErr := errors.New("sentinel auth client stop")
	capturedSession := ""
	s.tgMock.authClientFn = func(_ context.Context, session string, _ int) (services.TelegramClient, error) {
		capturedSession = session
		return nil, authClientErr
	}

	shareID := uuid.UUID(shares[0].ID)
	fileID := uuid.UUID(file.ID.Value)
	reqURL := s.server.URL + "/shares/" + shareID.String() + "/files/" + fileID.String() + "/content"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 after sentinel AuthClient error, got %d", resp.StatusCode)
	}
	if capturedSession != newestSession {
		t.Fatalf("expected AuthClient session %q, got %q", newestSession, capturedSession)
	}
}

func TestSharesStreamFileShareRejectsSubstitutedFile(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, client, _ := loginWithClient(t, s, 8408, "user8408")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910088), ChannelName: api.NewOptString("share-file-swap-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	sharedFile, err := client.FilesCreate(ctx, &api.File{
		Name:      "shared.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910088),
		Size:      api.NewOptInt64(12),
		Parts:     []api.Part{{ID: 920088}},
	})
	if err != nil {
		t.Fatalf("FilesCreate shared file failed: %v", err)
	}
	swappedFile, err := client.FilesCreate(ctx, &api.File{
		Name:      "swapped.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910088),
		Size:      api.NewOptInt64(12),
		Parts:     []api.Part{{ID: 920089}},
	})
	if err != nil {
		t.Fatalf("FilesCreate swapped file failed: %v", err)
	}

	shareID := createShareAndGetID(t, ctx, client, sharedFile.ID.Value)
	authClientCalls := 0
	s.tgMock.authClientFn = func(context.Context, string, int) (services.TelegramClient, error) {
		authClientCalls++
		return nil, errors.New("AuthClient should not be called")
	}

	resp := requestShareStream(t, ctx, s, uuid.UUID(shareID), uuid.UUID(swappedFile.ID.Value))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if authClientCalls != 0 {
		t.Fatalf("expected no AuthClient calls, got %d", authClientCalls)
	}
}

func TestSharesStreamFolderShareAuthorizesOnlyFileDescendants(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, client, _ := loginWithClient(t, s, 8409, "user8409")
	_, otherClient, _ := loginWithClient(t, s, 8410, "user8410")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910089), ChannelName: api.NewOptString("share-folder-stream-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := otherClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910090), ChannelName: api.NewOptString("share-folder-stream-other-test")}); err != nil {
		t.Fatalf("other UsersUpdateChannel failed: %v", err)
	}

	rootFolder, err := client.FilesCreate(ctx, &api.File{Name: "shared-folder", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate root folder failed: %v", err)
	}
	_, err = client.FilesCreate(ctx, &api.File{Name: "nested", Type: api.FileTypeFolder, Path: api.NewOptString("/shared-folder")})
	if err != nil {
		t.Fatalf("FilesCreate nested folder failed: %v", err)
	}
	descendant, err := client.FilesCreate(ctx, &api.File{
		Name:      "descendant.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/shared-folder/nested"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910089),
		Size:      api.NewOptInt64(12),
		Parts:     []api.Part{{ID: 920090}},
	})
	if err != nil {
		t.Fatalf("FilesCreate descendant failed: %v", err)
	}
	sibling, err := client.FilesCreate(ctx, &api.File{
		Name:      "sibling.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910089),
		Size:      api.NewOptInt64(12),
		Parts:     []api.Part{{ID: 920091}},
	})
	if err != nil {
		t.Fatalf("FilesCreate sibling failed: %v", err)
	}
	otherFile, err := otherClient.FilesCreate(ctx, &api.File{
		Name:      "other.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910090),
		Size:      api.NewOptInt64(12),
		Parts:     []api.Part{{ID: 920092}},
	})
	if err != nil {
		t.Fatalf("FilesCreate other file failed: %v", err)
	}

	shareID := createShareAndGetID(t, ctx, client, rootFolder.ID.Value)
	authClientErr := errors.New("sentinel descendant authorized")
	authClientCalls := 0
	s.tgMock.authClientFn = func(context.Context, string, int) (services.TelegramClient, error) {
		authClientCalls++
		return nil, authClientErr
	}

	resp := requestShareStream(t, ctx, s, uuid.UUID(shareID), uuid.UUID(descendant.ID.Value))
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 after descendant AuthClient sentinel, got %d", resp.StatusCode)
	}
	if authClientCalls != 1 {
		t.Fatalf("expected one AuthClient call for descendant, got %d", authClientCalls)
	}

	for name, fileID := range map[string]uuid.UUID{
		"outside sibling": uuid.UUID(sibling.ID.Value),
		"other owner":     uuid.UUID(otherFile.ID.Value),
		"folder target":   uuid.UUID(rootFolder.ID.Value),
	} {
		t.Run(name, func(t *testing.T) {
			authClientCalls = 0
			resp := requestShareStream(t, ctx, s, uuid.UUID(shareID), fileID)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", resp.StatusCode)
			}
			if authClientCalls != 0 {
				t.Fatalf("expected no AuthClient calls, got %d", authClientCalls)
			}
		})
	}
}

func createShareAndGetID(t *testing.T, ctx context.Context, client *api.Client, fileID api.UUID) api.UUID {
	t.Helper()

	if err := client.FilesCreateShare(ctx, &api.FileShareCreate{}, api.FilesCreateShareParams{ID: fileID}); err != nil {
		t.Fatalf("FilesCreateShare failed: %v", err)
	}
	shares, err := client.FilesListShares(ctx, api.FilesListSharesParams{ID: fileID})
	if err != nil || len(shares) == 0 {
		t.Fatalf("FilesListShares failed: %v len=%d", err, len(shares))
	}
	return shares[0].ID
}

func requestShareStream(t *testing.T, ctx context.Context, s *harness, shareID uuid.UUID, fileID uuid.UUID) *http.Response {
	t.Helper()

	reqURL := s.server.URL + "/shares/" + shareID.String() + "/files/" + fileID.String() + "/content"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestFilesEditShare(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, client, _ := loginWithClient(t, s, 8405, "user8405")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910085), ChannelName: api.NewOptString("edit-share-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "edit-share.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910085),
		Size:      api.NewOptInt64(12),
	})
	if err != nil {
		t.Fatalf("FilesCreate file failed: %v", err)
	}

	// Create an unprotected share.
	if err := client.FilesCreateShare(ctx, &api.FileShareCreate{}, api.FilesCreateShareParams{ID: file.ID.Value}); err != nil {
		t.Fatalf("FilesCreateShare failed: %v", err)
	}

	shares, err := client.FilesListShares(ctx, api.FilesListSharesParams{ID: file.ID.Value})
	if err != nil || len(shares) == 0 {
		t.Fatalf("FilesListShares failed: %v len=%d", err, len(shares))
	}

	// Edit the share to add a password.
	if err := client.FilesEditShare(ctx, &api.FileShareCreate{Password: api.NewOptString("newpw")}, api.FilesEditShareParams{ID: file.ID.Value, ShareId: shares[0].ID}); err != nil {
		t.Fatalf("FilesEditShare failed: %v", err)
	}
}

func TestFilesDeleteShare(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, client, _ := loginWithClient(t, s, 8406, "user8406")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910086), ChannelName: api.NewOptString("delete-share-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}

	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "delete-share.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910086),
		Size:      api.NewOptInt64(12),
	})
	if err != nil {
		t.Fatalf("FilesCreate file failed: %v", err)
	}

	// Create an unprotected share.
	if err := client.FilesCreateShare(ctx, &api.FileShareCreate{}, api.FilesCreateShareParams{ID: file.ID.Value}); err != nil {
		t.Fatalf("FilesCreateShare failed: %v", err)
	}

	shares, err := client.FilesListShares(ctx, api.FilesListSharesParams{ID: file.ID.Value})
	if err != nil || len(shares) == 0 {
		t.Fatalf("FilesListShares failed: %v len=%d", err, len(shares))
	}

	// Delete the share.
	if err := client.FilesDeleteShare(ctx, api.FilesDeleteShareParams{ID: file.ID.Value, ShareId: shares[0].ID}); err != nil {
		t.Fatalf("FilesDeleteShare failed: %v", err)
	}

	// Verify the share is gone — SharesGetById should return 404.
	_, err = client.SharesGetById(ctx, api.SharesGetByIdParams{ID: shares[0].ID})
	if statusCode(err) != 404 {
		t.Fatalf("expected 404 after delete, got %d err=%v", statusCode(err), err)
	}
}
