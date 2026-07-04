package ratelimit

import "testing"

func TestHashIPDeterministic(t *testing.T) {
	first := HashIP("203.0.113.42", "test-secret")
	second := HashIP("203.0.113.42", "test-secret")
	if first == "" {
		t.Fatal("expected non-empty hash")
	}
	if first != second {
		t.Fatalf("hash changed between calls: %q vs %q", first, second)
	}
}

func TestHashIPDiffersBySecret(t *testing.T) {
	first := HashIP("203.0.113.42", "secret-a")
	second := HashIP("203.0.113.42", "secret-b")
	if first == second {
		t.Fatal("expected different hashes for different secrets")
	}
}

func TestHashIPDoesNotReturnPlainIP(t *testing.T) {
	ip := "203.0.113.42"
	hash := HashIP(ip, "test-secret")
	if hash == ip {
		t.Fatal("hash must not equal plain ip")
	}
}
