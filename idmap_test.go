package main

import "testing"

func TestClerkIDToNumeric_Deterministic(t *testing.T) {
	id := "user_2abc123"
	a := ClerkIDToNumeric(id)
	b := ClerkIDToNumeric(id)
	if a != b {
		t.Errorf("not deterministic: %d != %d", a, b)
	}
}

func TestClerkIDToNumeric_DifferentInputs(t *testing.T) {
	ids := []string{"user_1", "user_2", "user_abc", "user_xyz", "user_2abc123"}
	seen := make(map[int64]string)
	for _, id := range ids {
		n := ClerkIDToNumeric(id)
		if prev, ok := seen[n]; ok {
			t.Errorf("collision: %q and %q both map to %d", prev, id, n)
		}
		seen[n] = id
	}
}

func TestClerkIDToNumeric_Positive(t *testing.T) {
	ids := []string{"user_1", "user_2", "", "a", "user_2xPq9LmKvN"}
	for _, id := range ids {
		n := ClerkIDToNumeric(id)
		if n <= 0 {
			t.Errorf("ClerkIDToNumeric(%q) = %d, want positive", id, n)
		}
	}
}

func TestClerkIDToNumeric_Within53Bits(t *testing.T) {
	ids := []string{"user_1", "user_2", "user_longid_12345678"}
	const max53 int64 = 1<<53 - 1
	for _, id := range ids {
		n := ClerkIDToNumeric(id)
		if n > max53 {
			t.Errorf("ClerkIDToNumeric(%q) = %d, exceeds 53-bit max %d", id, n, max53)
		}
	}
}
