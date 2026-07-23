package crypt

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestDecryptDataSeekResumesPlaintextRanges(t *testing.T) {
	plaintext := make([]byte, 3*blockDataSize+123)
	for i := range plaintext {
		plaintext[i] = byte(i % 251)
	}
	cipher, err := NewCipher("test-encryption-key", "test-salt")
	if err != nil {
		t.Fatalf("NewCipher failed: %v", err)
	}
	encryptedReader, err := cipher.EncryptData(bytes.NewReader(plaintext))
	if err != nil {
		t.Fatalf("EncryptData failed: %v", err)
	}
	encrypted, err := io.ReadAll(encryptedReader)
	if err != nil {
		t.Fatalf("read encrypted data: %v", err)
	}
	if err := encryptedReader.Close(); err != nil {
		t.Fatalf("close encrypted reader: %v", err)
	}

	open := func(_ context.Context, offset, limit int64) (io.ReadCloser, error) {
		if limit < 0 {
			limit = int64(len(encrypted)) - offset
		}
		return io.NopCloser(io.NewSectionReader(bytes.NewReader(encrypted), offset, limit)), nil
	}
	for _, test := range []struct {
		name   string
		offset int64
		limit  int64
	}{
		{name: "inside first block", offset: 1024, limit: 4096},
		{name: "cross block boundary", offset: blockDataSize - 17, limit: 64},
		{name: "later block", offset: 2*blockDataSize + 31, limit: 8192},
		{name: "near tail", offset: int64(len(plaintext) - 100), limit: 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			decrypted, err := cipher.DecryptDataSeek(context.Background(), open, test.offset, test.limit)
			if err != nil {
				t.Fatalf("DecryptDataSeek failed: %v", err)
			}
			got, err := io.ReadAll(decrypted)
			if err != nil {
				t.Fatalf("read decrypted range: %v", err)
			}
			if err := decrypted.Close(); err != nil {
				t.Fatalf("close decrypted range: %v", err)
			}
			want := plaintext[test.offset : test.offset+test.limit]
			if !bytes.Equal(got, want) {
				t.Fatalf("decrypted range mismatch: got %d bytes, want %d", len(got), len(want))
			}
		})
	}

	decrypted, err := cipher.DecryptDataSeek(context.Background(), open, 0, -1)
	if err != nil {
		t.Fatalf("DecryptDataSeek full stream failed: %v", err)
	}
	seekOffset := int64(blockDataSize + 37)
	if _, err := decrypted.Seek(seekOffset, io.SeekStart); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	got, err := io.ReadAll(decrypted)
	if err != nil {
		t.Fatalf("read open-ended decrypted range: %v", err)
	}
	if err := decrypted.Close(); err != nil {
		t.Fatalf("close open-ended decrypted range: %v", err)
	}
	if !bytes.Equal(got, plaintext[seekOffset:]) {
		t.Fatalf("open-ended decrypted range mismatch: got %d bytes, want %d", len(got), len(plaintext)-int(seekOffset))
	}
}
