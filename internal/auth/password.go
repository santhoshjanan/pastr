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
	"unicode/utf8"

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

	// Floors for parameters read back from a stored hash.
	minArgonMemory = 19 * 1024
	minSaltLen     = 8
	minKeyLen      = 16
)

// Each hash uses 64 MiB, so the number running at once is capped to keep a burst
// of sign-ins from exhausting memory.
var argonSlots = make(chan struct{}, 4)

func deriveKey(pw string, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
	argonSlots <- struct{}{}
	defer func() { <-argonSlots }()
	return argon2.IDKey([]byte(pw), salt, time, memory, threads, keyLen)
}

var usernameRE = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

func HashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := deriveKey(pw, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(encoded, pw string) bool {
	if len(pw) > maxPasswordLen {
		return false
	}
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
	// Refuse parameters far above ours, so a corrupted row cannot exhaust memory,
	// and ones too weak to be a hash this package produced.
	if memory > 4*argonMemory || memory < minArgonMemory || time > 10 || time < 1 || threads == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < minSaltLen {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < minKeyLen {
		return false
	}
	got := deriveKey(pw, salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// BurnTime does the work of a verification and discards it, so that signing in
// with an unknown username takes as long as signing in with a wrong password.
func BurnTime(pw string) {
	if len(pw) > maxPasswordLen {
		return
	}
	deriveKey(pw, make([]byte, argonSaltLen), argonTime, argonMemory, argonThreads, argonKeyLen)
}

func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordLen {
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
