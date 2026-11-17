package catalog

import "time"

type BlockRow struct {
	ID   string
	Name string
}

type SetRow struct {
	ID            string
	Code          string
	Name          string
	BlockID       string
	BlockName     string
	OfficialCount int
	TotalCount    int
	ReleaseDate   *time.Time
	LogoKey       *string
	SymbolKey     *string
}

type CardRow struct {
	ID       string
	SetID    string
	LocalID  string
	Name     string
	Rarity   string
	ImageKey *string
	ThumbKey *string
}

type CardDetailRow struct {
	CardRow
	Set SetRow

	Illustrator    string
	PriceCents     *int
	PriceCurrency  *string
	PriceUpdatedAt *time.Time

	PriceAvg1Cents, PriceAvg7Cents, PriceAvg30Cents *int
}

type FoilRow struct {
	Look     string
	FrontKey *string
	MaskKey  *string
	EtchKey  *string
}

type PricePointRow struct {
	Day   string
	Cents int
}
