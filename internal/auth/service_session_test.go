package auth

import (
	"context"
	"testing"
	"time"
)

func TestAnAccessTokenIsRefusedWhenUnknownOrExpired(t *testing.T) {
	s, now, _ := newService(t)
	session := register(t, s, "jean", "a long enough password")
	ctx := context.Background()

	_, err := s.Authenticate(ctx, "not-a-token")
	problem(t, err, "invalid_token")

	*now = now.Add(14 * time.Minute)
	if _, err := s.Authenticate(ctx, session.AccessToken); err != nil {
		t.Errorf("still valid after 14 minutes: %v", err)
	}
	*now = now.Add(2 * time.Minute)
	_, err = s.Authenticate(ctx, session.AccessToken)
	problem(t, err, "token_expired")
}

func TestARefreshTokenIsSpentAndReplacedByANewPair(t *testing.T) {
	s, now, _ := newService(t)
	first := register(t, s, "jean", "a long enough password")
	ctx := context.Background()
	*now = now.Add(20 * time.Minute)

	second, err := s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	if second.RefreshToken == first.RefreshToken || second.AccessToken == first.AccessToken || second.User.ID != first.User.ID {
		t.Errorf("the pair must be new: %+v", second)
	}
	if _, err := s.Authenticate(ctx, second.AccessToken); err != nil {
		t.Errorf("the new access token must work: %v", err)
	}
}

func TestUsingARefreshTokenTwiceEndsTheWholeSession(t *testing.T) {
	s, _, _ := newService(t)
	first := register(t, s, "jean", "a long enough password")
	ctx := context.Background()
	second, err := s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Refresh(ctx, first.RefreshToken)
	problem(t, err, "invalid_token")

	if _, err := s.Authenticate(ctx, second.AccessToken); err == nil {
		t.Error("the new access token must be dead too")
	}
	if _, err := s.Refresh(ctx, second.RefreshToken); err == nil {
		t.Error("the new refresh token must be dead too")
	}
}

func TestAnExpiredOrUnknownRefreshTokenIsRefused(t *testing.T) {
	s, now, _ := newService(t)
	session := register(t, s, "jean", "a long enough password")
	ctx := context.Background()

	_, err := s.Refresh(ctx, "not-a-token")
	problem(t, err, "invalid_token")

	*now = now.Add(91 * 24 * time.Hour)
	_, err = s.Refresh(ctx, session.RefreshToken)
	problem(t, err, "token_expired")
}

func TestSigningOutEndsOnlyThatDevice(t *testing.T) {
	s, _, _ := newService(t)
	phone := register(t, s, "jean", "a long enough password")
	tablet, err := s.Login(context.Background(), "6.6.6.6", "jean", "a long enough password")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := s.Logout(ctx, phone.RefreshToken); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Authenticate(ctx, phone.AccessToken); err == nil {
		t.Error("the phone is signed out")
	}
	if _, err := s.Authenticate(ctx, tablet.AccessToken); err != nil {
		t.Errorf("the tablet stays signed in: %v", err)
	}

	if err := s.Logout(ctx, phone.RefreshToken); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if err := s.Logout(ctx, "not-a-token"); err != nil {
		t.Errorf("unknown token: %v", err)
	}
}

func TestExpiredTokensArePurgedAtSignIn(t *testing.T) {
	s, now, tx := newService(t)
	register(t, s, "jean", "a long enough password")
	*now = now.Add(100 * 24 * time.Hour)

	if _, err := s.Login(context.Background(), "9.9.9.2", "jean", "a long enough password"); err != nil {
		t.Fatal(err)
	}

	var access, refresh int
	_ = tx.QueryRow(context.Background(), `SELECT count(*) FROM access_tokens`).Scan(&access)
	_ = tx.QueryRow(context.Background(), `SELECT count(*) FROM refresh_tokens`).Scan(&refresh)
	if access != 1 || refresh != 1 {
		t.Errorf("%d access and %d refresh tokens kept", access, refresh)
	}
}
