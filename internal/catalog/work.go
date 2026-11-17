package catalog

type FoilMirrorRow struct {
	CardID    string
	FrontURL  string
	MaskURL   string
	EtchURL   string
	Hash      string
	CopiedKey bool
}

type FoilCandidate struct {
	CardID  string
	SetID   string
	LocalID string
	Rarity  string
}

type LogoCandidate struct {
	ID   string
	Code string

	LogoSource   string
	SymbolSource string
	NeedsLogo    bool
	NeedsSymbol  bool
}

type MirrorCard struct {
	ID string

	SourceURL string
	SharpURL  string

	Hash string
}
