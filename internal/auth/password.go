package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	MinPasswordLength = 10

	maxPasswordBytes = 1024
	saltLength       = 16
	keyLength        = 32
)

type Hasher struct {
	Memory  uint32
	Time    uint32
	Threads uint8
}

var DefaultHasher = Hasher{Memory: 19 * 1024, Time: 2, Threads: 1}

const (
	maxStoredMemory  = 256 * 1024
	maxStoredTime    = 10
	maxStoredThreads = 8
)

// Hash returns the encoded hash of a password.
func (h Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: random salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.Time, h.Memory, h.Threads, keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.Memory, h.Time, h.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify tells whether a password matches its hash.
func (h Hasher) Verify(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	if memory == 0 || memory > maxStoredMemory || time == 0 || time > maxStoredTime || threads == 0 || threads > maxStoredThreads {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(salt) == 0 || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ValidatePassword checks the password's length.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return &Problem{Status: 400, Code: "weak_password", Detail: fmt.Sprintf("the password must have at least %d characters", MinPasswordLength)}
	}
	if len(password) > maxPasswordBytes {
		return &Problem{Status: 400, Code: "invalid_password", Detail: "the password is too long"}
	}
	return nil
}

const (
	minUsernameLength = 3
	maxUsernameLength = 30
)

// ValidateUsername trims and checks a username.
func ValidateUsername(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	bad := &Problem{Status: 400, Code: "invalid_username", Detail: "the username must have 3 to 30 characters: letters, digits, and . _ - inside"}
	if len(name) < minUsernameLength || len(name) > maxUsernameLength {
		return "", bad
	}
	for i, r := range name {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		isSeparator := r == '.' || r == '_' || r == '-'
		edge := i == 0 || i == len(name)-1
		if !isAlnum && (!isSeparator || edge) {
			return "", bad
		}
	}
	return name, nil
}

func usernameKey(name string) string { return strings.ToLower(name) }
