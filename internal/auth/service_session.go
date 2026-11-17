package auth

import (
	"context"
	"errors"
)

// Refresh swaps a refresh token for a new pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	state, sessionID, userID, err := s.repo.ConsumeRefresh(ctx, hashToken(refreshToken), s.now())
	if err != nil {
		return Session{}, err
	}
	switch state {
	case RefreshReplayed:
		if err := s.repo.EndSession(ctx, sessionID); err != nil {
			return Session{}, err
		}
		return Session{}, errInvalidToken
	case RefreshExpired:
		return Session{}, errTokenExpired
	case RefreshUnknown:
		return Session{}, errInvalidToken
	}
	user, _, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		return Session{}, err
	}
	tokens, err := s.issue(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	return Session{Tokens: tokens, User: user}, nil
}

// Logout ends the session of a refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {

	sessionID, err := s.repo.SessionOfRefresh(ctx, hashToken(refreshToken))
	if errors.Is(err, errNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.repo.EndSession(ctx, sessionID)
}

// Authenticate turns an access token into its account.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (Principal, error) {
	info, err := s.repo.AccessSession(ctx, hashToken(accessToken), s.now())
	if errors.Is(err, errNotFound) {
		return Principal{}, errInvalidToken
	}
	if err != nil {
		return Principal{}, err
	}
	if info.Expired {
		return Principal{}, errTokenExpired
	}
	return Principal{UserID: info.UserID, SessionID: info.SessionID}, nil
}
