package motd

import (
	"fmt"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/models"
)

// Generator handles MOTD content generation
type Generator struct{}

// NewGenerator creates a new MOTD generator
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateMOTD creates the MOTD content from rules and config
func (g *Generator) GenerateMOTD(config *models.ServerMOTDConfig, rules []models.ServerRule) string {
	var builder strings.Builder

	// Write prefix
	if config.PrefixText != "" {
		builder.WriteString(config.PrefixText)
		builder.WriteString("\n\n")
	}

	// Write rules section
	if config.AutoGenerateFromRules && len(rules) > 0 {
		g.writeRules(&builder, rules, config.IncludeRuleDescriptions, "", 0)
	}

	// Write suffix
	if config.SuffixText != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(config.SuffixText)
	}

	return builder.String()
}

// writeRules recursively writes rules with proper numbering
func (g *Generator) writeRules(builder *strings.Builder, rules []models.ServerRule, includeDescriptions bool, prefix string, depth int) {
	for i, rule := range rules {
		var number string
		if prefix == "" {
			number = fmt.Sprintf("%d", i+1)
		} else {
			number = fmt.Sprintf("%s.%d", prefix, i+1)
		}

		// Calculate indentation based on depth
		indent := strings.Repeat("    ", depth)

		// Write rule title
		builder.WriteString(fmt.Sprintf("%s%s. %s\n", indent, number, rule.Title))

		// Write description if enabled and present
		if includeDescriptions && rule.Description != "" {
			// Handle multi-line descriptions
			lines := strings.Split(rule.Description, "\n")
			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine != "" {
					// Check if line already starts with a bullet or asterisk
					if strings.HasPrefix(trimmedLine, "*") || strings.HasPrefix(trimmedLine, "-") || strings.HasPrefix(trimmedLine, "•") {
						builder.WriteString(fmt.Sprintf("%s    %s\n", indent, trimmedLine))
					} else {
						builder.WriteString(fmt.Sprintf("%s    * %s\n", indent, trimmedLine))
					}
				}
			}
		}

		// Process sub-rules recursively
		if len(rule.SubRules) > 0 {
			g.writeRules(builder, rule.SubRules, includeDescriptions, number, depth+1)
		}
	}
}

// CountRules returns the total number of rules (including sub-rules)
func (g *Generator) CountRules(rules []models.ServerRule) int {
	count := len(rules)
	for _, rule := range rules {
		count += g.CountRules(rule.SubRules)
	}
	return count
}
