package safety

import (
	"regexp"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// Validator implements the SafetyValidator interface
type Validator struct {
	patterns []types.DangerousPattern
}

// NewValidator creates a new safety validator
func NewValidator() interfaces.SafetyValidator {
	v := &Validator{}
	v.initializePatterns()
	return v
}

// ValidateCommand validates a command for safety
func (v *Validator) ValidateCommand(cmd *types.Command) (*types.SafetyResult, error) {
	// Implementation will be added in later tasks
	return &types.SafetyResult{
		IsSafe:      true,
		DangerLevel: types.Safe,
	}, nil
}

// IsDangerous checks if a command string is dangerous
func (v *Validator) IsDangerous(cmd string) bool {
	// Implementation will be added in later tasks
	return false
}

// GetDangerousPatterns returns all dangerous patterns
func (v *Validator) GetDangerousPatterns() []types.DangerousPattern {
	return v.patterns
}

// initializePatterns initializes the dangerous command patterns
func (v *Validator) initializePatterns() {
	v.patterns = []types.DangerousPattern{
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?/`),
			Description: "File deletion from root directory",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bdd\s+`),
			Description: "Disk duplication command",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\b(shutdown|reboot|halt)\b`),
			Description: "System control commands",
			Level:       types.Dangerous,
		},
		// More patterns will be added in later tasks
	}
}
