package auth

import "fmt"

type Problem struct {
	Status int
	Code   string
	Detail string

	RetryAfter int
}

func (p *Problem) Error() string { return fmt.Sprintf("%d %s: %s", p.Status, p.Code, p.Detail) }

var (
	errInvalidCredentials = &Problem{Status: 401, Code: "invalid_credentials", Detail: "wrong username or password"}
	errInvalidToken       = &Problem{Status: 401, Code: "invalid_token", Detail: "the token is not valid"}
	errTokenExpired       = &Problem{Status: 401, Code: "token_expired", Detail: "the token has expired"}
	errUsernameTaken      = &Problem{Status: 409, Code: "username_taken", Detail: "this username is already taken"}
	errInvalidInvite      = &Problem{Status: 403, Code: "invalid_invite_code", Detail: "the invitation code is not valid"}
	errRegistrationClosed = &Problem{Status: 403, Code: "registration_closed", Detail: "sign-up is closed on this server"}
)

func tooManyAttempts(retryAfterSeconds int) *Problem {
	return &Problem{Status: 429, Code: "too_many_attempts", Detail: fmt.Sprintf("too many attempts: try again in %d seconds", retryAfterSeconds), RetryAfter: retryAfterSeconds}
}
