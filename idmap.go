package main

import "hash/fnv"

// ClerkIDToNumeric converts a Clerk string user ID (e.g. "user_2abc123")
// to a deterministic positive int64 using FNV-1a, masked to 53 bits.
// 53 bits ensures safe representation in JSON (JavaScript number precision).
func ClerkIDToNumeric(clerkID string) int64 {
	h := fnv.New64a()
	h.Write([]byte(clerkID))
	n := int64(h.Sum64() & 0x1FFFFFFFFFFFFF) // 53-bit mask
	if n == 0 {
		n = 1
	}
	return n
}
