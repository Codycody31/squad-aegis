package squadlayers

import (
	"strings"
	"testing"
)

func TestClassifyLayer(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  LayerCategory
	}{
		{"jensens range layer", []string{"JensensRange_Vehicles_v1"}, CategoryTraining},
		{"jensens map classname", []string{"", "JensensRange", ""}, CategoryTraining},
		{"tutorial", []string{"Tutorial_Helicopter"}, CategoryTraining},
		{"seed layer", []string{"AlBasrah_Seed_v1"}, CategorySeeding},
		{"seed lowercase", []string{"sumari_seed_v1"}, CategorySeeding},
		{"competitive raas", []string{"Yehorivka_RAAS_v1"}, CategoryCompetitive},
		{"competitive aas", []string{"Gorodok_AAS_v2", "Gorodok", "Gorodok_AAS_v2"}, CategoryCompetitive},
		{"empty is competitive", []string{""}, CategoryCompetitive},
		{"no args is competitive", nil, CategoryCompetitive},
		{"training wins over seeding", []string{"JensensRange_Seed"}, CategoryTraining},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyLayer(tt.names...); got != tt.want {
				t.Errorf("ClassifyLayer(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

func TestLayerCategorySQL(t *testing.T) {
	expr := LayerCategorySQL("g.layer", "g.map_classname", "g.layer_classname")

	for _, want := range []string{
		"multiIf(",
		"g.layer",
		"g.map_classname",
		"g.layer_classname",
		"'jensen'",
		"'seed'",
		string(CategoryTraining),
		string(CategorySeeding),
		string(CategoryCompetitive),
	} {
		if !strings.Contains(expr, want) {
			t.Errorf("LayerCategorySQL() = %q, missing %q", expr, want)
		}
	}
}
