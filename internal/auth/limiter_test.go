package auth

import (
	"testing"
	"time"
)

func fakeClock() (*time.Time, func() time.Time) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	return &now, func() time.Time { return now }
}

func TestAKeyIsBlockedAfterTooManyFailuresAndFreedWhenTheOldestLeavesTheWindow(t *testing.T) {
	now, clock := fakeClock()
	l := newLimiter(3, 10*time.Minute)
	l.now = clock

	l.fail("a")
	*now = now.Add(2 * time.Minute)
	l.fail("a")
	l.fail("a")

	if wait := l.blockedFor("a"); wait != 8*time.Minute {
		t.Errorf("blocked for %v, want 8m", wait)
	}
	if l.blockedFor("b") != 0 {
		t.Error("another key is not affected")
	}
	*now = now.Add(8*time.Minute + time.Second)
	if wait := l.blockedFor("a"); wait != 0 {
		t.Errorf("after the window: blocked for %v", wait)
	}
}

func TestASuccessForgetsTheFailures(t *testing.T) {
	_, clock := fakeClock()
	l := newLimiter(2, time.Hour)
	l.now = clock
	l.fail("a")
	l.fail("a")

	l.reset("a")

	if l.blockedFor("a") != 0 {
		t.Error("a successful sign-in must clear the count")
	}
}

func TestAFloodOfDifferentKeysDoesNotGrowTheMapForever(t *testing.T) {
	now, clock := fakeClock()
	l := newLimiter(3, time.Minute)
	l.now = clock
	for i := 0; i < 10_000; i++ {
		l.fail(string(rune('a'+i%26)) + string(rune(i)))
	}
	*now = now.Add(2 * time.Minute)
	l.fail("one more")

	if len(l.failures) > 1 {
		t.Errorf("%d keys kept", len(l.failures))
	}
}

func TestTokensAreRandomAndOnlyTheirHashIsKept(t *testing.T) {
	a, hashA, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := newToken()

	if a == b || len(a) < 40 {
		t.Errorf("tokens %q %q", a, b)
	}
	if string(hashA) != string(hashToken(a)) || string(hashA) == a {
		t.Error("the stored value is the hash of the token")
	}
}
