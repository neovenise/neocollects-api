package catalog

import "time"

type Block struct {
	ID       string
	Name     string
	Position int

	Digital bool
}

// IsDigitalBlock reports whether a block is the mobile game.
func IsDigitalBlock(id string) bool {
	return id == "tcgp"
}

type Set struct {
	ID              string
	Code            string
	Name            string
	BlockID         string
	OfficialCount   int
	TotalCount      int
	ReleaseDate     *time.Time
	LogoSourceURL   string
	SymbolSourceURL string
}

type Card struct {
	ID             string
	SetID          string
	LocalID        string
	Name           string
	Rarity         string
	SourceImageURL string

	Category       string
	Types          []string
	HP             *int
	Stage          string
	TrainerType    string
	RegulationMark string
	Illustrator    string

	SharpImageURL string
}

type Price struct {
	CardID    string
	Cents     int
	Currency  string
	UpdatedAt time.Time

	Avg1Cents, Avg7Cents, Avg30Cents *int

	ProductID *int
}

type Foil struct {
	CardID     string
	Look       string
	FrontURL   string
	MaskURL    string
	EtchURL    string
	SourceHash string
}
