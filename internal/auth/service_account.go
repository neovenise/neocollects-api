package auth

import (
	"context"
	"errors"
)

// Me returns the account of a principal.
func (s *Service) Me(ctx context.Context, p Principal) (User, error) {
	user, _, err := s.repo.UserByID(ctx, p.UserID)
	return user, err
}

// ChangePassword sets a new password and ends other sessions.
func (s *Service) ChangePassword(ctx context.Context, p Principal, currentPassword, newPassword string) error {
	user, hash, err := s.repo.UserByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(currentPassword, hash) {
		return &Problem{Status: 403, Code: "invalid_password", Detail: "the current password is wrong"}
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.SetPassword(ctx, user.ID, newHash); err != nil {
		return err
	}
	return s.repo.EndOtherSessions(ctx, user.ID, p.SessionID)
}

// DeleteAccount removes the account and everything it owns.
func (s *Service) DeleteAccount(ctx context.Context, p Principal, password string) error {
	_, hash, err := s.repo.UserByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(password, hash) {
		return &Problem{Status: 403, Code: "invalid_password", Detail: "the password is wrong"}
	}
	return s.repo.DeleteUser(ctx, p.UserID)
}

// ResetPassword sets a password without the old one.
func (s *Service) ResetPassword(ctx context.Context, username, newPassword string) error {
	name, err := ValidateUsername(username)
	if err != nil {
		return err
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	user, _, err := s.repo.UserByUsername(ctx, name)
	if errors.Is(err, errNotFound) {
		return &Problem{Status: 404, Code: "user_not_found", Detail: "no account has this username"}
	}
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.SetPassword(ctx, user.ID, hash); err != nil {
		return err
	}
	return s.repo.EndOtherSessions(ctx, user.ID, "")
}
