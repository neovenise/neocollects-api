package catalog

type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type Facets struct {
	Rarities        []FacetValue `json:"rarities"`
	Categories      []FacetValue `json:"categories"`
	Types           []FacetValue `json:"types"`
	Stages          []FacetValue `json:"stages"`
	TrainerTypes    []FacetValue `json:"trainerTypes"`
	RegulationMarks []FacetValue `json:"regulationMarks"`
}
