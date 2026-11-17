package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

const maxBody = 8 << 10

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type userJSON struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
}

type sessionJSON struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	ExpiresIn    int      `json:"expiresIn"`
	User         userJSON `json:"user"`
}

func toUserJSON(u User) userJSON {
	return userJSON{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt.UTC()}
}

func toSessionJSON(s Session) sessionJSON {
	return sessionJSON{AccessToken: s.AccessToken, RefreshToken: s.RefreshToken, ExpiresIn: s.ExpiresIn, User: toUserJSON(s.User)}
}

// Register creates an account and returns its tokens.
func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		InviteCode string `json:"inviteCode"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	session, err := h.service.Register(c.Request.Context(), c.ClientIP(), req.Username, req.Password, req.InviteCode)
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusCreated, toSessionJSON(session))
}

// Login checks the password and returns new tokens.
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	session, err := h.service.Login(c.Request.Context(), c.ClientIP(), req.Username, req.Password)
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, toSessionJSON(session))
}

// Refresh swaps a refresh token for a new pair.
func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	session, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, toSessionJSON(session))
}

// Logout ends the session of a refresh token.
func (h *Handler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Me returns the signed-in account.
func (h *Handler) Me(c *gin.Context) {
	user, err := h.service.Me(c.Request.Context(), principal(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, toUserJSON(user))
}

// ChangePassword sets a new password and signs other devices out.
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), principal(c), req.CurrentPassword, req.NewPassword); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteAccount removes the account after checking its password.
func (h *Handler) DeleteAccount(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	if err := h.service.DeleteAccount(c.Request.Context(), principal(c), req.Password); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Required rejects requests without a valid access token.
func (h *Handler) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			c.Header("WWW-Authenticate", "Bearer")
			httpserver.WriteProblem(c, http.StatusUnauthorized, "invalid_token", "a bearer token is required")
			return
		}
		p, err := h.service.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.Header("WWW-Authenticate", "Bearer")
			h.fail(c, err)
			return
		}
		c.Set(principalKey, p)
		c.Next()
	}
}

const principalKey = "auth.principal"

// PrincipalOf returns the account behind a request.
func PrincipalOf(c *gin.Context) Principal { return principal(c) }

func principal(c *gin.Context) Principal {
	p, _ := c.Get(principalKey)
	pr, _ := p.(Principal)
	return pr
}

func (h *Handler) fail(c *gin.Context, err error) {
	var p *Problem
	if errors.As(err, &p) {
		if p.RetryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(p.RetryAfter))
		}
		httpserver.WriteProblem(c, p.Status, p.Code, p.Detail)
		return
	}
	h.logger.Error("auth: unexpected error", "error", err, "route", c.FullPath())
	httpserver.WriteProblem(c, http.StatusInternalServerError, "internal_error", "")
}
