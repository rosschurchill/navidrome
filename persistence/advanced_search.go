package persistence

import (
	"regexp"
	"strconv"
	"strings"

	. "github.com/Masterminds/squirrel"
)

// ParsedSearch represents a parsed search query with operators
type ParsedSearch struct {
	// FullText contains the remaining text for full-text search
	FullText string
	// Filters contains field-specific filters parsed from the query
	Filters And
}

// AdvancedSearchFields defines the supported search operators
var AdvancedSearchFields = map[string]string{
	"artist":      "media_file.artist",
	"album":       "media_file.album",
	"title":       "media_file.title",
	"genre":       "media_file.genre",
	"year":        "media_file.year",
	"rating":      "COALESCE(annotation.rating, 0)",
	"plays":       "COALESCE(annotation.play_count, 0)",
	"loved":       "COALESCE(annotation.starred, false)",
	"format":      "media_file.suffix",
	"bpm":         "media_file.bpm",
	"albumartist": "media_file.album_artist",
	"composer":    "media_file.composer",
	"path":        "media_file.path",
}

// Patterns for parsing search operators
var (
	// field:value pattern (e.g., artist:Beatles, year:2020)
	fieldPattern = regexp.MustCompile(`(\w+):([^\s"]+|"[^"]+")`)
	// range pattern for numeric values (e.g., year:2010-2020)
	rangePattern = regexp.MustCompile(`^(\d+)-(\d+)$`)
	// comparison pattern (e.g., rating:4+, year:>2000)
	comparisonPattern = regexp.MustCompile(`^([<>]=?)(\d+)$`)
	// numeric plus pattern (e.g., rating:4+)
	plusPattern = regexp.MustCompile(`^(\d+)\+$`)
)

// ParseAdvancedSearch parses a search query for field-specific operators
// Supported syntax:
//   - field:value - exact field match (e.g., artist:Beatles)
//   - field:"multi word" - quoted value for multi-word matches
//   - field:min-max - range query (e.g., year:2010-2020)
//   - field:n+ - greater than or equal (e.g., rating:4+)
//   - field:>n, field:<n, field:>=n, field:<=n - comparisons
//
// Remaining text is used for full-text search
func ParseAdvancedSearch(tableName, query string) ParsedSearch {
	result := ParsedSearch{
		FullText: query,
		Filters:  And{},
	}

	// Find all field:value patterns
	matches := fieldPattern.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return result
	}

	// Process each match
	for _, match := range matches {
		field := strings.ToLower(match[1])
		value := match[2]

		// Remove quotes from value if present
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}

		// Check if this is a supported field
		dbField, ok := AdvancedSearchFields[field]
		if !ok {
			continue
		}

		// Build the filter based on value pattern
		filter := buildFilter(dbField, value)
		if filter != nil {
			result.Filters = append(result.Filters, filter)
			// Remove the matched pattern from full-text query
			result.FullText = strings.Replace(result.FullText, match[0], "", 1)
		}
	}

	// Clean up remaining full-text query - normalize multiple spaces to single space
	result.FullText = strings.TrimSpace(result.FullText)
	// Replace multiple consecutive spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	result.FullText = spaceRegex.ReplaceAllString(result.FullText, " ")

	return result
}

// buildFilter creates a Sqlizer filter based on the value pattern
func buildFilter(field, value string) Sqlizer {
	// Check for range pattern (e.g., 2010-2020)
	if matches := rangePattern.FindStringSubmatch(value); matches != nil {
		min, _ := strconv.Atoi(matches[1])
		max, _ := strconv.Atoi(matches[2])
		return And{
			GtOrEq{field: min},
			LtOrEq{field: max},
		}
	}

	// Check for plus pattern (e.g., 4+)
	if matches := plusPattern.FindStringSubmatch(value); matches != nil {
		num, _ := strconv.Atoi(matches[1])
		return GtOrEq{field: num}
	}

	// Check for comparison pattern (e.g., >2000, >=4)
	if matches := comparisonPattern.FindStringSubmatch(value); matches != nil {
		op := matches[1]
		num, _ := strconv.Atoi(matches[2])
		switch op {
		case ">":
			return Gt{field: num}
		case "<":
			return Lt{field: num}
		case ">=":
			return GtOrEq{field: num}
		case "<=":
			return LtOrEq{field: num}
		}
	}

	// Check for boolean values
	lowerValue := strings.ToLower(value)
	if lowerValue == "true" || lowerValue == "yes" || lowerValue == "1" {
		return Eq{field: true}
	}
	if lowerValue == "false" || lowerValue == "no" || lowerValue == "0" {
		return Eq{field: false}
	}

	// Default to LIKE match for string fields
	if isStringField(field) {
		return Like{field: "%" + value + "%"}
	}

	// Exact match for numeric/other fields
	return Eq{field: value}
}

// isStringField returns true if the field should use LIKE matching
func isStringField(field string) bool {
	stringFields := map[string]bool{
		"media_file.artist":       true,
		"media_file.album":        true,
		"media_file.title":        true,
		"media_file.genre":        true,
		"media_file.album_artist": true,
		"media_file.composer":     true,
		"media_file.path":         true,
	}
	return stringFields[field]
}

// ApplyAdvancedSearch applies parsed search filters to a SelectBuilder
func ApplyAdvancedSearch(sq SelectBuilder, parsed ParsedSearch) SelectBuilder {
	if len(parsed.Filters) > 0 {
		sq = sq.Where(parsed.Filters)
	}
	return sq
}
