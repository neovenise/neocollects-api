package catalog

type SetFilter struct {
	BlockID       string
	OfficialCount *int
}

type CardFilter struct {
	IDs     []string
	Number  *int
	SetSize *int
	SetIDs  []string
	Name    string

	Query string

	ExactName string
	Rarities  []string

	Illustrator string

	Categories      []string
	Types           []string
	Stages          []string
	TrainerTypes    []string
	RegulationMarks []string
	Limit           int
	Offset          int
}
