// Package auth handles passwords, sessions and sign-in throttling.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16

	MinPasswordLen = 10
	maxPasswordLen = 1024
)

var usernameRE = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

func HashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	// Refuse parameters far above ours, so a corrupted row cannot exhaust memory.
	if memory > 4*argonMemory || time > 10 || threads == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// BurnTime does the work of a verification and discards it, so that signing in
// with an unknown username takes as long as signing in with a wrong password.
func BurnTime(pw string) {
	argon2.IDKey([]byte(pw), make([]byte, argonSaltLen), argonTime, argonMemory, argonThreads, argonKeyLen)
}

func ValidatePassword(pw string) error {
	if len(pw) < MinPasswordLen {
		return fmt.Errorf("Use at least %d characters.", MinPasswordLen)
	}
	if len(pw) > maxPasswordLen {
		return errors.New("That password is too long.")
	}
	return nil
}

// ValidateUsername trims and lower-cases the name, then checks its shape.
func ValidateUsername(raw string) (string, error) {
	u := strings.ToLower(strings.TrimSpace(raw))
	if !usernameRE.MatchString(u) {
		return "", errors.New("Usernames are 3 to 32 characters: letters, numbers, dashes and underscores.")
	}
	return u, nil
}
