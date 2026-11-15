package infrastructure

import "testing"

func TestPBKDF2Hasher(t *testing.T) {
	hasher := NewPBKDF2Hasher(1_000)
	encoded, err := hasher.Hash("123456")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if !hasher.Verify("123456", encoded) {
		t.Fatal("expected Verify to accept matching PIN")
	}
	if hasher.Verify("654321", encoded) {
		t.Fatal("expected Verify to reject wrong PIN")
	}
	for _, malformed := range []string{"", "bcrypt$10$a$b", "pbkdf2-sha256$0$aa$bb", "pbkdf2-sha256$10$not_b64$bb", "pbkdf2-sha256$10$aa$!!"} {
		if hasher.Verify("123456", malformed) {
			t.Errorf("expected Verify to reject malformed hash %q", malformed)
		}
	}
}
