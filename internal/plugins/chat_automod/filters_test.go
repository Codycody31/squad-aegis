package chat_automod

import "testing"

func newTestFilters() *LanguageFilters {
	return NewLanguageFilters("us", true, true, true)
}

func TestCheckMessageNoFalsePositives(t *testing.T) {
	t.Parallel()

	// boundary/narrowing fixes: these contain a retained pattern as a substring and must NOT match
	boundary := []string{
		"you absolute bastard",
		"pass me the mustard",
		"that custard tart was great",
		"I love Mongolia and Mongolian food",
		"flame retardant material",
	}

	// regression guards: terms deliberately dropped from the built-in list must stay dropped
	dropped := []string{
		"the dyke held back the flood",
		"need to fix the tranny in my truck",
		"the shemale scammer was banned",
		"some towelhead comment",
		"a raccoon got into the trash", // clean because "coon" pattern was DROPPED, not anchored
	}

	f := newTestFilters()
	for _, msg := range append(boundary, dropped...) {
		if r := f.CheckMessage(msg); r.Detected {
			t.Fatalf("expected %q to be clean, got category=%s terms=%v", msg, r.Category, r.MatchedTerms)
		}
	}
}

func TestCheckMessageStillDetectsSlurs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		msg  string
		want FilterCategory
	}{
		{"you are a nigger", CategoryRacial},
		{"stop being a spic", CategoryRacial},
		{"what a faggot", CategoryHomophobic},
		{"that is retarded", CategoryAbleist},
		{"you mongoloid", CategoryAbleist},
		{"you m0ng0l0id", CategoryAbleist},
		{"that chink comment", CategoryRacial},
		{"stop being a gook", CategoryRacial},
		{"you kike", CategoryRacial},
		{"go back wetback", CategoryRacial},
		{"what a beaner", CategoryRacial},
		{"sandnigger", CategoryRacial},
		{"you spaz", CategoryAbleist},
		{"jigaboo", CategoryRacial},
		{"you porchmonkey", CategoryRacial},
	}
	f := newTestFilters()
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			r := f.CheckMessage(tc.msg)
			if !r.Detected || r.Category != tc.want {
				t.Fatalf("msg %q: expected detected category=%s, got detected=%v category=%s", tc.msg, tc.want, r.Detected, r.Category)
			}
			if len(r.MatchedTerms) == 0 {
				t.Fatalf("msg %q: expected at least one matched term, got none", tc.msg)
			}
		})
	}
}
