package app

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/neovenise/neocollects-api/internal/auth"
	catalogapi "github.com/neovenise/neocollects-api/internal/catalog/api"
	"github.com/neovenise/neocollects-api/internal/catalog/store"
	"github.com/neovenise/neocollects-api/internal/collection"
	"github.com/neovenise/neocollects-api/internal/lists"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

func NewServer(cfg *config.Config, pool *pgxpool.Pool, logger *slog.Logger) (http.Handler, error) {
	accounts, err := auth.NewService(auth.NewRepository(pool), auth.DefaultHasher, auth.DefaultConfig(cfg.InviteCode))
	if err != nil {
		return nil, err
	}
	engine := httpserver.New(logger)
	Routes(engine, Handlers{
		Health:     httpserver.Health(pool),
		Catalog:    catalogapi.NewHandler(store.NewQueries(pool), catalogapi.NewImageURLs(cfg.ImagesBaseURL), logger),
		Auth:       auth.NewHandler(accounts, logger),
		Collection: collection.NewHandler(collection.NewRepository(pool), logger),
		Lists:      lists.NewHandler(lists.NewRepository(pool), logger),
	})
	return engine, nil
}
