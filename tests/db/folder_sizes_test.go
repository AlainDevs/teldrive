package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	jetmodel "github.com/tgdrive/teldrive/internal/database/jet/gen/model"
	"github.com/tgdrive/teldrive/pkg/repositories"
)

func TestFolderSizeRefresh_FileLifecycle(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7610)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	bID, err := fileRepo.CreateDirectories(ctx, uid, "/a/b")
	if err != nil {
		t.Fatalf("create directories /a/b: %v", err)
	}
	aID, err := fileRepo.ResolvePathID(ctx, "/a", uid)
	if err != nil {
		t.Fatalf("resolve /a: %v", err)
	}
	rootID, err := fileRepo.ResolvePathID(ctx, "/root", uid)
	if err != nil {
		t.Fatalf("resolve /root: %v", err)
	}

	active := "active"
	sz := int64(10)
	fileID := uuid.New()
	now := time.Now().UTC()
	if err := fileRepo.Create(ctx, &jetmodel.Files{
		ID:        fileID,
		Name:      "f.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  bID,
		Status:    &active,
		Size:      &sz,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create file: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid, *bID, 10)
	assertFolderSize(t, fileRepo, ctx, uid, *aID, 10)
	assertFolderSize(t, fileRepo, ctx, uid, *rootID, 10)

	newSize := int64(25)
	if err := fileRepo.Update(ctx, fileID, repositories.FileUpdate{Size: &newSize}); err != nil {
		t.Fatalf("update file size: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid, *bID, 25)
	assertFolderSize(t, fileRepo, ctx, uid, *aID, 25)
	assertFolderSize(t, fileRepo, ctx, uid, *rootID, 25)

	pending := "pending_deletion"
	if err := fileRepo.Update(ctx, fileID, repositories.FileUpdate{Status: &pending}); err != nil {
		t.Fatalf("update file status to pending_deletion: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid, *bID, 0)
	assertFolderSize(t, fileRepo, ctx, uid, *aID, 0)
	assertFolderSize(t, fileRepo, ctx, uid, *rootID, 0)

	if err := fileRepo.Update(ctx, fileID, repositories.FileUpdate{Status: &active}); err != nil {
		t.Fatalf("update file status back to active: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid, *bID, 25)
	assertFolderSize(t, fileRepo, ctx, uid, *aID, 25)
	assertFolderSize(t, fileRepo, ctx, uid, *rootID, 25)

	if err := fileRepo.Delete(ctx, []uuid.UUID{fileID}); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid, *bID, 0)
	assertFolderSize(t, fileRepo, ctx, uid, *aID, 0)
	assertFolderSize(t, fileRepo, ctx, uid, *rootID, 0)

	folderID := uuid.New()
	if err := fileRepo.Create(ctx, &jetmodel.Files{
		ID:        folderID,
		Name:      "child-folder",
		Type:      "folder",
		MimeType:  "drive/folder",
		UserID:    uid,
		ParentID:  bID,
		Status:    &active,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create folder child: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid, *bID, 0)
	assertFolderSize(t, fileRepo, ctx, uid, *aID, 0)
	assertFolderSize(t, fileRepo, ctx, uid, *rootID, 0)
}

func TestFolderSizeRefresh_MoveAndBulkCases(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid1 := int64(7611)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	srcID, err := fileRepo.CreateDirectories(ctx, uid1, "/src")
	if err != nil {
		t.Fatalf("create /src: %v", err)
	}
	dstID, err := fileRepo.CreateDirectories(ctx, uid1, "/dst")
	if err != nil {
		t.Fatalf("create /dst: %v", err)
	}
	root1ID, err := fileRepo.ResolvePathID(ctx, "/root", uid1)
	if err != nil {
		t.Fatalf("resolve /root user1: %v", err)
	}

	active := "active"
	fileSize := int64(40)
	fileID := uuid.New()
	now := time.Now().UTC()
	if err := fileRepo.Create(ctx, &jetmodel.Files{
		ID:        fileID,
		Name:      "move.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid1,
		ParentID:  srcID,
		Status:    &active,
		Size:      &fileSize,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create move file: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid1, *srcID, 40)
	assertFolderSize(t, fileRepo, ctx, uid1, *dstID, 0)
	assertFolderSize(t, fileRepo, ctx, uid1, *root1ID, 40)

	if err := fileRepo.Update(ctx, fileID, repositories.FileUpdate{ParentID: dstID}); err != nil {
		t.Fatalf("move file between parents: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid1, *srcID, 0)
	assertFolderSize(t, fileRepo, ctx, uid1, *dstID, 40)
	assertFolderSize(t, fileRepo, ctx, uid1, *root1ID, 40)

	p1ID, err := fileRepo.CreateDirectories(ctx, uid1, "/p1")
	if err != nil {
		t.Fatalf("create /p1: %v", err)
	}
	p2ID, err := fileRepo.CreateDirectories(ctx, uid1, "/p2")
	if err != nil {
		t.Fatalf("create /p2: %v", err)
	}
	subtreeID := uuid.New()
	if err := fileRepo.Create(ctx, &jetmodel.Files{
		ID:        subtreeID,
		Name:      "subtree",
		Type:      "folder",
		MimeType:  "drive/folder",
		UserID:    uid1,
		ParentID:  p1ID,
		Status:    &active,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create subtree folder: %v", err)
	}

	f1Size := int64(10)
	f2Size := int64(15)
	f1ID := uuid.New()
	f2ID := uuid.New()
	for _, item := range []struct {
		id   uuid.UUID
		name string
		size int64
	}{{f1ID, "f1.txt", f1Size}, {f2ID, "f2.txt", f2Size}} {
		sz := item.size
		if err := fileRepo.Create(ctx, &jetmodel.Files{
			ID:        item.id,
			Name:      item.name,
			Type:      "file",
			MimeType:  "text/plain",
			UserID:    uid1,
			ParentID:  &subtreeID,
			Status:    &active,
			Size:      &sz,
			Encrypted: false,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create subtree file %s: %v", item.name, err)
		}
	}
	assertFolderSize(t, fileRepo, ctx, uid1, subtreeID, 25)
	assertFolderSize(t, fileRepo, ctx, uid1, *p1ID, 25)
	assertFolderSize(t, fileRepo, ctx, uid1, *p2ID, 0)
	assertFolderSize(t, fileRepo, ctx, uid1, *root1ID, 65)

	if err := fileRepo.Update(ctx, subtreeID, repositories.FileUpdate{ParentID: p2ID}); err != nil {
		t.Fatalf("move subtree folder: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid1, subtreeID, 25)
	assertFolderSize(t, fileRepo, ctx, uid1, *p1ID, 0)
	assertFolderSize(t, fileRepo, ctx, uid1, *p2ID, 25)
	assertFolderSize(t, fileRepo, ctx, uid1, *root1ID, 65)

	for _, item := range []struct {
		id   uuid.UUID
		size int64
	}{{f1ID, 15}, {f2ID, 20}} {
		sz := item.size
		if err := fileRepo.Update(ctx, item.id, repositories.FileUpdate{Size: &sz}); err != nil {
			t.Fatalf("update subtree file size: %v", err)
		}
	}
	assertFolderSize(t, fileRepo, ctx, uid1, subtreeID, 35)
	assertFolderSize(t, fileRepo, ctx, uid1, *p2ID, 35)
	assertFolderSize(t, fileRepo, ctx, uid1, *root1ID, 75)

	if err := fileRepo.Delete(ctx, []uuid.UUID{f1ID, f2ID}); err != nil {
		t.Fatalf("bulk delete subtree files: %v", err)
	}
	assertFolderSize(t, fileRepo, ctx, uid1, subtreeID, 0)
	assertFolderSize(t, fileRepo, ctx, uid1, *p2ID, 0)
	assertFolderSize(t, fileRepo, ctx, uid1, *root1ID, 40)
}

func TestIncrementActiveAncestorFolderSizesScopesToActiveUserFolders(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid1 := int64(7612)
	uid2 := int64(7613)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	active := "active"
	pending := "pending_deletion"
	now := time.Now().UTC()

	root1ID, err := fileRepo.CreateDirectories(ctx, uid1, "/root")
	if err != nil {
		t.Fatalf("create user1 root: %v", err)
	}
	activeParentID, err := fileRepo.CreateDirectories(ctx, uid1, "/root/active-parent")
	if err != nil {
		t.Fatalf("create active parent: %v", err)
	}
	otherUserRootID, err := fileRepo.CreateDirectories(ctx, uid2, "/root")
	if err != nil {
		t.Fatalf("create user2 root: %v", err)
	}

	pendingParentID := uuid.New()
	if err := fileRepo.Create(ctx, &jetmodel.Files{
		ID:        pendingParentID,
		Name:      "pending-parent",
		Type:      "folder",
		MimeType:  "drive/folder",
		UserID:    uid1,
		ParentID:  root1ID,
		Status:    &pending,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create pending parent: %v", err)
	}

	fileAsParentID := uuid.New()
	fileSize := int64(99)
	if err := fileRepo.Create(ctx, &jetmodel.Files{
		ID:        fileAsParentID,
		Name:      "file-as-parent.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid1,
		ParentID:  root1ID,
		Status:    &active,
		Size:      &fileSize,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create file-as-parent: %v", err)
	}

	if err := fileRepo.IncrementActiveAncestorFolderSizes(ctx, uid1, *activeParentID, 17); err != nil {
		t.Fatalf("increment active ancestors: %v", err)
	}
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *activeParentID, 17)
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *root1ID, 17)
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *otherUserRootID, 0)

	if err := fileRepo.IncrementActiveAncestorFolderSizes(ctx, uid2, *activeParentID, 11); err != nil {
		t.Fatalf("increment wrong user ancestors: %v", err)
	}
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *activeParentID, 17)
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *root1ID, 17)
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *otherUserRootID, 0)

	if err := fileRepo.IncrementActiveAncestorFolderSizes(ctx, uid1, pendingParentID, 13); err != nil {
		t.Fatalf("increment pending parent ancestors: %v", err)
	}
	if err := fileRepo.Update(ctx, pendingParentID, repositories.FileUpdate{Status: &active}); err != nil {
		t.Fatalf("reactivate pending parent for size assertion: %v", err)
	}
	assertFolderSizeNoRefresh(t, fileRepo, ctx, pendingParentID, 0)
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *root1ID, 17)

	if err := fileRepo.IncrementActiveAncestorFolderSizes(ctx, uid1, fileAsParentID, 19); err != nil {
		t.Fatalf("increment file parent ancestors: %v", err)
	}
	assertFolderSizeNoRefresh(t, fileRepo, ctx, fileAsParentID, fileSize)
	assertFolderSizeNoRefresh(t, fileRepo, ctx, *root1ID, 17)
}

func assertFolderSize(t *testing.T, repo repositories.FileRepository, ctx context.Context, userID int64, folderID uuid.UUID, expected int64) {
	t.Helper()
	if err := repo.RefreshFolderSizesByUser(ctx, userID); err != nil {
		t.Fatalf("refresh folder sizes for user %d: %v", userID, err)
	}
	row, err := repo.GetByID(ctx, folderID)
	if err != nil {
		t.Fatalf("get folder %s: %v", folderID, err)
	}
	var actual int64
	if row.Size != nil {
		actual = *row.Size
	}
	if actual != expected {
		t.Fatalf("folder %s size mismatch: got %d want %d", folderID, actual, expected)
	}
}

func assertFolderSizeNoRefresh(t *testing.T, repo repositories.FileRepository, ctx context.Context, folderID uuid.UUID, expected int64) {
	t.Helper()
	row, err := repo.GetByID(ctx, folderID)
	if err != nil {
		t.Fatalf("get folder %s: %v", folderID, err)
	}
	var actual int64
	if row.Size != nil {
		actual = *row.Size
	}
	if actual != expected {
		t.Fatalf("folder %s size mismatch without refresh: got %d want %d", folderID, actual, expected)
	}
}
