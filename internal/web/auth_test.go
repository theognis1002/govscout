package web

import "testing"

func TestCheckPassword_AcceptsCorrect_RejectsVariants(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"exact match", "hunter2", true},
		{"one char different", "hunter3", false},
		{"case variant is rejected (bcrypt is case-sensitive)", "Hunter2", false},
		{"empty", "", false},
		{"trailing space not trimmed", "hunter2 ", false},
		{"leading space not trimmed", " hunter2", false},
		{"prefix only", "hunter", false},
		{"suffix only", "unter2", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CheckPassword(hash, tc.password); got != tc.want {
				t.Errorf("CheckPassword(%q) = %v, want %v", tc.password, got, tc.want)
			}
		})
	}
}

func TestCheckPassword_RejectsTamperedHash(t *testing.T) {
	// Guards against a regression where CheckPassword silently accepts any
	// password (e.g. if someone replaces bcrypt with `return true`) or where
	// hashes aren't actually validated.
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	tampered := []byte(hash)
	// Flip a byte in the middle of the hash body (past the "$2a$10$" header).
	tampered[len(tampered)/2] ^= 0x01
	if CheckPassword(string(tampered), "correct-horse-battery-staple") {
		t.Error("tampered hash must not validate against the original password")
	}
}

func TestCheckPassword_RejectsEmptyHash(t *testing.T) {
	// An empty/zero hash string must never validate — otherwise a user row
	// with an unset password_hash would be effectively passwordless.
	if CheckPassword("", "anything") {
		t.Error("empty hash must not validate")
	}
	if CheckPassword("not-a-bcrypt-hash", "anything") {
		t.Error("malformed hash must not validate")
	}
}
