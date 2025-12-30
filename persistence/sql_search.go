package persistence

import (
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/str"
)

func formatFullText(text ...string) string {
	fullText := str.SanitizeStrings(text...)
	return " " + fullText
}

// doSearch performs a full-text search with the specified parameters.
// The naturalOrder is used to sort results when no full-text filter is applied. It is useful for cases like
// OpenSubsonic, where an empty search query should return all results in a natural order. Normally the parameter
// should be `tableName + ".rowid"`, but some repositories (ex: artist) may use a different natural order.
//
// Advanced search operators are supported:
//   - field:value - filter by specific field (e.g., artist:Beatles, year:2020)
//   - field:"multi word" - quoted values for multi-word matches
//   - field:min-max - range queries (e.g., year:2010-2020)
//   - field:n+ - greater than or equal (e.g., rating:4+)
func (r sqlRepository) doSearch(sq SelectBuilder, q string, offset, size int, results any, naturalOrder string, orderBys ...string) error {
	q = strings.TrimSpace(q)
	q = strings.TrimSuffix(q, "*")
	if len(q) < 2 {
		return nil
	}

	// Parse for advanced search operators (field:value syntax)
	parsed := ParseAdvancedSearch(r.tableName, q)

	// Apply advanced search filters first
	sq = ApplyAdvancedSearch(sq, parsed)

	// Apply remaining full-text search on the unparsed portion
	filter := fullTextExpr(r.tableName, parsed.FullText)
	if filter != nil {
		sq = sq.Where(filter)
		sq = sq.OrderBy(orderBys...)
	} else if len(parsed.Filters) > 0 {
		// If we have field filters but no full-text, still apply sorting
		sq = sq.OrderBy(orderBys...)
	} else {
		// This is to speed up the results of `search3?query=""`, for OpenSubsonic
		// If the filter is empty, we sort by the specified natural order.
		sq = sq.OrderBy(naturalOrder)
	}
	sq = sq.Where(Eq{r.tableName + ".missing": false})
	sq = sq.Limit(uint64(size)).Offset(uint64(offset))
	return r.queryAll(sq, results, model.QueryOptions{Offset: offset})
}

func (r sqlRepository) searchByMBID(sq SelectBuilder, mbid string, mbidFields []string, results any) error {
	sq = sq.Where(mbidExpr(r.tableName, mbid, mbidFields...))
	sq = sq.Where(Eq{r.tableName + ".missing": false})

	return r.queryAll(sq, results)
}

func mbidExpr(tableName, mbid string, mbidFields ...string) Sqlizer {
	if uuid.Validate(mbid) != nil || len(mbidFields) == 0 {
		return nil
	}
	mbid = strings.ToLower(mbid)
	var cond []Sqlizer
	for _, mbidField := range mbidFields {
		cond = append(cond, Eq{tableName + "." + mbidField: mbid})
	}
	return Or(cond)
}

func fullTextExpr(tableName string, s string) Sqlizer {
	q := str.SanitizeStrings(s)
	if q == "" {
		return nil
	}
	var sep string
	if !conf.Server.SearchFullString {
		sep = " "
	}
	parts := strings.Split(q, " ")
	filters := And{}
	for _, part := range parts {
		filters = append(filters, Like{tableName + ".full_text": "%" + sep + part + "%"})
	}
	return filters
}
