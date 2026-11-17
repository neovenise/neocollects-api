package app

import (
	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/auth"
	catalogapi "github.com/neovenise/neocollects-api/internal/catalog/api"
	"github.com/neovenise/neocollects-api/internal/collection"
	"github.com/neovenise/neocollects-api/internal/lists"
)

type Handlers struct {
	Health     gin.HandlerFunc
	Catalog    *catalogapi.Handler
	Auth       *auth.Handler
	Collection *collection.Handler
	Lists      *lists.Handler
}

// Routes declares every route of the API.
func Routes(r gin.IRouter, h Handlers) {
	r.GET("/healthz", h.Health)
	v1 := r.Group("/v1")

	catalog := v1.Group("", catalogapi.OnlyFrench)
	catalog.GET("/blocks", h.Catalog.Blocks)
	catalog.GET("/sets", h.Catalog.Sets)
	catalog.GET("/sets/:setId", h.Catalog.Set)
	catalog.GET("/sets/:setId/cards", h.Catalog.SetCards)
	catalog.GET("/cards", h.Catalog.SearchCards)
	catalog.GET("/cards/facets", h.Catalog.Facets)
	catalog.GET("/cards/:cardId", h.Catalog.Card)
	catalog.GET("/cards/:cardId/foil", h.Catalog.Foil)
	catalog.GET("/cards/:cardId/price-history", h.Catalog.PriceHistory)
	catalog.GET("/catalog/snapshot", h.Catalog.Snapshot)

	v1.POST("/auth/register", h.Auth.Register)
	v1.POST("/auth/login", h.Auth.Login)
	v1.POST("/auth/refresh", h.Auth.Refresh)
	v1.POST("/auth/logout", h.Auth.Logout)

	me := v1.Group("/me", h.Auth.Required())
	me.GET("", h.Auth.Me)
	me.PUT("/password", h.Auth.ChangePassword)
	me.DELETE("", h.Auth.DeleteAccount)

	me.GET("/collection/entries", h.Collection.List)
	me.POST("/collection/entries", h.Collection.Add)
	me.PATCH("/collection/entries/:entryID", h.Collection.Update)
	me.DELETE("/collection/entries/:entryID", h.Collection.Remove)
	me.GET("/collection/summary", h.Collection.Summary)

	me.GET("/lists", h.Lists.List)
	me.POST("/lists", h.Lists.Create)
	me.PATCH("/lists/:listID", h.Lists.Rename)
	me.DELETE("/lists/:listID", h.Lists.Remove)
	me.PUT("/lists/:listID/cards/:cardID", h.Lists.PutCard)
	me.DELETE("/lists/:listID/cards/:cardID", h.Lists.RemoveCard)
}
