package auth

import (
	"strings"
	"testing"
)

var testHasher = Hasher{Memory: 64, Time: 1, Threads: 1}

func TestAPasswordIsHashedAndVerified(t *testing.T) {
	hash, err := testHasher.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$") || strings.Contains(hash, "correct horse") {
		t.Errorf("hash = %q", hash)
	}
	if !testHasher.Verify("correct horse battery", hash) {
		t.Error("the right password must verify")
	}
	if testHasher.Verify("correct horse batterY", hash) || testHasher.Verify("", hash) {
		t.Error("another password must not verify")
	}
}

func TestTwoHashesOfTheSamePasswordDiffer(t *testing.T) {
	a, _ := testHasher.Hash("the same password")
	b, _ := testHasher.Hash("the same password")

	if a == b {
		t.Error("the salt must be random")
	}
}

func TestTheCostIsReadFromTheHashSoItCanBeRaisedLater(t *testing.T) {
	old, _ := Hasher{Memory: 64, Time: 1, Threads: 1}.Hash("a password")
	stronger := Hasher{Memory: 128, Time: 2, Threads: 1}

	if !stronger.Verify("a password", old) {
		t.Error("a hash made with a lower cost must still verify")
	}
}

func TestAMalformedOrHostileHashIsSimplyNo(t *testing.T) {
	good, _ := testHasher.Hash("a password")
	for name, hash := range map[string]string{
		"empty":            "",
		"not argon":        "$2a$10$abcdefghijklmnopqrstuuxyz",
		"missing parts":    "$argon2id$v=19$m=64,t=1,p=1$c2FsdA",
		"bad base64":       "$argon2id$v=19$m=64,t=1,p=1$!!!$!!!",
		"another version":  strings.Replace(good, "v=19", "v=16", 1),
		"memory too large": "$argon2id$v=19$m=4194304,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5",
		"time too large":   "$argon2id$v=19$m=64,t=1000,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5",
		"zero memory":      "$argon2id$v=19$m=0,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5",
		"threads too many": "$argon2id$v=19$m=64,t=1,p=200$c2FsdHNhbHQ$a2V5a2V5a2V5",
	} {
		if testHasher.Verify("a password", hash) {
			t.Errorf("%s: must not verify", name)
		}
	}
}

func TestPasswordRules(t *testing.T) {
	for password, want := range map[string]string{
		"short":                         "weak_password",
		"123456789":                     "weak_password",
		strings.Repeat("é", 9):          "weak_password",
		strings.Repeat("a", 1025):       "invalid_password",
		"1234567890":                    "",
		strings.Repeat("é", 10):         "",
		"a long passphrase with spaces": "",
	} {
		err := ValidatePassword(password)
		if want == "" && err != nil {
			t.Errorf("%q must be accepted: %v", password[:min(len(password), 12)], err)
		}
		if want != "" {
			if p, ok := err.(*Problem); !ok || p.Code != want || p.Status != 400 {
				t.Errorf("%q: err = %v, want %s", password[:min(len(password), 12)], err, want)
			}
		}
	}
}

func TestUsernameRules(t *testing.T) {
	got, err := ValidateUsername("  Neo.Venise_2 ")
	if err != nil || got != "Neo.Venise_2" {
		t.Errorf("got %q (%v)", got, err)
	}
	for _, ok := range []string{"abc", "A1b", "neo-venise", "a.b.c", "user_01", "9lives", strings.Repeat("a", 30)} {
		if _, err := ValidateUsername(ok); err != nil {
			t.Errorf("%q must be accepted: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "ab", strings.Repeat("a", 31),
		"has space", "a b", "tab\tname", "nom-accentué", "émile", "emoji😀",
		".start", "end.", "-dash", "under_",
		"a@b.fr", "name!", "name/", "<script>", "a;b", "a'b",
	} {
		p, ok := ValidateUsername(bad)
		if ok == nil {
			t.Errorf("%q must be refused, got %q", bad, p)
		}
	}
}

func TestTwoUsernamesThatDifferOnlyByCaseHaveTheSameKey(t *testing.T) {
	if usernameKey("NeoVenise") != usernameKey("neovenise") {
		t.Error("the case must not make another account")
	}
}
