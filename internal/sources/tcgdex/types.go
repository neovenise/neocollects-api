package tcgdex

import (
	"math"
	"time"
)

type Serie struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SetRef struct {
	ID string `json:"id"`
}

type CardCount struct {
	Official int `json:"official"`
	Total    int `json:"total"`
}

type Set struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Logo         string    `json:"logo"`
	Symbol       string    `json:"symbol"`
	ReleaseDate  string    `json:"releaseDate"`
	CardCount    CardCount `json:"cardCount"`
	Serie        Serie     `json:"serie"`
	Abbreviation struct {
		Official string `json:"official"`
	} `json:"abbreviation"`
	Cards []CardBrief `json:"cards"`
}

type CardBrief struct {
	ID      string `json:"id"`
	LocalID string `json:"localId"`
	Name    string `json:"name"`

	Image string `json:"image"`
}

type Card struct {
	CardBrief
	Rarity  string   `json:"rarity"`
	Pricing *Pricing `json:"pricing"`

	Category       string   `json:"category"`
	Types          []string `json:"types"`
	HP             *int     `json:"hp"`
	Stage          string   `json:"stage"`
	TrainerType    string   `json:"trainerType"`
	RegulationMark string   `json:"regulationMark"`

	Illustrator string `json:"illustrator"`
}

type Pricing struct {
	Cardmarket *Cardmarket `json:"cardmarket"`
}

type Cardmarket struct {
	Trend *float64 `json:"trend"`

	Avg1  *float64 `json:"avg1"`
	Avg7  *float64 `json:"avg7"`
	Avg30 *float64 `json:"avg30"`

	IDProduct *int   `json:"idProduct"`
	Updated   string `json:"updated"`
}

// TrendCents returns the trend price in euro cents.
func (c Card) TrendCents() (int, bool) {
	if c.Pricing == nil || c.Pricing.Cardmarket == nil || c.Pricing.Cardmarket.Trend == nil {
		return 0, false
	}
	trend := *c.Pricing.Cardmarket.Trend
	if trend < 0 || math.IsNaN(trend) || math.IsInf(trend, 0) {
		return 0, false
	}
	return int(math.Round(trend * 100)), true
}

// AverageCents returns the 1, 7 and 30-day averages in cents.
func (c Card) AverageCents() (day, week, month *int) {
	if c.Pricing == nil || c.Pricing.Cardmarket == nil {
		return nil, nil, nil
	}
	cm := c.Pricing.Cardmarket
	return euroCents(cm.Avg1), euroCents(cm.Avg7), euroCents(cm.Avg30)
}

// ProductID returns the product the price is for, or nil.
func (c Card) ProductID() *int {
	if c.Pricing == nil || c.Pricing.Cardmarket == nil {
		return nil
	}
	return c.Pricing.Cardmarket.IDProduct
}

func euroCents(v *float64) *int {
	if v == nil || *v < 0 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	cents := int(math.Round(*v * 100))
	return &cents
}

// PriceUpdatedAt returns when the price was last refreshed.
func (c Card) PriceUpdatedAt() time.Time {
	if c.Pricing == nil || c.Pricing.Cardmarket == nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, c.Pricing.Cardmarket.Updated)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
