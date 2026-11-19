package auth

import "testing"

func TestPasswordAndTokenRoundTrip(t *testing.T) {
	hash, err := HashPassword("a-long-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "a-long-password-123") || CheckPassword(hash, "wrong-password-123") {
		t.Fatal("password verification did not round trip")
	}
	first, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 43 || first == second || TokenHash(first) == TokenHash(second) {
		t.Fatal("tokens are not unique random values")
	}
}

func TestRejectsShortPasswordsAndInvalidUsernames(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	for _, username := range []string{"a", "has spaces", "../../admin", "api", "this-name-is-far-too-long-for-the-limit"} {
		if _, err := ValidateUsername(username); err == nil {
			t.Errorf("invalid username %q accepted", username)
		}
	}
}
