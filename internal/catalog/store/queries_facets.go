package store

import (
	"context"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// Facets counts matching cards per filter value, most common first.
func (q *Queries) Facets(ctx context.Context) (catalog.Facets, error) {
	var f catalog.Facets
	for _, spec := range []struct {
		into *[]catalog.FacetValue
		expr string
		skip string
	}{
		{&f.Rarities, "c.rarity", "c.rarity <> ''"},
		{&f.Categories, "c.category", "c.category <> ''"},
		{&f.Types, "unnest(c.types)", "cardinality(c.types) > 0"},
		{&f.Stages, "c.stage", "c.stage <> ''"},
		{&f.TrainerTypes, "c.trainer_type", "c.trainer_type <> ''"},
		{&f.RegulationMarks, "c.regulation_mark", "c.regulation_mark <> ''"},
	} {
		rows, err := q.db.Query(ctx, `
			SELECT v, count(*) FROM (
				SELECT `+spec.expr+` AS v FROM cards c JOIN sets s ON s.id = c.set_id JOIN blocks b ON b.id = s.block_id
				WHERE NOT b.is_digital AND `+spec.skip+`) t
			GROUP BY v ORDER BY count(*) DESC, v`)
		if err != nil {
			return catalog.Facets{}, wrap("facets", err)
		}
		values := []catalog.FacetValue{}
		for rows.Next() {
			var v catalog.FacetValue
			if err := rows.Scan(&v.Value, &v.Count); err != nil {
				rows.Close()
				return catalog.Facets{}, wrap("scan facet", err)
			}
			values = append(values, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return catalog.Facets{}, wrap("facets", err)
		}
		*spec.into = values
	}
	return f, nil
}
