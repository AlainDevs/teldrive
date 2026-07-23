package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gotd/td/tg"
	"github.com/tgdrive/teldrive/internal/api"
	"github.com/tgdrive/teldrive/internal/crypt"
	jetmodel "github.com/tgdrive/teldrive/internal/database/jet/gen/model"
	"github.com/tgdrive/teldrive/pkg/repositories"
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

func TestSharesStream_OutOfBoundsRangeReturns416(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, client, _ := loginWithClient(t, s, 8511, "user8511")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910111), ChannelName: api.NewOptString("share-range-test")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	file, err := client.FilesCreate(ctx, &api.File{
		Name:      "share-range.txt",
		Type:      api.FileTypeFile,
		Path:      api.NewOptString("/"),
		MimeType:  api.NewOptString("text/plain"),
		ChannelId: api.NewOptInt64(910111),
		Size:      api.NewOptInt64(100),
	})
	if err != nil {
		t.Fatalf("FilesCreate failed: %v", err)
	}
	shareID := createShareAndGetID(t, ctx, client, file.ID.Value)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.server.URL+"/shares/"+uuid.UUID(shareID).String()+"/files/"+uuid.UUID(file.ID.Value).String()+"/content", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Range", "bytes=1000-2000")
	response, err := s.httpCli.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, want 416", response.StatusCode)
	}
	if got := response.Header.Get("Content-Range"); got != "bytes */100" {
		t.Fatalf("Content-Range = %q, want %q", got, "bytes */100")
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

func TestSharesCreateFile_WriteAuthorizationAndContainment(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8501, "user8501")
	_, otherClient, _ := loginWithClient(t, s, 8502, "user8502")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910101), ChannelName: api.NewOptString("share-write-owner")}); err != nil {
		t.Fatalf("owner UsersUpdateChannel failed: %v", err)
	}
	if err := otherClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910102), ChannelName: api.NewOptString("share-write-other")}); err != nil {
		t.Fatalf("other UsersUpdateChannel failed: %v", err)
	}

	rootFolder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "shared-write-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate shared root failed: %v", err)
	}
	outsideFolder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "outside-write-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate outside root failed: %v", err)
	}
	sharedFile, err := ownerClient.FilesCreate(ctx, &api.File{Name: "shared-file.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), ChannelId: api.NewOptInt64(910101), Size: api.NewOptInt64(0)})
	if err != nil {
		t.Fatalf("FilesCreate shared file failed: %v", err)
	}

	readOnlyShareID := createShareAndGetID(t, ctx, ownerClient, rootFolder.ID.Value)
	_, err = public.SharesCreateFile(ctx, &api.File{Name: "denied", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: readOnlyShareID})
	if statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected 403 for read-only share write, got %d err=%v", statusCode(err), err)
	}

	writeShareID := createShareWithRequestAndGetID(t, ctx, ownerClient, rootFolder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	writeShareInfo, err := public.SharesGetById(ctx, api.SharesGetByIdParams{ID: writeShareID})
	if err != nil {
		t.Fatalf("SharesGetById writable failed: %v", err)
	}
	if !writeShareInfo.AllowUpload {
		t.Fatalf("expected allowUpload in share info response")
	}
	if writeShareInfo.EncryptUploads {
		t.Fatalf("expected default encryptUploads=false")
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(true)}); err != nil {
		t.Fatalf("UsersUpdateConfig true failed: %v", err)
	}
	writeShareInfo, err = public.SharesGetById(ctx, api.SharesGetByIdParams{ID: writeShareID})
	if err != nil {
		t.Fatalf("SharesGetById after enable failed: %v", err)
	}
	if !writeShareInfo.EncryptUploads {
		t.Fatalf("expected existing share encryptUploads=true after owner toggle")
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(false)}); err != nil {
		t.Fatalf("UsersUpdateConfig false failed: %v", err)
	}
	writeShareInfo, err = public.SharesGetById(ctx, api.SharesGetByIdParams{ID: writeShareID})
	if err != nil {
		t.Fatalf("SharesGetById after disable failed: %v", err)
	}
	if writeShareInfo.EncryptUploads {
		t.Fatalf("expected existing share encryptUploads=false after owner toggle")
	}

	rootCreated, err := public.SharesCreateFile(ctx, &api.File{Name: "from-public-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: writeShareID})
	if err != nil {
		t.Fatalf("SharesCreateFile root failed: %v", err)
	}
	if !rootCreated.ParentId.IsSet() || rootCreated.ParentId.Value != rootFolder.ID.Value {
		t.Fatalf("expected root-created parent %s, got %+v", rootFolder.ID.Value, rootCreated.ParentId)
	}

	nestedCreated, err := public.SharesCreateFile(ctx, &api.File{Name: "from-public-nested", Type: api.FileTypeFolder, ParentId: api.NewOptUUID(rootCreated.ID.Value)}, api.SharesCreateFileParams{ID: writeShareID})
	if err != nil {
		t.Fatalf("SharesCreateFile nested failed: %v", err)
	}
	if !nestedCreated.ParentId.IsSet() || nestedCreated.ParentId.Value != rootCreated.ID.Value {
		t.Fatalf("expected nested parent %s, got %+v", rootCreated.ID.Value, nestedCreated.ParentId)
	}

	_, err = public.SharesCreateFile(ctx, &api.File{Name: "from-public-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: writeShareID})
	if statusCode(err) != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate name, got %d err=%v", statusCode(err), err)
	}

	_, err = public.SharesCreateFile(ctx, &api.File{Name: "outside", Type: api.FileTypeFolder, ParentId: api.NewOptUUID(outsideFolder.ID.Value)}, api.SharesCreateFileParams{ID: writeShareID})
	if statusCode(err) != http.StatusNotFound {
		t.Fatalf("expected 404 for outside-subtree parent, got %d err=%v", statusCode(err), err)
	}

	if err := ownerClient.FilesCreateShare(ctx, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)}, api.FilesCreateShareParams{ID: sharedFile.ID.Value}); statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected 403 creating writable file share, got %d err=%v", statusCode(err), err)
	}
	fileShareID := createShareAndGetID(t, ctx, ownerClient, sharedFile.ID.Value)
	if err := ownerClient.FilesEditShare(ctx, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)}, api.FilesEditShareParams{ID: sharedFile.ID.Value, ShareId: fileShareID}); statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected 403 editing file share writable, got %d err=%v", statusCode(err), err)
	}

	if err := otherClient.FilesEditShare(ctx, &api.FileShareCreate{Password: api.NewOptString("other")}, api.FilesEditShareParams{ID: rootFolder.ID.Value, ShareId: writeShareID}); statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected 403 editing another user's share, got %d err=%v", statusCode(err), err)
	}
	if err := otherClient.FilesDeleteShare(ctx, api.FilesDeleteShareParams{ID: rootFolder.ID.Value, ShareId: writeShareID}); statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected 403 deleting another user's share, got %d err=%v", statusCode(err), err)
	}
}

func TestSharesCreateFile_ProtectedAndExpiredWrites(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8503, "user8503")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910103), ChannelName: api.NewOptString("share-write-protected")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "protected-write-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}

	protectedShareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{Password: api.NewOptString("pw-write"), AllowUpload: api.NewOptBool(true)})
	_, err = public.SharesCreateFile(ctx, &api.File{Name: "needs-token", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: protectedShareID})
	if statusCode(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 without share token, got %d err=%v", statusCode(err), err)
	}
	_, err = public.SharesCreateFile(ctx, &api.File{Name: "bad-token", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: protectedShareID, ShareToken: api.NewOptString("bad-token")})
	if statusCode(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid share token, got %d err=%v", statusCode(err), err)
	}
	unlockRes, err := public.SharesUnlock(ctx, &api.ShareUnlock{Password: "pw-write"}, api.SharesUnlockParams{ID: protectedShareID})
	if err != nil {
		t.Fatalf("SharesUnlock failed: %v", err)
	}
	shareToken, err := tokenFromSetCookie(unlockRes.SetCookie)
	if err != nil {
		t.Fatalf("parse share token: %v", err)
	}
	created, err := public.SharesCreateFile(ctx, &api.File{Name: "with-token", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: protectedShareID, ShareToken: api.NewOptString(shareToken)})
	if err != nil {
		t.Fatalf("SharesCreateFile with valid share token failed: %v", err)
	}
	if created.Name != "with-token" {
		t.Fatalf("expected created folder name with-token, got %q", created.Name)
	}

	expiredShareID := uuid.New()
	pastTime := time.Now().UTC().Add(-time.Hour)
	if err := s.repos.Shares.Create(ctx, &jetmodel.FileShares{ID: expiredShareID, FileID: uuid.UUID(folder.ID.Value), ExpiresAt: &pastTime, UserID: 8503, AllowUpload: true}); err != nil {
		t.Fatalf("seed expired writable share: %v", err)
	}
	_, err = public.SharesCreateFile(ctx, &api.File{Name: "expired", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: api.UUID(expiredShareID)})
	if statusCode(err) != http.StatusNotFound {
		t.Fatalf("expected 404 for expired share write, got %d err=%v", statusCode(err), err)
	}
}

func TestSharesUploadAndFinalize_UsesShareOwner(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8504, "user8504")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910104), ChannelName: api.NewOptString("share-upload-owner")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "upload-write-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	payload := []byte("shared upload payload")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, partName string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910104 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		if partName == "" {
			return 0, 0, fmt.Errorf("expected generated part name")
		}
		got, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if !bytes.Equal(got, payload) {
			return 0, 0, fmt.Errorf("payload mismatch")
		}
		return 12104, fileSize, nil
	}

	part, status, raw := shareUploadPartRaw(t, s, shareID, "share-up-8504", "shared-upload.txt", 1, true, payload)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from SharesUpload, got %d body=%s", status, string(raw))
	}
	if part.PartId != 12104 || part.ChannelId != 910104 || part.Size != int64(len(payload)) {
		t.Fatalf("unexpected share upload part: %+v", part)
	}

	clientUploadID := "share-up-8504"
	namespacedUploadID := "share:" + uuid.UUID(shareID).String() + ":" + clientUploadID
	uploadRows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, namespacedUploadID, 8504)
	if err != nil {
		t.Fatalf("GetByUploadIDAndUserID failed: %v", err)
	}
	if len(uploadRows) != 1 {
		t.Fatalf("expected one owner upload row, got %d", len(uploadRows))
	}
	rawUploadRows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, clientUploadID, 8504)
	if err != nil {
		t.Fatalf("GetByUploadIDAndUserID raw failed: %v", err)
	}
	if len(rawUploadRows) != 0 {
		t.Fatalf("expected no raw share upload rows, got %d", len(rawUploadRows))
	}

	created, err := public.SharesCreateFile(ctx, &api.File{Name: "shared-upload.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(payload))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile finalization failed: %v", err)
	}
	if !created.ParentId.IsSet() || created.ParentId.Value != folder.ID.Value {
		t.Fatalf("expected finalized file under shared folder %s, got %+v", folder.ID.Value, created.ParentId)
	}
	if !created.ChannelId.IsSet() || created.ChannelId.Value != 910104 {
		t.Fatalf("expected finalized file channel 910104, got %+v", created.ChannelId)
	}
	uploadRows, err = s.repos.Uploads.GetByUploadIDAndUserID(ctx, namespacedUploadID, 8504)
	if err != nil {
		t.Fatalf("GetByUploadIDAndUserID after finalize failed: %v", err)
	}
	if len(uploadRows) != 0 {
		t.Fatalf("expected owner upload rows cleaned up, got %d", len(uploadRows))
	}
}

func TestSharesUploadAndFinalize_UpdatesAncestorFolderSizes(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8512, "user8512")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910112), ChannelName: api.NewOptString("share-upload-folder-sizes")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	rootFolder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "upload-size-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate root folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, rootFolder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	nestedFolder, err := public.SharesCreateFile(ctx, &api.File{Name: "nested-size", Type: api.FileTypeFolder, Path: api.NewOptString("/")}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile nested folder failed: %v", err)
	}

	payload := []byte("nested share upload payload")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910112 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		got, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if !bytes.Equal(got, payload) {
			return 0, 0, fmt.Errorf("payload mismatch")
		}
		return 13112, fileSize, nil
	}

	clientUploadID := "share-up-8512"
	_, status, raw := shareUploadPartRaw(t, s, shareID, clientUploadID, "nested-upload.txt", 1, false, payload)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from SharesUpload, got %d body=%s", status, string(raw))
	}
	created, err := public.SharesCreateFile(ctx, &api.File{Name: "nested-upload.txt", Type: api.FileTypeFile, ParentId: api.NewOptUUID(nestedFolder.ID.Value), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(payload))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile finalization failed: %v", err)
	}
	if !created.ParentId.IsSet() || created.ParentId.Value != nestedFolder.ID.Value {
		t.Fatalf("expected finalized file under nested folder %s, got %+v", nestedFolder.ID.Value, created.ParentId)
	}

	nestedAfter, err := ownerClient.FilesGetById(ctx, api.FilesGetByIdParams{ID: nestedFolder.ID.Value})
	if err != nil {
		t.Fatalf("FilesGetById nested folder failed: %v", err)
	}
	rootAfter, err := ownerClient.FilesGetById(ctx, api.FilesGetByIdParams{ID: rootFolder.ID.Value})
	if err != nil {
		t.Fatalf("FilesGetById root folder failed: %v", err)
	}
	wantSize := int64(len(payload))
	if !nestedAfter.Size.IsSet() || nestedAfter.Size.Value != wantSize {
		t.Fatalf("expected nested folder size %d, got %+v", wantSize, nestedAfter.Size)
	}
	if !rootAfter.Size.IsSet() || rootAfter.Size.Value != wantSize {
		t.Fatalf("expected root folder size %d, got %+v", wantSize, rootAfter.Size)
	}
}

func TestSharesUploadAndFinalize_ConcurrentSameUploadIDSingleUse(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8516, "user8516")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910116), ChannelName: api.NewOptString("share-upload-single-use")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "single-use-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate root folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	payload := []byte("single-use payload")
	clientUploadID := "single-use-upload"
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910116 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		got, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if !bytes.Equal(got, payload) {
			return 0, 0, fmt.Errorf("payload mismatch")
		}
		return 13116, fileSize, nil
	}
	_, status, raw := shareUploadPartRaw(t, s, shareID, clientUploadID, "single-use.txt", 1, false, payload)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from SharesUpload, got %d body=%s", status, string(raw))
	}

	start := make(chan struct{})
	type finalizeResult struct {
		file *api.File
		err  error
	}
	results := make([]finalizeResult, 2)
	names := []string{"single-use-a.txt", "single-use-b.txt"}
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			file, err := public.SharesCreateFile(context.Background(), &api.File{Name: names[index], Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(payload))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
			results[index] = finalizeResult{file: file, err: err}
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	badRequests := 0
	createdNames := map[string]bool{}
	for _, result := range results {
		if result.err == nil {
			successes++
			createdNames[result.file.Name] = true
			continue
		}
		if statusCode(result.err) == http.StatusBadRequest {
			badRequests++
		}
	}
	if successes != 1 || badRequests != 1 {
		t.Fatalf("expected one success and one 400, got successes=%d badRequests=%d results=%+v", successes, badRequests, results)
	}

	createdFiles := 0
	parentUUID := uuid.UUID(folder.ID.Value)
	for _, name := range names {
		file, err := s.repos.Files.GetActiveByNameAndParent(ctx, 8516, name, &parentUUID)
		if err == nil {
			createdFiles++
			if !createdNames[file.Name] {
				t.Fatalf("created unexpected finalized file %q", file.Name)
			}
			continue
		}
		if !errors.Is(err, repositories.ErrNotFound) {
			t.Fatalf("lookup finalized name %q: %v", name, err)
		}
	}
	if createdFiles != 1 {
		t.Fatalf("expected one created finalized file, got %d", createdFiles)
	}

	folderAfter, err := ownerClient.FilesGetById(ctx, api.FilesGetByIdParams{ID: folder.ID.Value})
	if err != nil {
		t.Fatalf("FilesGetById folder failed: %v", err)
	}
	if !folderAfter.Size.IsSet() || folderAfter.Size.Value != int64(len(payload)) {
		t.Fatalf("expected one ancestor size increment %d, got %+v", len(payload), folderAfter.Size)
	}
	namespacedUploadID := "share:" + uuid.UUID(shareID).String() + ":" + clientUploadID
	rows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, namespacedUploadID, 8516)
	if err != nil {
		t.Fatalf("GetByUploadIDAndUserID after concurrent finalize: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected staged rows gone after single-use finalize, got %d", len(rows))
	}
}

func TestSharesUploadAndFinalize_UsesStagedEncryptionMetadata(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	s.cfg.TG.Uploads.EncryptionKey = "integration-test-encryption-key"
	public, ownerClient, _ := loginWithClient(t, s, 8513, "user8513")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910113), ChannelName: api.NewOptString("share-upload-encrypted")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(true)}); err != nil {
		t.Fatalf("UsersUpdateConfig failed: %v", err)
	}
	rootFolder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "upload-encrypted-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate root folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, rootFolder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	plaintext := []byte("secret share upload payload")
	expectedEncryptedSize := crypt.EncryptedSize(int64(len(plaintext)))
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910113 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		ciphertext, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if fileSize != expectedEncryptedSize {
			return 0, 0, fmt.Errorf("encrypted file size mismatch got=%d want=%d", fileSize, expectedEncryptedSize)
		}
		if int64(len(ciphertext)) != expectedEncryptedSize {
			return 0, 0, fmt.Errorf("encrypted payload size mismatch got=%d want=%d", len(ciphertext), expectedEncryptedSize)
		}
		if bytes.Equal(ciphertext, plaintext) {
			return 0, 0, fmt.Errorf("expected ciphertext to differ from plaintext")
		}
		return 13113, fileSize, nil
	}

	clientUploadID := "share-up-8513"
	part, status, raw := shareUploadPartRawWithQuery(t, s, shareID, clientUploadID, "encrypted-share.txt", 1, false, bytes.NewReader(plaintext), int64(len(plaintext)), url.Values{"encrypted": []string{"false"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200 from encrypted SharesUpload, got %d body=%s", status, string(raw))
	}
	if !part.Encrypted {
		t.Fatalf("expected encrypted share upload part")
	}
	if !part.Salt.IsSet() || part.Salt.Value == "" {
		t.Fatalf("expected salt in encrypted share upload part")
	}
	if part.Size != expectedEncryptedSize {
		t.Fatalf("expected encrypted part size %d, got %d", expectedEncryptedSize, part.Size)
	}

	created, err := public.SharesCreateFile(ctx, &api.File{Name: "encrypted-share.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(plaintext))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile encrypted finalization failed: %v", err)
	}
	if !created.Encrypted.IsSet() || !created.Encrypted.Value {
		t.Fatalf("expected created file encrypted=true, got %+v", created.Encrypted)
	}
	if !created.Size.IsSet() || created.Size.Value != int64(len(plaintext)) {
		t.Fatalf("expected created plaintext size %d, got %+v", len(plaintext), created.Size)
	}
	if len(created.Parts) != 1 || !created.Parts[0].Salt.IsSet() || created.Parts[0].Salt.Value != part.Salt.Value {
		t.Fatalf("expected created part salt %q, got %+v", part.Salt.Value, created.Parts)
	}

	fileDB, err := s.repos.Files.GetByID(ctx, uuid.UUID(created.ID.Value))
	if err != nil {
		t.Fatalf("GetByID created encrypted file failed: %v", err)
	}
	if !fileDB.Encrypted {
		t.Fatalf("expected DB file encrypted=true")
	}
	if fileDB.Size == nil || *fileDB.Size != int64(len(plaintext)) {
		t.Fatalf("expected DB plaintext size %d, got %+v", len(plaintext), fileDB.Size)
	}
}

func TestSharesUploadAndFinalize_RejectsStalePlaintextWhenOwnerPolicyEnabled(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	s.cfg.TG.Uploads.EncryptionKey = "integration-test-encryption-key"
	public, ownerClient, _ := loginWithClient(t, s, 8523, "user8523")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910123), ChannelName: api.NewOptString("share-stale-plaintext")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(false)}); err != nil {
		t.Fatalf("UsersUpdateConfig false failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "stale-plaintext-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	payload := []byte("stale plaintext payload")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910123 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		got, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if !bytes.Equal(got, payload) {
			return 0, 0, fmt.Errorf("expected plaintext payload")
		}
		return 13231, fileSize, nil
	}

	clientUploadID := "stale-plaintext-upload"
	part, status, raw := shareUploadPartRaw(t, s, shareID, clientUploadID, "stale-plaintext.txt", 1, false, payload)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from plaintext SharesUpload, got %d body=%s", status, string(raw))
	}
	if part.Encrypted {
		t.Fatalf("expected staged plaintext part")
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(true)}); err != nil {
		t.Fatalf("UsersUpdateConfig true failed: %v", err)
	}

	_, err = public.SharesCreateFile(ctx, &api.File{Name: "stale-plaintext.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(payload))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for stale plaintext staged upload, got %d err=%v", statusCode(err), err)
	}
	if eb := errorResponse(err); eb == nil || eb.Message != "staged upload encryption does not match current owner policy" {
		t.Fatalf("expected stale policy error message, got %+v", eb)
	}
	assertShareFinalizeRejectedPreserved(t, ctx, s, 8523, uuid.UUID(shareID), clientUploadID, "stale-plaintext.txt", uuid.UUID(folder.ID.Value), 13231)
}

func TestSharesUploadAndFinalize_RejectsStaleEncryptedWhenOwnerPolicyDisabled(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	s.cfg.TG.Uploads.EncryptionKey = "integration-test-encryption-key"
	public, ownerClient, _ := loginWithClient(t, s, 8524, "user8524")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910124), ChannelName: api.NewOptString("share-stale-encrypted")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(true)}); err != nil {
		t.Fatalf("UsersUpdateConfig true failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "stale-encrypted-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	plaintext := []byte("stale encrypted payload")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910124 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		ciphertext, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if bytes.Equal(ciphertext, plaintext) {
			return 0, 0, fmt.Errorf("expected ciphertext")
		}
		return 13241, fileSize, nil
	}

	clientUploadID := "stale-encrypted-upload"
	part, status, raw := shareUploadPartRaw(t, s, shareID, clientUploadID, "stale-encrypted.txt", 1, false, plaintext)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from encrypted SharesUpload, got %d body=%s", status, string(raw))
	}
	if !part.Encrypted {
		t.Fatalf("expected staged encrypted part")
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(false)}); err != nil {
		t.Fatalf("UsersUpdateConfig false failed: %v", err)
	}

	_, err = public.SharesCreateFile(ctx, &api.File{Name: "stale-encrypted.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(plaintext))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for stale encrypted staged upload, got %d err=%v", statusCode(err), err)
	}
	if eb := errorResponse(err); eb == nil || eb.Message != "staged upload encryption does not match current owner policy" {
		t.Fatalf("expected stale policy error message, got %+v", eb)
	}
	assertShareFinalizeRejectedPreserved(t, ctx, s, 8524, uuid.UUID(shareID), clientUploadID, "stale-encrypted.txt", uuid.UUID(folder.ID.Value), 13241)
}

func TestSharesUpload_OwnerPolicyTrueWithoutKeyReturns400(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8522, "user8522")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910122), ChannelName: api.NewOptString("share-upload-encrypted-no-key")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(true)}); err != nil {
		t.Fatalf("UsersUpdateConfig failed: %v", err)
	}
	rootFolder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "upload-no-key-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate root folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, rootFolder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	telegramCalls := 0
	s.tgMock.uploadPartFn = func(context.Context, *tg.Client, int64, string, io.Reader, int64, int) (int, int64, error) {
		telegramCalls++
		return 0, 0, errors.New("UploadPart should not be called")
	}
	_, status, raw := shareUploadPartRawWithQuery(t, s, shareID, "share-up-no-key", "no-key.txt", 1, false, bytes.NewReader([]byte("abc")), 3, url.Values{"encrypted": []string{"false"}})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 from encrypted SharesUpload without key, got %d body=%s", status, string(raw))
	}
	if telegramCalls != 0 {
		t.Fatalf("expected no Telegram upload calls, got %d", telegramCalls)
	}
	if _, err := public.SharesGetById(ctx, api.SharesGetByIdParams{ID: shareID}); err != nil {
		t.Fatalf("public SharesGetById should still work: %v", err)
	}
}

func TestSharesUploadAndFinalize_OwnerPolicyFalseIgnoresEncryptedQuery(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	s.cfg.TG.Uploads.EncryptionKey = "integration-test-encryption-key"
	public, ownerClient, _ := loginWithClient(t, s, 8521, "user8521")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910121), ChannelName: api.NewOptString("share-upload-plaintext-policy")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(false)}); err != nil {
		t.Fatalf("UsersUpdateConfig failed: %v", err)
	}
	rootFolder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "upload-plaintext-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate root folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, rootFolder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	plaintext := []byte("public forced plaintext payload")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, channelID int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		if channelID != 910121 {
			return 0, 0, fmt.Errorf("unexpected channel id: %d", channelID)
		}
		got, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if !bytes.Equal(got, plaintext) {
			return 0, 0, fmt.Errorf("expected plaintext payload")
		}
		if fileSize != int64(len(plaintext)) {
			return 0, 0, fmt.Errorf("plaintext size mismatch got=%d want=%d", fileSize, len(plaintext))
		}
		return 13121, fileSize, nil
	}

	clientUploadID := "share-up-8521"
	part, status, raw := shareUploadPartRawWithQuery(t, s, shareID, clientUploadID, "plaintext-share.txt", 1, false, bytes.NewReader(plaintext), int64(len(plaintext)), url.Values{"encrypted": []string{"true"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200 from plaintext SharesUpload, got %d body=%s", status, string(raw))
	}
	if part.Encrypted {
		t.Fatalf("expected plaintext share upload part")
	}
	if part.Salt.IsSet() {
		t.Fatalf("expected no salt for plaintext share upload part")
	}
	if part.Size != int64(len(plaintext)) {
		t.Fatalf("expected plaintext part size %d, got %d", len(plaintext), part.Size)
	}

	created, err := public.SharesCreateFile(ctx, &api.File{Name: "plaintext-share.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(plaintext))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile plaintext finalization failed: %v", err)
	}
	if created.Encrypted.IsSet() && created.Encrypted.Value {
		t.Fatalf("expected created file encrypted=false, got %+v", created.Encrypted)
	}

	fileDB, err := s.repos.Files.GetByID(ctx, uuid.UUID(created.ID.Value))
	if err != nil {
		t.Fatalf("GetByID created plaintext file failed: %v", err)
	}
	if fileDB.Encrypted {
		t.Fatalf("expected DB file encrypted=false")
	}
}

func TestSharesCreateFile_RejectsMalformedStagedSaltMetadata(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8514, "user8514")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910114), ChannelName: api.NewOptString("share-salt-validation")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "salt-validation-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	encryptedSize := crypt.EncryptedSize(12)
	cases := []struct {
		name      string
		uploadID  string
		partID    int32
		size      int64
		encrypted bool
		salt      *string
	}{
		{name: "encrypted nil salt", uploadID: "encrypted-nil-salt", partID: 13141, size: encryptedSize, encrypted: true},
		{name: "encrypted empty salt", uploadID: "encrypted-empty-salt", partID: 13142, size: encryptedSize, encrypted: true, salt: stringPtr("")},
		{name: "encrypted blank salt", uploadID: "encrypted-blank-salt", partID: 13143, size: encryptedSize, encrypted: true, salt: stringPtr("  ")},
		{name: "plaintext salt", uploadID: "plaintext-salt", partID: 13144, size: 12, encrypted: false, salt: stringPtr("unexpected")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			namespacedUploadID := "share:" + uuid.UUID(shareID).String() + ":" + tc.uploadID
			if err := s.repos.Uploads.Create(ctx, &jetmodel.Uploads{UploadID: namespacedUploadID, Name: tc.uploadID + ".txt", UserID: int64Ptr(8514), PartNo: 1, PartID: tc.partID, ChannelID: 910114, Size: tc.size, Encrypted: tc.encrypted, Salt: tc.salt}); err != nil {
				t.Fatalf("seed upload row: %v", err)
			}
			_, err := public.SharesCreateFile(ctx, &api.File{Name: tc.uploadID + ".txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(12), UploadId: api.NewOptString(tc.uploadID)}, api.SharesCreateFileParams{ID: shareID})
			if statusCode(err) != http.StatusBadRequest {
				t.Fatalf("expected 400 for malformed salt metadata, got %d err=%v", statusCode(err), err)
			}
			rows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, namespacedUploadID, 8514)
			if err != nil {
				t.Fatalf("lookup malformed staged rows: %v", err)
			}
			if len(rows) != 1 || rows[0].PartID != tc.partID {
				t.Fatalf("expected malformed validation to preserve staged row %d, got %+v", tc.partID, rows)
			}
		})
	}
}

func TestSharesCreateFile_PersistsTrimmedEncryptedStagedSalt(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	s.cfg.TG.Uploads.EncryptionKey = "integration-test-encryption-key"
	public, ownerClient, _ := loginWithClient(t, s, 8515, "user8515")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910115), ChannelName: api.NewOptString("share-salt-trim")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	if err := ownerClient.UsersUpdateConfig(ctx, &api.UserConfigUpdate{EncryptFiles: api.NewOptBool(true)}); err != nil {
		t.Fatalf("UsersUpdateConfig failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "salt-trim-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	clientUploadID := "encrypted-trimmed-salt"
	namespacedUploadID := "share:" + uuid.UUID(shareID).String() + ":" + clientUploadID
	rawSalt := "  staged-salt  "
	if err := s.repos.Uploads.Create(ctx, &jetmodel.Uploads{UploadID: namespacedUploadID, Name: "trimmed.txt", UserID: int64Ptr(8515), PartNo: 1, PartID: 13151, ChannelID: 910115, Size: crypt.EncryptedSize(12), Encrypted: true, Salt: &rawSalt}); err != nil {
		t.Fatalf("seed upload row: %v", err)
	}

	created, err := public.SharesCreateFile(ctx, &api.File{Name: "trimmed.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(12), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile failed: %v", err)
	}
	if len(created.Parts) != 1 || !created.Parts[0].Salt.IsSet() || created.Parts[0].Salt.Value != "staged-salt" {
		t.Fatalf("expected trimmed encrypted salt, got %+v", created.Parts)
	}
}

func TestSharesCreateFile_FinalizeRequiresUploadIDAndStagedParts(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8505, "user8505")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910105), ChannelName: api.NewOptString("share-finalize-empty")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "finalize-empty-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	_, err = public.SharesCreateFile(ctx, &api.File{Name: "empty-uploadid.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(0)}, api.SharesCreateFileParams{ID: shareID})
	if statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty uploadId, got %d err=%v", statusCode(err), err)
	}
	_, err = public.SharesCreateFile(ctx, &api.File{Name: "missing-staged.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(1), UploadId: api.NewOptString("missing")}, api.SharesCreateFileParams{ID: shareID})
	if statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing staged parts, got %d err=%v", statusCode(err), err)
	}
}

func TestSharesUploadAndFinalize_NamespaceIsolatesAuthenticatedUploads(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8506, "user8506")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910106), ChannelName: api.NewOptString("share-auth-collision")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "auth-collision-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	clientUploadID := "collision-upload"
	if err := s.repos.Uploads.Create(ctx, &jetmodel.Uploads{UploadID: clientUploadID, Name: "auth-part", UserID: int64Ptr(8506), PartNo: 1, PartID: 12601, ChannelID: 910106, Size: 1}); err != nil {
		t.Fatalf("seed authenticated upload row: %v", err)
	}

	payload := []byte("share-owned")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, _ int64, _ string, fileStream io.Reader, fileSize int64, _ int) (int, int64, error) {
		got, err := io.ReadAll(fileStream)
		if err != nil {
			return 0, 0, err
		}
		if !bytes.Equal(got, payload) {
			return 0, 0, fmt.Errorf("payload mismatch")
		}
		return 12602, fileSize, nil
	}
	_, status, raw := shareUploadPartRaw(t, s, shareID, clientUploadID, "share-collision.txt", 1, false, payload)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from SharesUpload, got %d body=%s", status, string(raw))
	}

	created, err := public.SharesCreateFile(ctx, &api.File{Name: "share-collision.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(payload))), UploadId: api.NewOptString(clientUploadID)}, api.SharesCreateFileParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesCreateFile failed: %v", err)
	}
	if !created.Size.IsSet() || created.Size.Value != int64(len(payload)) {
		t.Fatalf("expected created size %d, got %+v", len(payload), created.Size)
	}
	rawRows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, clientUploadID, 8506)
	if err != nil {
		t.Fatalf("lookup raw upload rows: %v", err)
	}
	if len(rawRows) != 1 || rawRows[0].PartID != 12601 {
		t.Fatalf("expected authenticated upload row preserved, got %+v", rawRows)
	}
}

func TestSharesUploadAndFinalize_NamespaceIsolatesSiblingShares(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8507, "user8507")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910107), ChannelName: api.NewOptString("share-sibling-collision")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "sibling-collision-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	firstShareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	secondShareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	if firstShareID == secondShareID {
		t.Fatalf("expected distinct writable shares")
	}

	payload := []byte("first share")
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, _ int64, _ string, _ io.Reader, fileSize int64, _ int) (int, int64, error) {
		return 12701, fileSize, nil
	}
	_, status, raw := shareUploadPartRaw(t, s, firstShareID, "same-client-id", "first-share.txt", 1, false, payload)
	if status != http.StatusOK {
		t.Fatalf("expected 200 from first share upload, got %d body=%s", status, string(raw))
	}
	_, err = public.SharesCreateFile(ctx, &api.File{Name: "wrong-share.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(int64(len(payload))), UploadId: api.NewOptString("same-client-id")}, api.SharesCreateFileParams{ID: secondShareID})
	if statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 finalizing sibling share upload, got %d err=%v", statusCode(err), err)
	}
}

func TestSharesUpload_RejectsInvalidSizePartAndRateBeforeTelegram(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	_, ownerClient, _ := loginWithClient(t, s, 8508, "user8508")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910108), ChannelName: api.NewOptString("share-upload-validation")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "upload-validation-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	telegramCalls := 0
	s.tgMock.uploadPartFn = func(_ context.Context, _ *tg.Client, _ int64, _ string, _ io.Reader, fileSize int64, _ int) (int, int64, error) {
		telegramCalls++
		return 12800 + telegramCalls, fileSize, nil
	}

	_, status, raw := shareUploadPartRaw(t, s, shareID, "bad-part", "bad-part.txt", 0, false, []byte("x"))
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid partNo, got %d body=%s", status, string(raw))
	}
	status, raw = shareUploadPartHeaderOnlyRaw(t, s, shareID, "too-large", "too-large.txt", 1, false, 500*1024*1024+1)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized part, got %d body=%s", status, string(raw))
	}
	if telegramCalls != 0 {
		t.Fatalf("expected invalid requests to avoid Telegram calls, got %d", telegramCalls)
	}

	s.cfg.TG.RateLimit = true
	s.cfg.TG.RateBurst = 1
	s.cfg.TG.Rate = 60000
	_, status, raw = shareUploadPartRaw(t, s, shareID, "limited", "limited.txt", 1, false, []byte("ok"))
	if status != http.StatusOK {
		t.Fatalf("expected first rate-limited upload to pass, got %d body=%s", status, string(raw))
	}
	_, status, raw = shareUploadPartRaw(t, s, shareID, "limited", "limited.txt", 2, false, []byte("no"))
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected second rate-limited upload to fail 429, got %d body=%s", status, string(raw))
	}
	if telegramCalls != 1 {
		t.Fatalf("expected only first valid upload to call Telegram, got %d", telegramCalls)
	}
}

func TestSharesCreateFile_RejectsMismatchedStagedPartTotal(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, ownerClient, _ := loginWithClient(t, s, 8509, "user8509")

	if err := ownerClient.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910109), ChannelName: api.NewOptString("share-total-validation")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := ownerClient.FilesCreate(ctx, &api.File{Name: "total-validation-root", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, ownerClient, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})
	namespacedUploadID := "share:" + uuid.UUID(shareID).String() + ":bad-total"
	if err := s.repos.Uploads.Create(ctx, &jetmodel.Uploads{UploadID: namespacedUploadID, Name: "part", UserID: int64Ptr(8509), PartNo: 1, PartID: 12901, ChannelID: 910109, Size: 10}); err != nil {
		t.Fatalf("seed upload row: %v", err)
	}

	_, err = public.SharesCreateFile(ctx, &api.File{Name: "bad-total.txt", Type: api.FileTypeFile, Path: api.NewOptString("/"), MimeType: api.NewOptString("text/plain"), Size: api.NewOptInt64(11), UploadId: api.NewOptString("bad-total")}, api.SharesCreateFileParams{ID: shareID})
	if statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for staged total mismatch, got %d err=%v", statusCode(err), err)
	}
	rows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, namespacedUploadID, 8509)
	if err != nil {
		t.Fatalf("lookup mismatched staged rows: %v", err)
	}
	if len(rows) != 1 || rows[0].PartID != 12901 {
		t.Fatalf("expected mismatched validation to preserve staged row 12901, got %+v", rows)
	}
}

func shareUploadPartRaw(t *testing.T, s *harness, shareID api.UUID, uploadID, fileName string, partNo int, hashing bool, body []byte) (api.UploadPart, int, []byte) {
	t.Helper()
	return shareUploadPartWithLengthRaw(t, s, shareID, uploadID, fileName, partNo, hashing, bytes.NewReader(body), int64(len(body)))
}

func shareUploadPartWithLengthRaw(t *testing.T, s *harness, shareID api.UUID, uploadID, fileName string, partNo int, hashing bool, body io.Reader, contentLength int64) (api.UploadPart, int, []byte) {
	t.Helper()
	return shareUploadPartRawWithQuery(t, s, shareID, uploadID, fileName, partNo, hashing, body, contentLength, nil)
}

func shareUploadPartRawWithQuery(t *testing.T, s *harness, shareID api.UUID, uploadID, fileName string, partNo int, hashing bool, body io.Reader, contentLength int64, extraQuery url.Values) (api.UploadPart, int, []byte) {
	t.Helper()

	q := url.Values{}
	q.Set("fileName", fileName)
	q.Set("partNo", strconv.Itoa(partNo))
	q.Set("hashing", strconv.FormatBool(hashing))
	for key, values := range extraQuery {
		q.Del(key)
		for _, value := range values {
			q.Add(key, value)
		}
	}

	u := fmt.Sprintf("%s/shares/%s/uploads/%s?%s", s.server.URL, uuid.UUID(shareID).String(), url.PathEscape(uploadID), q.Encode())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, u, body)
	if err != nil {
		t.Fatalf("create share upload request: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = contentLength

	resp, err := s.httpCli.Do(req)
	if err != nil {
		t.Fatalf("execute share upload request: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read share upload response: %v", err)
	}

	var out api.UploadPart
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode share upload response: %v body=%s", err, string(raw))
		}
	}

	return out, resp.StatusCode, raw
}

func shareUploadPartHeaderOnlyRaw(t *testing.T, s *harness, shareID api.UUID, uploadID, fileName string, partNo int, hashing bool, contentLength int64) (int, []byte) {
	t.Helper()

	baseURL, err := url.Parse(s.server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	q := url.Values{}
	q.Set("fileName", fileName)
	q.Set("partNo", strconv.Itoa(partNo))
	q.Set("hashing", strconv.FormatBool(hashing))
	path := fmt.Sprintf("/shares/%s/uploads/%s?%s", uuid.UUID(shareID).String(), url.PathEscape(uploadID), q.Encode())

	conn, err := net.Dial("tcp", baseURL.Host)
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", path, baseURL.Host, contentLength); err != nil {
		t.Fatalf("write raw request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read raw response: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read raw response body: %v", err)
	}
	return resp.StatusCode, raw
}

func int64Ptr(v int64) *int64 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

func assertShareFinalizeRejectedPreserved(t *testing.T, ctx context.Context, s *harness, userID int64, shareID uuid.UUID, clientUploadID, fileName string, parentID uuid.UUID, partID int32) {
	t.Helper()

	namespacedUploadID := "share:" + shareID.String() + ":" + clientUploadID
	rows, err := s.repos.Uploads.GetByUploadIDAndUserID(ctx, namespacedUploadID, userID)
	if err != nil {
		t.Fatalf("lookup rejected staged rows: %v", err)
	}
	if len(rows) != 1 || rows[0].PartID != partID {
		t.Fatalf("expected rejected finalization to preserve staged part %d, got %+v", partID, rows)
	}
	_, err = s.repos.Files.GetActiveByNameAndParent(ctx, userID, fileName, &parentID)
	if !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("expected no finalized file %q, got err=%v", fileName, err)
	}
}

func createShareAndGetID(t *testing.T, ctx context.Context, client *api.Client, fileID api.UUID) api.UUID {
	t.Helper()

	return createShareWithRequestAndGetID(t, ctx, client, fileID, &api.FileShareCreate{})
}

func createShareWithRequestAndGetID(t *testing.T, ctx context.Context, client *api.Client, fileID api.UUID, req *api.FileShareCreate) api.UUID {
	t.Helper()

	if err := client.FilesCreateShare(ctx, req, api.FilesCreateShareParams{ID: fileID}); err != nil {
		t.Fatalf("FilesCreateShare failed: %v", err)
	}
	shares, err := client.FilesListShares(ctx, api.FilesListSharesParams{ID: fileID})
	if err != nil || len(shares) == 0 {
		t.Fatalf("FilesListShares failed: %v len=%d", err, len(shares))
	}
	for _, share := range shares {
		if req.AllowUpload.IsSet() && share.AllowUpload != req.AllowUpload.Value {
			continue
		}
		if req.Password.IsSet() && !share.Protected {
			continue
		}
		return share.ID
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

func TestFilesEditShare_PreservesAllowUploadWhenOmitted(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	public, client, _ := loginWithClient(t, s, 8510, "user8510")

	if err := client.UsersUpdateChannel(ctx, &api.ChannelUpdate{ChannelId: api.NewOptInt64(910110), ChannelName: api.NewOptString("edit-share-preserve-upload")}); err != nil {
		t.Fatalf("UsersUpdateChannel failed: %v", err)
	}
	folder, err := client.FilesCreate(ctx, &api.File{Name: "edit-share-upload-folder", Type: api.FileTypeFolder, Path: api.NewOptString("/")})
	if err != nil {
		t.Fatalf("FilesCreate folder failed: %v", err)
	}
	shareID := createShareWithRequestAndGetID(t, ctx, client, folder.ID.Value, &api.FileShareCreate{AllowUpload: api.NewOptBool(true)})

	if err := client.FilesEditShare(ctx, &api.FileShareCreate{Password: api.NewOptString("new-password")}, api.FilesEditShareParams{ID: folder.ID.Value, ShareId: shareID}); err != nil {
		t.Fatalf("FilesEditShare failed: %v", err)
	}
	share, err := public.SharesGetById(ctx, api.SharesGetByIdParams{ID: shareID})
	if err != nil {
		t.Fatalf("SharesGetById failed: %v", err)
	}
	if !share.AllowUpload {
		t.Fatal("expected password-only edit to preserve allowUpload")
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
