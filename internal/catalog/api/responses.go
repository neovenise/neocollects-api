package api

import (
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

type blockJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type setJSON struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	BlockID       string  `json:"blockId"`
	BlockName     string  `json:"blockName"`
	OfficialCount int     `json:"officialCount"`
	TotalCount    int     `json:"totalCount"`
	ReleaseDate   *string `json:"releaseDate,omitempty"`
	LogoURL       *string `json:"logoUrl,omitempty"`
	SymbolURL     *string `json:"symbolUrl,omitempty"`
}

type cardSummaryJSON struct {
	ID       string  `json:"id"`
	LocalID  string  `json:"localId"`
	Name     string  `json:"name"`
	Rarity   string  `json:"rarity"`
	ImageURL *string `json:"imageUrl,omitempty"`
	ThumbURL *string `json:"thumbUrl,omitempty"`
	SetID    string  `json:"setId"`
}

type cardJSON struct {
	cardSummaryJSON
	Set setJSON `json:"set"`

	Illustrator    string     `json:"illustrator,omitempty"`
	PriceCents     *int       `json:"priceCents,omitempty"`
	PriceCurrency  *string    `json:"priceCurrency,omitempty"`
	PriceUpdatedAt *time.Time `json:"priceUpdatedAt,omitempty"`

	PriceAverages *priceAveragesJSON `json:"priceAverages,omitempty"`
}

type priceAveragesJSON struct {
	Day   *int `json:"day,omitempty"`
	Week  *int `json:"week,omitempty"`
	Month *int `json:"month,omitempty"`
}

type foilJSON struct {
	Look     string  `json:"look"`
	FrontURL string  `json:"frontUrl"`
	MaskURL  string  `json:"maskUrl"`
	EtchURL  *string `json:"etchUrl,omitempty"`
}

type pricePointJSON struct {
	Date  string `json:"date"`
	Cents int    `json:"cents"`
}

type listJSON[T any] struct {
	Items []T `json:"items"`
}

type pageJSON[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func (h *Handler) setJSON(s catalog.SetRow) setJSON {
	out := setJSON{
		ID: s.ID, Code: s.Code, Name: s.Name, BlockID: s.BlockID, BlockName: s.BlockName,
		OfficialCount: s.OfficialCount, TotalCount: s.TotalCount,
		LogoURL: h.urls.SetLogo(s), SymbolURL: h.urls.SetSymbol(s),
	}
	if s.ReleaseDate != nil {
		date := s.ReleaseDate.Format(time.DateOnly)
		out.ReleaseDate = &date
	}
	return out
}

func (h *Handler) cardSummary(c catalog.CardRow) cardSummaryJSON {
	image, thumb := h.urls.Card(c)
	return cardSummaryJSON{
		ID: c.ID, LocalID: c.LocalID, Name: c.Name, Rarity: c.Rarity, ImageURL: image, ThumbURL: thumb, SetID: c.SetID,
	}
}

func (h *Handler) cardSummaries(cards []catalog.CardRow) []cardSummaryJSON {
	out := make([]cardSummaryJSON, 0, len(cards))
	for _, c := range cards {
		out = append(out, h.cardSummary(c))
	}
	return out
}
