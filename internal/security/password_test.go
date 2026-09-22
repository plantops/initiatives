package security

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("a-long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "a-long-test-password") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(h, "wrong-password") {
		t.Fatal("wrong password accepted")
	}
}
