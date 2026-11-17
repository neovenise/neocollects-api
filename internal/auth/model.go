package auth

import "time"

type Config struct {
	InviteCode string

	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// DefaultConfig: 15-minute access tokens, 90-day refresh tokens.
func DefaultConfig(inviteCode string) Config {
	return Config{InviteCode: inviteCode, AccessTTL: 15 * time.Minute, RefreshTTL: 90 * 24 * time.Hour}
}

type Tokens struct {
	AccessToken  string
	RefreshToken string

	ExpiresIn int
}

type Session struct {
	Tokens
	User User
}

type Principal struct {
	UserID    string
	SessionID string
}
