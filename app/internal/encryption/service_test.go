package encryption

import (
	"testing"
)

func TestEncryptDecryptAES(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes key
	originalText := []byte("Hello, World!")

	encrypted, err := EncryptAES(originalText, key)

	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	decrypted, err := DecryptAES(encrypted, key)

	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if string(decrypted) != string(originalText) {
		t.Errorf("expected %s, got %s", originalText, decrypted)
	}
}
