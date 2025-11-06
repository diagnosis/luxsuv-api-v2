package secure

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// these parameters are a secure baseline; you can tune later
	argonTime    = 1         // number of iterations
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4         // number of parallel threads
	argonKeyLen  = 32        // length of the derived key
)

// HashPassword returns an encoded Argon2id hash string
// Format: argon2id$v=19$t=1$m=65536$p=4$<salt>$<hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf("argon2id$v=19$t=%d$m=%d$p=%d$%s$%s",
		argonTime, argonMemory, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword checks whether a plaintext password matches a stored Argon2id hash.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	// expected format: argon2id$v=19$t=1$m=65536$p=4$<salt>$<hash>
	if len(parts) != 7 || parts[0] != "argon2id" {
		return false
	}

	tStr := strings.TrimPrefix(parts[2], "t=")
	mStr := strings.TrimPrefix(parts[3], "m=")
	pStr := strings.TrimPrefix(parts[4], "p=")

	t, _ := strconv.ParseUint(tStr, 10, 32)
	m, _ := strconv.ParseUint(mStr, 10, 32)
	p, _ := strconv.ParseUint(pStr, 10, 8)

	salt, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[6])
	if err != nil {
		return false
	}

	got := argon2.IDKey([]byte(password), salt, uint32(t), uint32(m), uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
