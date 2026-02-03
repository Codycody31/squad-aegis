package chat_automod

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// FilterCategory represents the category of a detected violation
type FilterCategory string

const (
	CategoryRacial     FilterCategory = "racial"
	CategoryHomophobic FilterCategory = "homophobic"
	CategoryAbleist    FilterCategory = "ableist"
	CategoryHateSpeech FilterCategory = "hate_speech"
	CategoryCustom     FilterCategory = "custom"
)

// FilterResult contains detection information
type FilterResult struct {
	Detected     bool
	Category     FilterCategory
	MatchedTerms []string
	Severity     int // 1-3 (minor, moderate, severe)
}

// LanguageFilters handles all language filtering logic
type LanguageFilters struct {
	racialPatterns     []*regexp.Regexp
	homophobicPatterns []*regexp.Regexp
	ableistPatterns    []*regexp.Regexp
	hateSpeechPatterns []*regexp.Regexp
	customPatterns     []*regexp.Regexp

	whitelist        map[string]bool
	region           string
	regionalExempt   map[string]map[string]bool // region -> words exempt in that region

	// Enable flags
	enableRacial     bool
	enableHomophobic bool
	enableAbleist    bool
	enableHateSpeech bool
}

// NewLanguageFilters creates a new LanguageFilters instance
func NewLanguageFilters(region string, enableRacial, enableHomophobic, enableAbleist, enableHateSpeech bool) *LanguageFilters {
	f := &LanguageFilters{
		whitelist:        make(map[string]bool),
		region:           strings.ToLower(region),
		regionalExempt:   make(map[string]map[string]bool),
		enableRacial:     enableRacial,
		enableHomophobic: enableHomophobic,
		enableAbleist:    enableAbleist,
		enableHateSpeech: enableHateSpeech,
	}

	// Initialize built-in patterns
	f.initBuiltInPatterns()
	f.initRegionalExemptions()

	return f
}

// initBuiltInPatterns compiles all built-in regex patterns
func (f *LanguageFilters) initBuiltInPatterns() {
	// Racial slurs - patterns designed to catch common variations and evasions
	// These are intentionally broad to catch l33t speak and character substitutions
	racialTerms := []string{
		`n+[i1!|]+g+[e3]+r+s?`,           // n-word and variations
		`n+[i1!|]+g+[a@4]+s?`,            // n-word alternate ending
		`ch+[i1!|]+n+k+s?`,               // anti-Asian slur
		`g+[o0]+[o0]+k+s?`,               // anti-Asian slur
		`sp+[i1!|]+c+s?`,                 // anti-Hispanic slur
		`w+[e3]+t+b+[a@4]+c+k+s?`,        // anti-Hispanic slur
		`b+[e3]+[a@4]+n+[e3]+r+s?`,       // anti-Hispanic slur
		`k+[i1!|]+k+[e3]+s?`,             // antisemitic slur
		`t+[o0]+w+[e3]+l+h+[e3]+[a@4]+d`, // anti-Middle Eastern slur
		`c+[a@4]+m+[e3]+l+j+[o0]+c+k+[e3]+y`, // anti-Middle Eastern slur
		`s+[a@4]+n+d+n+[i1!|]+g+`,        // anti-Middle Eastern slur
		`c+[o0]+[o0]+n+s?`,               // racial slur (contextual)
		`j+[i1!|]+g+[a@4]+b+[o0]+[o0]+`,  // racial slur
		`p+[o0]+r+c+h+m+[o0]+n+k+[e3]+y`, // racial slur
	}

	f.racialPatterns = compilePatterns(racialTerms)

	// Homophobic slurs
	homophobicTerms := []string{
		`f+[a@4]+g+[o0]?[t+]?s?`,     // f-slur and variations
		`f+[a@4]+g+g+[o0]+t+s?`,      // f-slur full form
		`d+[y]+k+[e3]+s?`,            // lesbian slur
		`t+r+[a@4]+n+n+[y1!|]+[e3]?s?`, // trans slur
		`sh+[e3]+m+[a@4]+l+[e3]+s?`,  // trans slur
	}

	f.homophobicPatterns = compilePatterns(homophobicTerms)

	// Ableist slurs
	ableistTerms := []string{
		`r+[e3]+t+[a@4]+r+d+[e3]?d?s?`, // r-word and variations
		`t+[a@4]+r+d+s?`,               // short form
		`sp+[a@4]+z+z?`,                // ableist term (UK specific)
		`m+[o0]+n+g+[o0]?l?[o0]?[i1!|]?d?s?`, // ableist term
	}

	f.ableistPatterns = compilePatterns(ableistTerms)

	// Hate speech patterns - phrases indicating violent intent toward groups
	hateSpeechTerms := []string{
		`k+[i1!|]+l+l+\s*(all|the|every)\s*\w+`,           // "kill all X"
		`d+[e3]+[a@4]+t+h+\s*t+[o0]\s*\w+`,                // "death to X"
		`g+[a@4]+s+\s*t+h+[e3]\s*\w+`,                     // "gas the X"
		`[e3]+x+t+[e3]+r+m+[i1!|]+n+[a@4]+t+[e3]+\s*\w+`,  // "exterminate X"
		`g+[e3]+n+[o0]+c+[i1!|]+d+[e3]?`,                  // "genocide"
		`h+[i1!|]+t+l+[e3]+r+\s*w+[a@4]+s+\s*r+[i1!|]+g+h+t+`, // Nazi glorification
		`s+[i1!|]+[e3]+g+\s*h+[e3]+[i1!|]+l+`,             // Nazi salute
		`wh+[i1!|]+t+[e3]+\s*p+[o0]+w+[e3]+r+`,            // white supremacist phrase
		`r+[a@4]+c+[e3]+\s*w+[a@4]+r+`,                    // "race war"
	}

	f.hateSpeechPatterns = compilePatterns(hateSpeechTerms)
}

// initRegionalExemptions sets up regional word exemptions
func (f *LanguageFilters) initRegionalExemptions() {
	// Australian region - "cunt" is commonly used casually
	f.regionalExempt["au"] = map[string]bool{
		"cunt":  true,
		"cunts": true,
	}

	// UK region
	f.regionalExempt["uk"] = map[string]bool{
		"bloody": true,
		"bollocks": true,
	}

	// EU region - similar to UK
	f.regionalExempt["eu"] = map[string]bool{
		"bloody": true,
	}
}

// compilePatterns compiles a list of pattern strings into regexes
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		// Case insensitive matching
		re, err := regexp.Compile(`(?i)` + p)
		if err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// SetWhitelist sets the whitelist of exempt terms
func (f *LanguageFilters) SetWhitelist(words []string) {
	f.whitelist = make(map[string]bool)
	for _, word := range words {
		f.whitelist[strings.ToLower(word)] = true
	}
}

// SetCustomPatterns sets custom blacklist patterns
func (f *LanguageFilters) SetCustomPatterns(patterns []string) {
	f.customPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		// Try to compile as regex, if fails treat as literal
		re, err := regexp.Compile(`(?i)` + p)
		if err != nil {
			// Escape and compile as literal
			re, err = regexp.Compile(`(?i)` + regexp.QuoteMeta(p))
		}
		if err == nil {
			f.customPatterns = append(f.customPatterns, re)
		}
	}
}

// CheckMessage analyzes a message for violations
func (f *LanguageFilters) CheckMessage(message string) *FilterResult {
	// Normalize the message
	normalized := f.normalize(message)
	original := strings.ToLower(message)

	// Check whitelist - if entire message or significant portions are whitelisted, skip
	if f.isWhitelisted(original) {
		return &FilterResult{Detected: false}
	}

	// Check each enabled category in priority order
	// Priority: racial > homophobic > hate_speech > ableist > custom

	if f.enableRacial {
		if matches := f.checkPatterns(normalized, f.racialPatterns); len(matches) > 0 {
			if !f.areAllRegionallyExempt(matches) {
				return &FilterResult{
					Detected:     true,
					Category:     CategoryRacial,
					MatchedTerms: matches,
					Severity:     3, // Severe
				}
			}
		}
	}

	if f.enableHomophobic {
		if matches := f.checkPatterns(normalized, f.homophobicPatterns); len(matches) > 0 {
			if !f.areAllRegionallyExempt(matches) {
				return &FilterResult{
					Detected:     true,
					Category:     CategoryHomophobic,
					MatchedTerms: matches,
					Severity:     3, // Severe
				}
			}
		}
	}

	if f.enableHateSpeech {
		if matches := f.checkPatterns(normalized, f.hateSpeechPatterns); len(matches) > 0 {
			return &FilterResult{
				Detected:     true,
				Category:     CategoryHateSpeech,
				MatchedTerms: matches,
				Severity:     3, // Severe
			}
		}
	}

	if f.enableAbleist {
		if matches := f.checkPatterns(normalized, f.ableistPatterns); len(matches) > 0 {
			if !f.areAllRegionallyExempt(matches) {
				return &FilterResult{
					Detected:     true,
					Category:     CategoryAbleist,
					MatchedTerms: matches,
					Severity:     2, // Moderate
				}
			}
		}
	}

	// Custom patterns
	if len(f.customPatterns) > 0 {
		if matches := f.checkPatterns(normalized, f.customPatterns); len(matches) > 0 {
			return &FilterResult{
				Detected:     true,
				Category:     CategoryCustom,
				MatchedTerms: matches,
				Severity:     2, // Moderate (configurable severity for custom)
			}
		}
	}

	return &FilterResult{Detected: false}
}

// normalize handles common evasion techniques
func (f *LanguageFilters) normalize(message string) string {
	// Convert to lowercase
	result := strings.ToLower(message)

	// Unicode normalization (NFD form for decomposition)
	result = norm.NFD.String(result)

	// Remove diacritics
	result = removeDiacritics(result)

	// L33t speak substitutions
	l33tMap := map[rune]rune{
		'@': 'a',
		'4': 'a',
		'8': 'b',
		'3': 'e',
		'1': 'i',
		'!': 'i',
		'|': 'i',
		'0': 'o',
		'5': 's',
		'$': 's',
		'7': 't',
		'+': 't',
		'2': 'z',
	}

	var normalized strings.Builder
	for _, r := range result {
		if replacement, ok := l33tMap[r]; ok {
			normalized.WriteRune(replacement)
		} else {
			normalized.WriteRune(r)
		}
	}
	result = normalized.String()

	// Remove special characters between letters (n-i-g-g-e-r -> nigger)
	// But preserve spaces between words
	result = removeInterspersedChars(result)

	// Reduce repeated characters (niiiggaaa -> niga)
	result = reduceRepeatedChars(result)

	return result
}

// removeDiacritics removes accent marks and diacritics
func removeDiacritics(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			// Skip combining marks
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// removeInterspersedChars removes non-alphanumeric chars between letters
func removeInterspersedChars(s string) string {
	var result strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Keep letters, numbers, and spaces
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		}
		// Skip special chars that are between letters (evasion technique)
	}

	return result.String()
}

// reduceRepeatedChars reduces sequences of same character to max 2
func reduceRepeatedChars(s string) string {
	if len(s) == 0 {
		return s
	}

	var result strings.Builder
	runes := []rune(s)
	count := 1

	result.WriteRune(runes[0])

	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			count++
			if count <= 2 {
				result.WriteRune(runes[i])
			}
		} else {
			count = 1
			result.WriteRune(runes[i])
		}
	}

	return result.String()
}

// checkPatterns checks a message against a list of patterns
func (f *LanguageFilters) checkPatterns(message string, patterns []*regexp.Regexp) []string {
	var matches []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		found := pattern.FindAllString(message, -1)
		for _, match := range found {
			if !seen[match] {
				seen[match] = true
				matches = append(matches, match)
			}
		}
	}

	return matches
}

// isWhitelisted checks if the message contains only whitelisted terms
func (f *LanguageFilters) isWhitelisted(message string) bool {
	words := strings.Fields(message)
	for _, word := range words {
		// Clean word of punctuation for matching
		cleaned := strings.Trim(word, ".,!?;:'\"")
		if f.whitelist[cleaned] {
			return true
		}
	}
	return false
}

// areAllRegionallyExempt checks if all matched terms are exempt in the current region
func (f *LanguageFilters) areAllRegionallyExempt(matches []string) bool {
	exemptions, exists := f.regionalExempt[f.region]
	if !exists {
		return false
	}

	for _, match := range matches {
		cleaned := strings.ToLower(strings.TrimSpace(match))
		if !exemptions[cleaned] {
			return false
		}
	}

	return true
}

// GetCategoryDisplayName returns a human-readable name for a category
func GetCategoryDisplayName(category FilterCategory) string {
	switch category {
	case CategoryRacial:
		return "Racial Slur"
	case CategoryHomophobic:
		return "Homophobic Slur"
	case CategoryAbleist:
		return "Ableist Language"
	case CategoryHateSpeech:
		return "Hate Speech"
	case CategoryCustom:
		return "Prohibited Language"
	default:
		return "Language Violation"
	}
}
