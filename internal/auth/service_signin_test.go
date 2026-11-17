package auth

import (
	"context"
	"testing"
	"time"
)

func TestSigningUpCreatesTheAccountAndSignsItIn(t *testing.T) {
	s, _, tx := newService(t)

	session := register(t, s, "  Jean  ", "a long enough password")

	if session.User.Username != "Jean" || session.User.ID == "" || session.ExpiresIn != 900 {
		t.Errorf("session = %+v", session)
	}
	who, err := s.Authenticate(context.Background(), session.AccessToken)
	if err != nil || who.UserID != session.User.ID {
		t.Errorf("the access token must work at once: %+v (%v)", who, err)
	}

	var hash string
	_ = tx.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE id = $1`, session.User.ID).Scan(&hash)
	if hash == "a long enough password" || len(hash) < 40 {
		t.Errorf("password_hash = %q", hash)
	}
	var plainTokens int
	_ = tx.QueryRow(context.Background(), `SELECT count(*) FROM access_tokens WHERE token_hash = $1 OR token_hash = $2`,
		[]byte(session.AccessToken), []byte(session.RefreshToken)).Scan(&plainTokens)
	if plainTokens != 0 {
		t.Error("a token must never be stored as it was given")
	}
}

func TestSigningUpNeedsTheInvitationCode(t *testing.T) {
	s, _, _ := newService(t)

	_, err := s.Register(context.Background(), "1.1.1.1", "alice", "a long enough password", "wrong")
	problem(t, err, "invalid_invite_code")

	closed, _ := NewService(s.repo, testHasher, DefaultConfig(""))
	_, err = closed.Register(context.Background(), "1.1.1.1", "alice", "a long enough password", "")
	problem(t, err, "registration_closed")
}

func TestGuessingTheInvitationCodeIsSlowedDown(t *testing.T) {
	s, _, _ := newService(t)
	for i := 0; i < 20; i++ {
		_, err := s.Register(context.Background(), "9.9.9.9", "alice", "a long enough password", "guess")
		problem(t, err, "invalid_invite_code")
	}

	_, err := s.Register(context.Background(), "9.9.9.9", "alice", "a long enough password", invite)

	problem(t, err, "too_many_attempts")
	if p := err.(*Problem); p.RetryAfter < 1 {
		t.Errorf("RetryAfter = %d", p.RetryAfter)
	}
}

func TestSignUpRefusesABadEmailAWeakPasswordAndATakenEmail(t *testing.T) {
	s, _, _ := newService(t)
	register(t, s, "jean", "a long enough password")

	_, err := s.Register(context.Background(), "1.1.1.1", "not a name!", "a long enough password", invite)
	problem(t, err, "invalid_username")
	_, err = s.Register(context.Background(), "1.1.1.1", "newbie", "short", invite)
	problem(t, err, "weak_password")
	_, err = s.Register(context.Background(), "1.1.1.1", "JEAN", "another long password", invite)
	problem(t, err, "username_taken")
}

func TestSigningInWithTheRightPasswordAndRefusingTheWrongOnesTheSameWay(t *testing.T) {
	s, _, _ := newService(t)
	register(t, s, "jean", "a long enough password")
	ctx := context.Background()

	session, err := s.Login(ctx, "2.2.2.2", "JEAN", "a long enough password")
	if err != nil || session.User.Username != "jean" {
		t.Fatalf("login = %+v (%v)", session, err)
	}

	_, wrongPassword := s.Login(ctx, "2.2.2.2", "jean", "wrong password")
	_, unknownEmail := s.Login(ctx, "2.2.2.2", "nobody", "a long enough password")
	_, malformed := s.Login(ctx, "2.2.2.2", "not a name!", "a long enough password")
	for name, err := range map[string]error{"wrong password": wrongPassword, "unknown username": unknownEmail, "malformed": malformed} {
		p, ok := err.(*Problem)
		if !ok || p.Code != "invalid_credentials" || p.Status != 401 || p.Detail != "wrong username or password" {
			t.Errorf("%s: %v (it must not tell whether the account exists)", name, err)
		}
	}
}

func TestTooManyWrongPasswordsBlockTheAccountEvenForTheRightOne(t *testing.T) {
	s, now, _ := newService(t)
	register(t, s, "jean", "a long enough password")
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := s.Login(ctx, "3.3."+string(rune('0'+i))+".3", "jean", "wrong password")
		problem(t, err, "invalid_credentials")
	}

	_, err := s.Login(ctx, "4.4.4.4", "jean", "a long enough password")
	problem(t, err, "too_many_attempts")

	*now = now.Add(16 * time.Minute)
	if _, err := s.Login(ctx, "4.4.4.4", "jean", "a long enough password"); err != nil {
		t.Errorf("after the window the right password works: %v", err)
	}
}

func TestASuccessfulSignInClearsTheCountOfWrongPasswords(t *testing.T) {
	s, _, _ := newService(t)
	register(t, s, "jean", "a long enough password")
	ctx := context.Background()
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			_, _ = s.Login(ctx, "5.5.5.5", "jean", "wrong password")
		}
		if _, err := s.Login(ctx, "5.5.5.5", "jean", "a long enough password"); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
	}
}
