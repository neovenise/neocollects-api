package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"
)

type Service struct {
	repo   *Repository
	hasher Hasher
	cfg    Config
	now    func() time.Time

	byUsername *limiter
	byAddress  *limiter

	dummyHash string
}

func NewService(repo *Repository, hasher Hasher, cfg Config) (*Service, error) {
	dummy, err := hasher.Hash("not a real password")
	if err != nil {
		return nil, err
	}
	return &Service{
		repo: repo, hasher: hasher, cfg: cfg, now: time.Now,
		byUsername: newLimiter(5, 15*time.Minute), byAddress: newLimiter(20, 15*time.Minute),
		dummyHash: dummy,
	}, nil
}

// Register creates an account and signs it in.
func (s *Service) Register(ctx context.Context, address, username, password, inviteCode string) (Session, error) {
	if s.cfg.InviteCode == "" {
		return Session{}, errRegistrationClosed
	}
	if wait := s.byAddress.blockedFor(address); wait > 0 {
		return Session{}, tooManyAttempts(seconds(wait))
	}
	if subtle.ConstantTimeCompare([]byte(inviteCode), []byte(s.cfg.InviteCode)) != 1 {
		s.byAddress.fail(address)
		return Session{}, errInvalidInvite
	}
	username, err := ValidateUsername(username)
	if err != nil {
		return Session{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Session{}, err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return Session{}, err
	}
	user, err := s.repo.CreateUser(ctx, username, hash)
	if err != nil {
		return Session{}, err
	}
	return s.signIn(ctx, user)
}

// Login signs an account in with its password.
func (s *Service) Login(ctx context.Context, address, username, password string) (Session, error) {
	name, err := ValidateUsername(username)
	if err != nil {

		return Session{}, errInvalidCredentials
	}
	nameKey, addressKey := "user:"+usernameKey(name), address
	if wait := max(s.byUsername.blockedFor(nameKey), s.byAddress.blockedFor(addressKey)); wait > 0 {
		return Session{}, tooManyAttempts(seconds(wait))
	}

	user, hash, err := s.repo.UserByUsername(ctx, name)
	switch {
	case errors.Is(err, errNotFound):
		s.hasher.Verify(password, s.dummyHash)
		s.recordFailure(nameKey, addressKey)
		return Session{}, errInvalidCredentials
	case err != nil:
		return Session{}, err
	}
	if !s.hasher.Verify(password, hash) {
		s.recordFailure(nameKey, addressKey)
		return Session{}, errInvalidCredentials
	}
	s.byUsername.reset(nameKey)
	return s.signIn(ctx, user)
}

func (s *Service) recordFailure(nameKey, addressKey string) {
	s.byUsername.fail(nameKey)
	s.byAddress.fail(addressKey)
}

func (s *Service) signIn(ctx context.Context, user User) (Session, error) {
	_ = s.repo.PurgeExpired(ctx, s.now())
	sessionID, err := s.repo.CreateSession(ctx, user.ID)
	if err != nil {
		return Session{}, err
	}
	tokens, err := s.issue(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	return Session{Tokens: tokens, User: user}, nil
}

func (s *Service) issue(ctx context.Context, sessionID string) (Tokens, error) {
	refresh, refreshHash, err := newToken()
	if err != nil {
		return Tokens{}, err
	}
	access, accessHash, err := newToken()
	if err != nil {
		return Tokens{}, err
	}
	now := s.now()
	if err := s.repo.AddTokens(ctx, sessionID, refreshHash, now.Add(s.cfg.RefreshTTL), accessHash, now.Add(s.cfg.AccessTTL)); err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(s.cfg.AccessTTL.Seconds())}, nil
}

func seconds(d time.Duration) int {
	n := int((d + time.Second - 1) / time.Second)
	if n < 1 {
		return 1
	}
	return n
}
