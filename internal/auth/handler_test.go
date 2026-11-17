package auth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

type okDB struct{}

func (okDB) Ping(context.Context) error { return nil }

func newEngine(t *testing.T) (http.Handler, *Service) {
	t.Helper()
	s, _, _ := newService(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return mount(httpserver.New(logger), NewHandler(s, logger)), s
}

func mount(engine *gin.Engine, h *Handler) *gin.Engine {
	engine.POST("/v1/auth/register", h.Register)
	engine.POST("/v1/auth/login", h.Login)
	engine.POST("/v1/auth/refresh", h.Refresh)
	engine.POST("/v1/auth/logout", h.Logout)
	me := engine.Group("/v1/me", h.Required())
	me.GET("", h.Me)
	me.PUT("/password", h.ChangePassword)
	me.DELETE("", h.DeleteAccount)
	return engine
}

func call(t *testing.T, h http.Handler, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:5555"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON (%d): %q", rec.Code, rec.Body.String())
	}
	return out
}

func registerBody(username string) string {
	return `{"username": "` + username + `", "password": "a long enough password", "inviteCode": "` + invite + `"}`
}

func TestRegisteringAnswers201WithTheTokensAndTheUserAndNeverCachesThem(t *testing.T) {
	h, _ := newEngine(t)

	rec := call(t, h, http.MethodPost, "/v1/auth/register", registerBody("Jean"))

	if rec.Code != http.StatusCreated || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("status %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	body := decode(t, rec)
	user := body["user"].(map[string]any)
	if body["accessToken"] == "" || body["refreshToken"] == "" || body["expiresIn"] != float64(900) ||
		user["username"] != "Jean" || user["id"] == "" || user["createdAt"] == "" {
		t.Errorf("body = %v", body)
	}
	if _, has := body["password"]; has {
		t.Error("a password must never be echoed")
	}
}

func TestErrorsAreProblemJSONWithAStableCode(t *testing.T) {
	h, _ := newEngine(t)
	call(t, h, http.MethodPost, "/v1/auth/register", registerBody("jean"))

	for name, tt := range map[string]struct {
		target, body string
		status       int
		code         string
	}{
		"wrong invite":     {"/v1/auth/register", `{"username": "alice", "password": "a long enough password", "inviteCode": "no"}`, 403, "invalid_invite_code"},
		"taken username":   {"/v1/auth/register", registerBody("jean"), 409, "username_taken"},
		"weak password":    {"/v1/auth/register", `{"username": "bob", "password": "short", "inviteCode": "` + invite + `"}`, 400, "weak_password"},
		"wrong password":   {"/v1/auth/login", `{"username": "jean", "password": "wrong"}`, 401, "invalid_credentials"},
		"unknown username": {"/v1/auth/login", `{"username": "nobody", "password": "whatever"}`, 401, "invalid_credentials"},
		"not JSON":         {"/v1/auth/login", `<html>`, 400, "invalid_body"},
		"empty body":       {"/v1/auth/refresh", ``, 400, "invalid_body"},
		"unknown refresh":  {"/v1/auth/refresh", `{"refreshToken": "nope"}`, 401, "invalid_token"},
	} {
		rec := call(t, h, http.MethodPost, tt.target, tt.body)
		body := decode(t, rec)
		if rec.Code != tt.status || body["code"] != tt.code || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
			t.Errorf("%s: %d %v (%s)", name, rec.Code, body["code"], rec.Header().Get("Content-Type"))
		}
	}
}

func TestABodyThatIsTooBigIsRefused(t *testing.T) {
	h, _ := newEngine(t)

	rec := call(t, h, http.MethodPost, "/v1/auth/login", `{"username": "`+strings.Repeat("a", 20_000)+`"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestTooManyWrongPasswordsAnswer429WithRetryAfter(t *testing.T) {
	h, _ := newEngine(t)
	call(t, h, http.MethodPost, "/v1/auth/register", registerBody("jean"))
	for i := 0; i < 5; i++ {
		call(t, h, http.MethodPost, "/v1/auth/login", `{"username": "jean", "password": "wrong"}`)
	}

	rec := call(t, h, http.MethodPost, "/v1/auth/login", `{"username": "jean", "password": "a long enough password"}`)

	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || decode(t, rec)["code"] != "too_many_attempts" {
		t.Errorf("%d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestTheAccountRoutesNeedABearerToken(t *testing.T) {
	h, _ := newEngine(t)

	for name, headers := range map[string][]string{
		"no header":     nil,
		"not bearer":    {"Authorization", "Basic abc"},
		"empty bearer":  {"Authorization", "Bearer "},
		"unknown token": {"Authorization", "Bearer nope"},
	} {
		rec := call(t, h, http.MethodGet, "/v1/me", "", headers...)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "Bearer" || decode(t, rec)["code"] != "invalid_token" {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}
}

func TestTheWholeLifeOfAnAccountOverHTTP(t *testing.T) {
	h, _ := newEngine(t)
	signUp := decode(t, call(t, h, http.MethodPost, "/v1/auth/register", registerBody("jean")))
	access := "Bearer " + signUp["accessToken"].(string)

	me := decode(t, call(t, h, http.MethodGet, "/v1/me", "", "Authorization", access))
	if me["username"] != "jean" || me["id"] != signUp["user"].(map[string]any)["id"] {
		t.Errorf("me = %v", me)
	}

	refreshed := call(t, h, http.MethodPost, "/v1/auth/refresh", `{"refreshToken": "`+signUp["refreshToken"].(string)+`"}`)
	if refreshed.Code != http.StatusOK || decode(t, refreshed)["accessToken"] == signUp["accessToken"] {
		t.Fatalf("refresh = %d", refreshed.Code)
	}
	if again := call(t, h, http.MethodPost, "/v1/auth/refresh", `{"refreshToken": "`+signUp["refreshToken"].(string)+`"}`); again.Code != http.StatusUnauthorized {
		t.Errorf("a spent refresh token: %d", again.Code)
	}

	newAccess := "Bearer " + decode(t, refreshed)["accessToken"].(string)
	if rec := call(t, h, http.MethodGet, "/v1/me", "", "Authorization", newAccess); rec.Code != http.StatusUnauthorized {
		t.Errorf("after a replay: %d", rec.Code)
	}

	login := decode(t, call(t, h, http.MethodPost, "/v1/auth/login", `{"username": "jean", "password": "a long enough password"}`))
	auth := "Bearer " + login["accessToken"].(string)
	if rec := call(t, h, http.MethodPut, "/v1/me/password", `{"currentPassword": "a long enough password", "newPassword": "a brand new password"}`, "Authorization", auth); rec.Code != http.StatusNoContent {
		t.Errorf("change password = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodDelete, "/v1/me", `{"password": "wrong"}`, "Authorization", auth); rec.Code != http.StatusForbidden {
		t.Errorf("delete with a wrong password = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodDelete, "/v1/me", `{"password": "a brand new password"}`, "Authorization", auth); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodGet, "/v1/me", "", "Authorization", auth); rec.Code != http.StatusUnauthorized {
		t.Errorf("after the deletion: %d", rec.Code)
	}
	if rec := call(t, h, http.MethodPost, "/v1/auth/login", `{"username": "jean", "password": "a brand new password"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("signing in to a deleted account: %d", rec.Code)
	}
}

func TestLoggingOutAnswers204AndKillsTheSession(t *testing.T) {
	h, _ := newEngine(t)
	signUp := decode(t, call(t, h, http.MethodPost, "/v1/auth/register", registerBody("jean")))

	rec := call(t, h, http.MethodPost, "/v1/auth/logout", `{"refreshToken": "`+signUp["refreshToken"].(string)+`"}`)

	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("logout = %d %q", rec.Code, rec.Body.String())
	}
	if rec := call(t, h, http.MethodGet, "/v1/me", "", "Authorization", "Bearer "+signUp["accessToken"].(string)); rec.Code != http.StatusUnauthorized {
		t.Errorf("after logout: %d", rec.Code)
	}
}

func TestAnExpiredAccessTokenSaysSoSoTheAppRefreshes(t *testing.T) {
	h, s := newEngine(t)
	signUp := decode(t, call(t, h, http.MethodPost, "/v1/auth/register", registerBody("jean")))
	later := s.now().Add(20 * 60 * 1e9)
	s.now = func() time.Time { return later }

	rec := call(t, h, http.MethodGet, "/v1/me", "", "Authorization", "Bearer "+signUp["accessToken"].(string))

	if rec.Code != http.StatusUnauthorized || decode(t, rec)["code"] != "token_expired" {
		t.Errorf("%d %v", rec.Code, rec.Body.String())
	}
}
