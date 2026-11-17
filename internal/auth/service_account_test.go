package auth

import (
	"context"
	"testing"
)

func TestChangingThePasswordKeepsThisDeviceAndSignsTheOthersOut(t *testing.T) {
	s, _, _ := newService(t)
	phone := register(t, s, "jean", "a long enough password")
	tablet, _ := s.Login(context.Background(), "6.6.6.6", "jean", "a long enough password")
	ctx := context.Background()
	who, _ := s.Authenticate(ctx, phone.AccessToken)

	problem(t, s.ChangePassword(ctx, who, "wrong current", "a brand new password"), "invalid_password")
	problem(t, s.ChangePassword(ctx, who, "a long enough password", "short"), "weak_password")
	if err := s.ChangePassword(ctx, who, "a long enough password", "a brand new password"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Authenticate(ctx, phone.AccessToken); err != nil {
		t.Errorf("this device stays signed in: %v", err)
	}
	if _, err := s.Authenticate(ctx, tablet.AccessToken); err == nil {
		t.Error("the other device must be signed out")
	}
	if _, err := s.Login(ctx, "7.7.7.7", "jean", "a long enough password"); err == nil {
		t.Error("the old password must stop working")
	}
	if _, err := s.Login(ctx, "7.7.7.7", "jean", "a brand new password"); err != nil {
		t.Errorf("the new password works: %v", err)
	}
}

func TestDeletingTheAccountRemovesEverythingAtOnce(t *testing.T) {
	s, _, tx := newService(t)
	session := register(t, s, "jean", "a long enough password")
	ctx := context.Background()
	who, _ := s.Authenticate(ctx, session.AccessToken)

	problem(t, s.DeleteAccount(ctx, who, "wrong password"), "invalid_password")
	if err := s.DeleteAccount(ctx, who, "a long enough password"); err != nil {
		t.Fatal(err)
	}

	var users, sessions, tokens int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions)
	_ = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM access_tokens) + (SELECT count(*) FROM refresh_tokens)`).Scan(&tokens)
	if users != 0 || sessions != 0 || tokens != 0 {
		t.Errorf("left behind: %d users, %d sessions, %d tokens", users, sessions, tokens)
	}
	if _, err := s.Authenticate(ctx, session.AccessToken); err == nil {
		t.Error("the access token must be dead")
	}
	if _, err := s.Login(ctx, "8.8.8.8", "jean", "a long enough password"); err == nil {
		t.Error("the account is gone")
	}
}

func TestAnAdministratorCanResetAPasswordAndEverySessionEnds(t *testing.T) {
	s, _, _ := newService(t)
	session := register(t, s, "jean", "a long enough password")
	ctx := context.Background()

	problem(t, s.ResetPassword(ctx, "nobody", "a brand new password"), "user_not_found")
	problem(t, s.ResetPassword(ctx, "jean", "short"), "weak_password")
	if err := s.ResetPassword(ctx, "JEAN", "a brand new password"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Authenticate(ctx, session.AccessToken); err == nil {
		t.Error("every session must end")
	}
	if _, err := s.Login(ctx, "9.9.9.1", "jean", "a brand new password"); err != nil {
		t.Errorf("the new password works: %v", err)
	}
}
