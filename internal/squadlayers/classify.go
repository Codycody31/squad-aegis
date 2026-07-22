// Package squadlayers classifies Squad map/layer names into play-context
// categories so teamkill metrics can distinguish real-match play from training
// (Jensen's Range) and seeding layers, where teamkills carry no admin signal.
//
// The pattern lists are the single source of truth shared by the Go classifier
// (ClassifyLayer, used for live data) and the ClickHouse classifier
// (LayerCategorySQL, used for historical aggregates), so both stay in lockstep.
package squadlayers

import (
	"fmt"
	"strings"
)

// LayerCategory is the play-context bucket a layer-scoped event falls into.
type LayerCategory string

const (
	// CategoryCompetitive is real-match play: the only context that counts
	// toward the headline TK rate and risk flags.
	CategoryCompetitive LayerCategory = "competitive"
	// CategorySeeding is a low-population seeding layer.
	CategorySeeding LayerCategory = "seeding"
	// CategoryTraining is a training map such as Jensen's Range.
	CategoryTraining LayerCategory = "training"
)

// Pattern lists are matched case-insensitively as substrings against a
// combined layer/map/level name. Training is checked before seeding.
var (
	TrainingLayerPatterns = []string{"jensen", "training", "tutorial", "range"}
	SeedingLayerPatterns  = []string{"seed"}
)

// ClassifyLayer returns the category for the given layer/map name(s). Pass any
// combination of layer, level, map_classname and layer_classname. The first
// matching training pattern wins, then seeding, otherwise competitive. Empty or
// unknown input classifies as competitive (the safe default: it never hides a
// real teamkill).
func ClassifyLayer(names ...string) LayerCategory {
	haystack := strings.ToLower(strings.Join(names, "|"))
	if containsAny(haystack, TrainingLayerPatterns) {
		return CategoryTraining
	}
	if containsAny(haystack, SeedingLayerPatterns) {
		return CategorySeeding
	}
	return CategoryCompetitive
}

func containsAny(haystack string, patterns []string) bool {
	for _, p := range patterns {
		if p != "" && strings.Contains(haystack, p) {
			return true
		}
	}
	return false
}

// LayerCategorySQL returns a ClickHouse expression that evaluates to one of the
// LayerCategory string values from the given nullable layer/map/layer-class
// columns (e.g. "g.layer", "g.map_classname", "g.layer_classname"). It uses the
// same pattern lists as ClassifyLayer. A NULL/empty layer classifies as
// competitive.
//
// The patterns are compile-time constants, never user input, so the emitted
// literals are safe to inline.
func LayerCategorySQL(layerCol, mapCol, layerClassCol string) string {
	ctx := fmt.Sprintf(
		"concat(lower(coalesce(%s, '')), '|', lower(coalesce(%s, '')), '|', lower(coalesce(%s, '')))",
		layerCol, mapCol, layerClassCol,
	)
	return fmt.Sprintf(
		"multiIf("+
			"arrayExists(p -> position(%[1]s, p) > 0, %[2]s), '%[3]s', "+
			"arrayExists(p -> position(%[1]s, p) > 0, %[4]s), '%[5]s', "+
			"'%[6]s')",
		ctx,
		sqlPatternArray(TrainingLayerPatterns), CategoryTraining,
		sqlPatternArray(SeedingLayerPatterns), CategorySeeding,
		CategoryCompetitive,
	)
}

// sqlPatternArray renders the patterns as a lowercased ClickHouse string array
// literal, e.g. ['jensen', 'training'].
func sqlPatternArray(patterns []string) string {
	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = "'" + strings.ReplaceAll(strings.ToLower(p), "'", "\\'") + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
