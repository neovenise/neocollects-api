package database

import (
	"net/url"
	"os"
	"strings"

	"github.com/neovenise/neocollects-api/internal/platform/config"
)

// TestURL returns the database the repository tests run on: TEST_DATABASE_URL, else the main database (DATABASE_URL or DB_*)
// with "_test" added to its name. Tests need an empty database of their own, never the real one.
func TestURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	raw := config.DatabaseURL()
	u, err := url.Parse(raw)
	if raw == "" || err != nil {
		return ""
	}
	if !strings.HasSuffix(u.Path, "_test") {
		u.Path += "_test"
	}
	return u.String()
}
