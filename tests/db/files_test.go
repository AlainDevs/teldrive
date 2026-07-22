package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	jetmodel "github.com/tgdrive/teldrive/internal/database/jet/gen/model"
	"github.com/tgdrive/teldrive/pkg/repositories"
)

func TestFileCreateAndGetByID(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7001)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	parentID, err := fileRepo.CreateDirectories(ctx, uid, "/test-create")
	if err != nil {
		t.Fatalf("create parent dir: %v", err)
	}

	active := "active"
	size := int64(1024)
	now := time.Now().UTC()
	fileID := uuid.New()

	file := &jetmodel.Files{
		ID:        fileID,
		Name:      "test.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  parentID,
		Status:    &active,
		Size:      &size,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.ID != fileID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, fileID)
	}
	if got.Name != "test.txt" {
		t.Errorf("Name mismatch: got %s, want test.txt", got.Name)
	}
	if got.Type != "file" {
		t.Errorf("Type mismatch: got %s, want file", got.Type)
	}
	if got.MimeType != "text/plain" {
		t.Errorf("MimeType mismatch: got %s, want text/plain", got.MimeType)
	}
	if got.UserID != uid {
		t.Errorf("UserID mismatch: got %d, want %d", got.UserID, uid)
	}
	if got.Status == nil || *got.Status != "active" {
		t.Errorf("Status mismatch: got %v, want active", got.Status)
	}
	if got.Size == nil || *got.Size != 1024 {
		t.Errorf("Size mismatch: got %v, want 1024", got.Size)
	}
	if got.Encrypted {
		t.Errorf("Encrypted mismatch: got true, want false")
	}
}

func TestFileGetByID_NotFound(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()

	fileRepo := repositories.NewJetFileRepository(s.pool)

	_, err := fileRepo.GetByID(ctx, uuid.New())
	if err != repositories.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileGetByIDAndUser(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7003)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	parentID, err := fileRepo.CreateDirectories(ctx, uid, "/test-user")
	if err != nil {
		t.Fatalf("create parent dir: %v", err)
	}

	active := "active"
	size := int64(512)
	now := time.Now().UTC()
	fileID := uuid.New()

	file := &jetmodel.Files{
		ID:        fileID,
		Name:      "user-file.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  parentID,
		Status:    &active,
		Size:      &size,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Should succeed with correct user
	got, err := fileRepo.GetByIDAndUser(ctx, fileID, uid)
	if err != nil {
		t.Fatalf("GetByIDAndUser with correct user failed: %v", err)
	}
	if got.ID != fileID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, fileID)
	}
	if got.UserID != uid {
		t.Errorf("UserID mismatch: got %d, want %d", got.UserID, uid)
	}

	// Should fail with wrong user
	_, err = fileRepo.GetByIDAndUser(ctx, fileID, 9999)
	if err != repositories.ErrNotFound {
		t.Fatalf("expected ErrNotFound for wrong user, got %v", err)
	}
}

func TestFileGetByIDAndUserInSubtree(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7013)
	otherUID := int64(7014)
	s.ensureUserExists(uid)
	s.ensureUserExists(otherUID)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	rootID, err := fileRepo.CreateDirectories(ctx, uid, "/share-root")
	if err != nil {
		t.Fatalf("create root dir: %v", err)
	}
	nestedID, err := fileRepo.CreateDirectories(ctx, uid, "/share-root/nested")
	if err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	siblingID, err := fileRepo.CreateDirectories(ctx, uid, "/share-sibling")
	if err != nil {
		t.Fatalf("create sibling dir: %v", err)
	}
	otherRootID, err := fileRepo.CreateDirectories(ctx, otherUID, "/share-root")
	if err != nil {
		t.Fatalf("create other root dir: %v", err)
	}
	otherNestedID, err := fileRepo.CreateDirectories(ctx, otherUID, "/share-root/nested")
	if err != nil {
		t.Fatalf("create other nested dir: %v", err)
	}

	directFileID := createTestFile(t, ctx, fileRepo, uid, "direct.txt", rootID, "active")
	nestedFileID := createTestFile(t, ctx, fileRepo, uid, "nested.txt", nestedID, "active")
	siblingFileID := createTestFile(t, ctx, fileRepo, uid, "sibling.txt", siblingID, "active")
	otherUserFileID := createTestFile(t, ctx, fileRepo, otherUID, "other.txt", otherNestedID, "active")
	inactiveFileID := createTestFile(t, ctx, fileRepo, uid, "inactive.txt", nestedID, "pending_deletion")

	got, err := fileRepo.GetByIDAndUserInSubtree(ctx, directFileID, uid, *rootID)
	if err != nil {
		t.Fatalf("direct descendant lookup failed: %v", err)
	}
	if got.ID != directFileID {
		t.Fatalf("direct descendant ID mismatch: got %v want %v", got.ID, directFileID)
	}

	got, err = fileRepo.GetByIDAndUserInSubtree(ctx, nestedFileID, uid, *rootID)
	if err != nil {
		t.Fatalf("nested descendant lookup failed: %v", err)
	}
	if got.ID != nestedFileID {
		t.Fatalf("nested descendant ID mismatch: got %v want %v", got.ID, nestedFileID)
	}

	for name, fileID := range map[string]uuid.UUID{
		"sibling":            siblingFileID,
		"other user":         otherUserFileID,
		"wrong-owner root":   nestedFileID,
		"inactive requested": inactiveFileID,
	} {
		t.Run(name, func(t *testing.T) {
			root := *rootID
			userID := uid
			if name == "wrong-owner root" {
				root = *otherRootID
			}
			_, err := fileRepo.GetByIDAndUserInSubtree(ctx, fileID, userID, root)
			if err != repositories.ErrNotFound {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}
		})
	}

	if err := fileRepo.Update(ctx, *rootID, repositories.FileUpdate{ParentID: nestedID}); err != nil {
		t.Fatalf("create root/nested folder cycle: %v", err)
	}
	cycleCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	got, err = fileRepo.GetByIDAndUserInSubtree(cycleCtx, nestedFileID, uid, *rootID)
	if err != nil {
		t.Fatalf("cyclic subtree lookup failed: %v", err)
	}
	if got.ID != nestedFileID {
		t.Fatalf("cyclic subtree ID mismatch: got %v want %v", got.ID, nestedFileID)
	}
}

func createTestFile(t *testing.T, ctx context.Context, fileRepo repositories.FileRepository, userID int64, name string, parentID *uuid.UUID, status string) uuid.UUID {
	t.Helper()

	now := time.Now().UTC()
	size := int64(12)
	fileID := uuid.New()
	file := &jetmodel.Files{
		ID:        fileID,
		Name:      name,
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    userID,
		ParentID:  parentID,
		Status:    &status,
		Size:      &size,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return fileID
}

func TestFileUpdateName(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7004)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	parentID, err := fileRepo.CreateDirectories(ctx, uid, "/test-update-name")
	if err != nil {
		t.Fatalf("create parent dir: %v", err)
	}

	active := "active"
	now := time.Now().UTC()
	fileID := uuid.New()

	file := &jetmodel.Files{
		ID:        fileID,
		Name:      "old-name.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  parentID,
		Status:    &active,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	newName := "new-name.txt"
	if err := fileRepo.Update(ctx, fileID, repositories.FileUpdate{Name: &newName}); err != nil {
		t.Fatalf("Update name failed: %v", err)
	}

	got, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if got.Name != "new-name.txt" {
		t.Errorf("Name not updated: got %s, want new-name.txt", got.Name)
	}
}

func TestFileUpdateSize(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7005)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	parentID, err := fileRepo.CreateDirectories(ctx, uid, "/test-update-size")
	if err != nil {
		t.Fatalf("create parent dir: %v", err)
	}

	active := "active"
	initialSize := int64(100)
	now := time.Now().UTC()
	fileID := uuid.New()

	file := &jetmodel.Files{
		ID:        fileID,
		Name:      "size-test.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  parentID,
		Status:    &active,
		Size:      &initialSize,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	newSize := int64(2048)
	if err := fileRepo.Update(ctx, fileID, repositories.FileUpdate{Size: &newSize}); err != nil {
		t.Fatalf("Update size failed: %v", err)
	}

	got, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if got.Size == nil || *got.Size != 2048 {
		t.Errorf("Size not updated: got %v, want 2048", got.Size)
	}
}

func TestFileUpdateStatus(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7006)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	parentID, err := fileRepo.CreateDirectories(ctx, uid, "/test-update-status")
	if err != nil {
		t.Fatalf("create parent dir: %v", err)
	}

	active := "active"
	now := time.Now().UTC()
	fileID := uuid.New()

	file := &jetmodel.Files{
		ID:        fileID,
		Name:      "status-test.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  parentID,
		Status:    &active,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify initial state
	got, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		t.Fatalf("GetByID before update failed: %v", err)
	}
	if got.Status == nil || *got.Status != "active" {
		t.Errorf("Initial status mismatch: got %v, want active", got.Status)
	}

	// Update status to pending_deletion
	pending := "pending_deletion"
	updated, err := fileRepo.UpdateReturning(ctx, fileID, repositories.FileUpdate{Status: &pending})
	if err != nil {
		t.Fatalf("Update status failed: %v", err)
	}
	if updated.Status == nil || *updated.Status != "pending_deletion" {
		t.Errorf("Status not updated via UpdateReturning: got %v, want pending_deletion", updated.Status)
	}

	// GetByID filters on status=active, so it should return ErrNotFound now
	_, err = fileRepo.GetByID(ctx, fileID)
	if err != repositories.ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-active file via GetByID, got %v", err)
	}
}

func TestFileDelete(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7007)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	parentID, err := fileRepo.CreateDirectories(ctx, uid, "/test-delete")
	if err != nil {
		t.Fatalf("create parent dir: %v", err)
	}

	active := "active"
	now := time.Now().UTC()
	fileID := uuid.New()

	file := &jetmodel.Files{
		ID:        fileID,
		Name:      "delete-me.txt",
		Type:      "file",
		MimeType:  "text/plain",
		UserID:    uid,
		ParentID:  parentID,
		Status:    &active,
		Encrypted: false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify exists
	if _, err := fileRepo.GetByID(ctx, fileID); err != nil {
		t.Fatalf("GetByID before delete failed: %v", err)
	}

	// Delete
	if err := fileRepo.Delete(ctx, []uuid.UUID{fileID}); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify gone
	_, err = fileRepo.GetByID(ctx, fileID)
	if err != repositories.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFileCreateDirectories(t *testing.T) {
	s := newHarness(t)
	ctx := context.Background()
	uid := int64(7008)
	s.ensureUserExists(uid)

	fileRepo := repositories.NewJetFileRepository(s.pool)

	// Create nested directories
	cID, err := fileRepo.CreateDirectories(ctx, uid, "/a/b/c")
	if err != nil {
		t.Fatalf("CreateDirectories failed: %v", err)
	}
	if cID == nil {
		t.Fatal("CreateDirectories returned nil ID")
	}

	// Resolve each path level
	aID, err := fileRepo.ResolvePathID(ctx, "/a", uid)
	if err != nil {
		t.Fatalf("ResolvePathID /a failed: %v", err)
	}
	if aID == nil {
		t.Fatal("ResolvePathID /a returned nil")
	}

	bID, err := fileRepo.ResolvePathID(ctx, "/a/b", uid)
	if err != nil {
		t.Fatalf("ResolvePathID /a/b failed: %v", err)
	}
	if bID == nil {
		t.Fatal("ResolvePathID /a/b returned nil")
	}

	resolvedCID, err := fileRepo.ResolvePathID(ctx, "/a/b/c", uid)
	if err != nil {
		t.Fatalf("ResolvePathID /a/b/c failed: %v", err)
	}
	if resolvedCID == nil {
		t.Fatal("ResolvePathID /a/b/c returned nil")
	}

	// Verify CreateDirectories result matches the resolved last segment
	if *cID != *resolvedCID {
		t.Errorf("CreateDirectories result %v does not match ResolvePathID /a/b/c %v", *cID, *resolvedCID)
	}

	// Verify hierarchy: a has a parent (root), b's parent is a, c's parent is b
	a, err := fileRepo.GetByID(ctx, *aID)
	if err != nil {
		t.Fatalf("GetByID /a failed: %v", err)
	}
	if a.ParentID == nil {
		t.Errorf("expected /a to have a parent (root)")
	}

	b, err := fileRepo.GetByID(ctx, *bID)
	if err != nil {
		t.Fatalf("GetByID /a/b failed: %v", err)
	}
	if b.ParentID == nil || *b.ParentID != *aID {
		t.Errorf("expected /a/b parent to be /a (%v), got %v", *aID, b.ParentID)
	}

	c, err := fileRepo.GetByID(ctx, *cID)
	if err != nil {
		t.Fatalf("GetByID /a/b/c failed: %v", err)
	}
	if c.ParentID == nil || *c.ParentID != *bID {
		t.Errorf("expected /a/b/c parent to be /a/b (%v), got %v", *bID, c.ParentID)
	}
}
