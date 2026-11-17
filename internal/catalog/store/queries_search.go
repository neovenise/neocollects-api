package store

import (
	"context"
	"strconv"
	"strings"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// SearchCards finds the cards matching every filter, newest set first.
func (q *Queries) SearchCards(ctx context.Context, f catalog.CardFilter) (cards []catalog.CardRow, hasMore bool, err error) {

	conds := []string{"NOT b.is_digital"}
	var args []any
	add := func(cond string, arg any) {
		args = append(args, arg)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}

	if len(f.IDs) > 0 {
		add("c.id = ANY(?)", f.IDs)
	}
	if f.Number != nil {
		add("c.local_number = ?", *f.Number)
	}
	if f.SetSize != nil {
		add("s.official_count = ?", *f.SetSize)
	}
	if len(f.SetIDs) > 0 {
		add("c.set_id = ANY(?)", f.SetIDs)
	}
	if f.Name != "" {
		name := catalog.SearchName(f.Name)
		if name == "" {

			return []catalog.CardRow{}, false, nil
		}
		add(`c.name_search LIKE ? ESCAPE '\'`, escapeLike(name)+"%")
	}
	if f.ExactName != "" {
		name := catalog.SearchName(f.ExactName)
		if name == "" {
			return []catalog.CardRow{}, false, nil
		}
		add("c.name_search = ?", name)
	}

	relevance := ""
	catalogOrder := "s.release_date DESC NULLS LAST, s.id, c.local_number NULLS LAST, c.local_id"
	if f.Query != "" {
		part := catalog.SearchName(f.Query)
		if part == "" {
			return []catalog.CardRow{}, false, nil
		}
		add(`c.name_search LIKE ? ESCAPE '\'`, "%"+escapeLike(part)+"%")
		args = append(args, escapeLike(part)+"%")
		relevance = `(c.name_search LIKE $` + strconv.Itoa(len(args)) + ` ESCAPE '\') DESC, `
	}
	if len(f.Rarities) > 0 {
		add("c.rarity = ANY(?)", f.Rarities)
	}
	if f.Illustrator != "" {
		add("lower(c.illustrator) = lower(?)", f.Illustrator)
	}
	if len(f.Categories) > 0 {
		add("c.category = ANY(?)", f.Categories)
	}
	if len(f.Types) > 0 {
		add("c.types && ?::text[]", f.Types)
	}
	if len(f.Stages) > 0 {
		add("c.stage = ANY(?)", f.Stages)
	}
	if len(f.TrainerTypes) > 0 {
		add("c.trainer_type = ANY(?)", f.TrainerTypes)
	}
	if len(f.RegulationMarks) > 0 {
		add("c.regulation_mark = ANY(?)", f.RegulationMarks)
	}

	order := relevance + catalogOrder
	if len(f.IDs) == 0 && f.Number == nil && f.Name == "" {
		order = "NOT " + hasPicture + ", " + order
	}

	args = append(args, f.Limit+1, f.Offset)
	query := `SELECT ` + cardColumns + `
		FROM cards c
		JOIN sets s ON s.id = c.set_id
		JOIN blocks b ON b.id = s.block_id
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY ` + order + `
		LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))

	rows, err := q.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, wrap("search cards", err)
	}
	defer rows.Close()

	cards, err = collectCards(rows)
	if err != nil {
		return nil, false, err
	}
	if len(cards) > f.Limit {
		return cards[:f.Limit], true, nil
	}
	return cards, false, nil
}

const hasPicture = `COALESCE(c.image_key, '') <> ''`

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
