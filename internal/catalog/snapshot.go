package catalog

type SnapshotSet struct {
	ID            string
	Code          string
	Name          string
	BlockName     string
	OfficialCount int
	TotalCount    int
}

type SnapshotCard struct {
	ID      string
	SetID   string
	LocalID string
	Name    string
	Rarity  string
}
