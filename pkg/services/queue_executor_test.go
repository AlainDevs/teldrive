package services

import (
	"testing"

	"github.com/tgdrive/teldrive/pkg/repositories"
)

func TestGroupPendingFilesKeepsUnserviceableRowsUnresolved(t *testing.T) {
	channelID := int64(123)
	validParts := `[{"id":42}]`
	rows := []repositories.PendingFile{
		{ID: "valid", UserID: 7, ChannelID: &channelID, Parts: &validParts},
		{ID: "missing-session", UserID: 8, ChannelID: &channelID, Parts: &validParts},
		{ID: "missing-channel", UserID: 7, Parts: &validParts},
		{ID: "missing-parts", UserID: 7, ChannelID: &channelID},
	}

	groups, unresolved := groupPendingFiles(rows, map[int64]string{7: "session"})
	if len(groups) != 1 {
		t.Fatalf("group count mismatch: got %d want 1", len(groups))
	}
	if len(unresolved) != 3 {
		t.Fatalf("unresolved count mismatch: got %d want 3", len(unresolved))
	}
	for _, group := range groups {
		if len(group.fileIDs) != 1 || group.fileIDs[0] != "valid" {
			t.Fatalf("unexpected grouped files: %+v", group.fileIDs)
		}
		if len(group.partIDs) != 1 || group.partIDs[0] != 42 {
			t.Fatalf("unexpected grouped parts: %+v", group.partIDs)
		}
	}
}
