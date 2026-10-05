// Package argon2 implements port.PasswordHasher with argon2id, storing hashes
// in the PHC string format ($argon2id$v=19$m=...,t=...,p=...$salt$hash) so
// parameters can be raised later without breaking existing hashes.
package argon2

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/CaioAP/shinrin/backend/internal/port"
)

// Params are argon2id cost parameters.
type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	SaltLen   int
	KeyLen    uint32
}

// DefaultParams follow RFC 9106's second recommended option (64 MiB, 3
// passes), which takes tens of milliseconds on a small server.
var DefaultParams = Params{MemoryKiB: 64 * 1024, Time: 3, Threads: 2, SaltLen: 16, KeyLen: 32}

// Hasher implements port.PasswordHasher.
type Hasher struct{ p Params }

var _ port.PasswordHasher = Hasher{}

// New returns a hasher with the given parameters.
func New(p Params) Hasher { return Hasher{p: p} }

var b64 = base64.RawStdEncoding

// Hash implements port.PasswordHasher.
func (h Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.p.Time, h.p.MemoryKiB, h.p.Threads, h.p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.p.MemoryKiB, h.p.Time, h.p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

var errFormat = errors.New("argon2: malformed hash")

// Verify implements port.PasswordHasher, using the parameters stored in the
// hash rather than the current ones.
func (h Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errFormat
	}
	var v int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return false, errFormat
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return false, errFormat
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errFormat
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errFormat
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
