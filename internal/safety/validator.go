package safety

import (
	"regexp"
	"strings"
	"time"

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
	if cmd == nil {
		return &types.SafetyResult{
			IsSafe:               false,
			DangerLevel:          types.Critical,
			Warnings:             []string{"Command is nil"},
			RequiresConfirmation: true,
		}, nil
	}

	commandText := cmd.Generated
	if commandText == "" {
		commandText = cmd.Original
	}

	return v.validateCommandString(commandText), nil
}

// ValidateCommandWithOptions validates a command for safety with bypass options
func (v *Validator) ValidateCommandWithOptions(cmd *types.Command, opts *types.ValidationOptions) (*types.SafetyResult, error) {
	// First perform normal validation
	result, err := v.ValidateCommand(cmd)
	if err != nil {
		return result, err
	}

	// If no options provided, return normal result
	if opts == nil {
		return result, nil
	}

	// If command is nil, we can't do any further processing
	if cmd == nil {
		return result, nil
	}

	commandText := cmd.Generated
	if commandText == "" {
		commandText = cmd.Original
	}

	// Create audit entry
	auditEntry := &types.AuditEntry{
		Timestamp:   time.Now(),
		Command:     commandText,
		UserID:      opts.UserID,
		DangerLevel: result.DangerLevel,
		Reason:      opts.Reason,
		SessionID:   "", // TODO: Add session tracking
	}

	// Determine if bypass should be applied
	shouldBypass := opts.SkipConfirmation &&
		result.DangerLevel <= opts.BypassLevel &&
		result.DangerLevel > types.Safe

	if shouldBypass {
		// Apply bypass
		result.RequiresConfirmation = false
		result.Bypassed = true
		result.Warnings = append(result.Warnings, "Safety check bypassed by user")
		auditEntry.Action = types.AuditActionBypassed

		// Log audit event if logger is provided
		if opts.AuditLogger != nil {
			if logErr := opts.AuditLogger.LogAuditEvent(auditEntry); logErr != nil {
				// Don't fail validation due to logging error, but add warning
				result.Warnings = append(result.Warnings, "Failed to log audit event: "+logErr.Error())
			}
		}
	} else {
		// Normal validation or blocked
		if result.DangerLevel > types.Safe {
			auditEntry.Action = types.AuditActionValidated
		} else {
			auditEntry.Action = types.AuditActionValidated
		}

		// Log audit event if logger is provided and configured to audit all
		if opts.AuditLogger != nil {
			if logErr := opts.AuditLogger.LogAuditEvent(auditEntry); logErr != nil {
				// Don't fail validation due to logging error
				result.Warnings = append(result.Warnings, "Failed to log audit event: "+logErr.Error())
			}
		}
	}

	result.AuditEntry = auditEntry
	return result, nil
}

// IsDangerous checks if a command string is dangerous
func (v *Validator) IsDangerous(cmd string) bool {
	result := v.validateCommandString(cmd)
	return result.DangerLevel > types.Safe
}

// GetDangerousPatterns returns all dangerous patterns
func (v *Validator) GetDangerousPatterns() []types.DangerousPattern {
	return v.patterns
}

// validateCommandString performs the actual validation logic
func (v *Validator) validateCommandString(cmd string) *types.SafetyResult {
	if cmd == "" {
		return &types.SafetyResult{
			IsSafe:               true,
			DangerLevel:          types.Safe,
			Warnings:             []string{},
			RequiresConfirmation: false,
		}
	}

	// Normalize the command for analysis
	normalizedCmd := strings.TrimSpace(strings.ToLower(cmd))

	var maxDangerLevel types.DangerLevel = types.Safe
	var warnings []string

	// Check against all patterns
	for _, pattern := range v.patterns {
		if pattern.Pattern.MatchString(normalizedCmd) {
			warnings = append(warnings, pattern.Description)

			if pattern.Level > maxDangerLevel {
				maxDangerLevel = pattern.Level
			}
		}
	}

	// Apply context-aware analysis to refine danger level
	contextAwareDangerLevel, contextWarnings := v.analyzeCommandContext(normalizedCmd, maxDangerLevel)
	if contextAwareDangerLevel != maxDangerLevel {
		maxDangerLevel = contextAwareDangerLevel
	}
	warnings = append(warnings, contextWarnings...)

	// Determine if confirmation is required
	requiresConfirmation := maxDangerLevel >= types.Warning

	return &types.SafetyResult{
		IsSafe:               maxDangerLevel == types.Safe,
		DangerLevel:          maxDangerLevel,
		Warnings:             warnings,
		RequiresConfirmation: requiresConfirmation,
	}
}

// initializePatterns initializes the dangerous command patterns
func (v *Validator) initializePatterns() {
	v.patterns = []types.DangerousPattern{
		// Critical patterns - extremely dangerous operations
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?/(\s|$)`),
			Description: "File deletion from root directory",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?/(bin|boot|dev|etc|lib|lib64|proc|root|sbin|sys|usr)(/|\s|$)`),
			Description: "Deletion of critical system directories",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?/var/(cache|lib|log|mail|opt|run|spool|www)(/|\s|$)`),
			Description: "Deletion of critical /var subdirectories",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bdd\s+.*of=/dev/(sd[a-z]|nvme[0-9]|hd[a-z])`),
			Description: "Writing to physical disk devices",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bdd\s+if=/dev/zero\s+of=/dev/`),
			Description: "Disk wiping with dd command",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\b(shutdown|poweroff|halt)(\s+(-[a-z]*\s+)?(now|\+0|0))?(\s|$)`),
			Description: "System shutdown command",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\breboot(\s+(now|\+0|0))?(\s|$)`),
			Description: "System reboot command",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bmkfs\.[a-z0-9]+\s+/dev/`),
			Description: "File system creation on device",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bfdisk\s+/dev/`),
			Description: "Disk partitioning command",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bparted\s+/dev/`),
			Description: "Disk partitioning with parted",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bcfdisk\s+/dev/`),
			Description: "Disk partitioning with cfdisk",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bkill\s+(-[0-9]+\s+)?1(\s|$)`),
			Description: "Killing init process (PID 1)",
			Level:       types.Critical,
		},
		{
			Pattern:     regexp.MustCompile(`\bkillall\s+(-[0-9]+\s+)?(init|systemd|kernel)`),
			Description: "Killing critical system processes",
			Level:       types.Critical,
		},

		// Dangerous patterns - potentially harmful operations
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?[^/\s]*\*`),
			Description: "Wildcard file deletion",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?[a-zA-Z0-9_./]+`),
			Description: "File deletion command",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?~`),
			Description: "Deletion of home directory",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\brm\s+(-[rf]*\s+)?\$home`),
			Description: "Deletion of home directory via variable",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\brmdir\s+(-[a-z]*\s+)?/`),
			Description: "Directory removal from root",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bchmod\s+(-[a-z]*\s+)?777\b`),
			Description: "Setting overly permissive file permissions",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bchmod\s+(-[a-z]*\s+)?[0-7]*[67][67]`),
			Description: "Setting world-writable permissions",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bsu\s+(-[a-z]*\s+)?root`),
			Description: "Switch to root user",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bsudo\s+`),
			Description: "Elevated privilege command",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bdoas\s+`),
			Description: "Elevated privilege command (doas)",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bpkill\s+(-[0-9]+\s+)?(ssh|sshd|systemd|init)`),
			Description: "Killing critical system services",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bsystemctl\s+(stop|disable|mask)\s+(ssh|sshd|network|NetworkManager)`),
			Description: "Disabling critical system services",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bservice\s+(ssh|sshd|network|networking)\s+stop`),
			Description: "Stopping critical system services",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bufw\s+(--force\s+)?reset`),
			Description: "Resetting firewall rules",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\biptables\s+(-F|-X|--flush)`),
			Description: "Flushing firewall rules",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bfind\s+.*-delete`),
			Description: "Find command with delete action",
			Level:       types.Dangerous,
		},
		{
			Pattern:     regexp.MustCompile(`\bfind\s+.*-exec\s+rm`),
			Description: "Find command executing rm",
			Level:       types.Dangerous,
		},

		// Warning patterns - operations that need attention
		{
			Pattern:     regexp.MustCompile(`\bmv\s+.*\s+/(bin|boot|etc|lib|sbin|usr|var)`),
			Description: "Moving files to system directories",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bcp\s+.*\s+/(bin|boot|etc|lib|sbin|usr|var)`),
			Description: "Copying files to system directories",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bchown\s+(-[a-z]*\s+)?root:`),
			Description: "Changing ownership to root",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bchown\s+`),
			Description: "Changing file ownership",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bchmod\s+`),
			Description: "Changing file permissions",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`>\s*/dev/(sd[a-z]|nvme[0-9]|hd[a-z])`),
			Description: "Writing directly to disk devices",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`>\s*/dev/[a-z0-9]+`),
			Description: "Writing to device files",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bwget\s+.*\|\s*(sh|bash|zsh|fish)`),
			Description: "Downloading and executing scripts",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bcurl\s+.*\|\s*(sh|bash|zsh|fish)`),
			Description: "Downloading and executing scripts",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bmount\s+.*\s+/`),
			Description: "Mounting filesystems",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bumount\s+/`),
			Description: "Unmounting filesystems",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bkill\s+(-[0-9]+\s+)?[0-9]+`),
			Description: "Killing processes by PID",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bkillall\s+`),
			Description: "Killing processes by name",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bpkill\s+`),
			Description: "Killing processes by pattern",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bcrontab\s+(-[a-z]*\s+)?-r`),
			Description: "Removing all cron jobs",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bhistory\s+(-[a-z]*\s+)?-c`),
			Description: "Clearing command history",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bexport\s+path\s*=`),
			Description: "Modifying PATH environment variable",
			Level:       types.Warning,
		},
		{
			Pattern:     regexp.MustCompile(`\bunset\s+path(\s|$)`),
			Description: "Unsetting PATH environment variable",
			Level:       types.Warning,
		},
	}
}

// analyzeCommandContext performs context-aware analysis of commands
func (v *Validator) analyzeCommandContext(cmd string, currentLevel types.DangerLevel) (types.DangerLevel, []string) {
	var warnings []string
	dangerLevel := currentLevel

	// Context-aware analysis for rm commands
	if strings.Contains(cmd, "rm ") {
		// Check for rm with specific dangerous combinations first
		if v.hasRmDangerousCombination(cmd) {
			dangerLevel = types.Critical
			warnings = append(warnings, "Extremely dangerous rm command combination detected")
		} else if strings.Contains(cmd, "$home") || strings.Contains(cmd, "~") {
			// $HOME or ~ deletion should be dangerous but not critical
			dangerLevel = types.Dangerous
			warnings = append(warnings, "Home directory deletion detected")
		} else if v.isRmInSafeDirectory(cmd) {
			// If rm is targeting files in safe directories like /tmp or current dir,
			// should be dangerous regardless of initial level
			dangerLevel = types.Dangerous
			warnings = append(warnings, "File deletion in potentially safe directory, but still requires caution")
		} else if v.isRmTargetingSystemPaths(cmd) {
			// System paths should remain critical
			if dangerLevel < types.Critical {
				dangerLevel = types.Critical
			}
		}
	}

	// Context-aware analysis for dd commands
	if strings.Contains(cmd, "dd ") {
		if v.isDdToPhysicalDevice(cmd) {
			dangerLevel = types.Critical
			warnings = append(warnings, "dd command targeting physical device - potential data loss")
		}
	}

	// Context-aware analysis for chmod commands
	if strings.Contains(cmd, "chmod ") {
		if v.isChmodOnSystemFiles(cmd) {
			if dangerLevel < types.Dangerous {
				dangerLevel = types.Dangerous
			}
			warnings = append(warnings, "chmod on system files detected")
		}
	}

	// Context-aware analysis for network/service commands
	if v.isNetworkServiceCommand(cmd) {
		if dangerLevel < types.Dangerous {
			dangerLevel = types.Dangerous
		}
		warnings = append(warnings, "Command affects network services or connectivity")
	}

	return dangerLevel, warnings
}

// isRmInSafeDirectory checks if rm command is targeting relatively safe directories
func (v *Validator) isRmInSafeDirectory(cmd string) bool {
	safePatterns := []string{
		`\brm\s+.*\./`,            // Current directory
		`\brm\s+.*/tmp/`,          // Temp directory
		`\brm\s+.*/var/tmp/`,      // Var temp directory
		`\brm\s+.*\$\{?tmpdir\}?`, // TMPDIR variable
	}

	for _, pattern := range safePatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}
	return false
}

// isRmTargetingSystemPaths checks if rm is targeting critical system paths
func (v *Validator) isRmTargetingSystemPaths(cmd string) bool {
	systemPatterns := []string{
		`\brm\s+.*/(bin|boot|dev|etc|lib|lib64|proc|root|sbin|sys|usr)(/|\s|$)`,
		`\brm\s+.*/var/(cache|lib|log|mail|opt|run|spool|www)`, // /var/ subdirs but not /var/tmp
		`\brm\s+.*/\*`,
		`\brm\s+.*\$home`,
		`\brm\s+.*~`,
	}

	for _, pattern := range systemPatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}
	return false
}

// hasRmDangerousCombination checks for particularly dangerous rm combinations
func (v *Validator) hasRmDangerousCombination(cmd string) bool {
	dangerousPatterns := []string{
		`\brm\s+-rf\s+/\*`,        // rm -rf /*
		`\brm\s+-rf\s+/(\s|$)`,    // rm -rf / (end of command or followed by space)
		`\brm\s+-rf\s+\\\$\(.*\)`, // rm -rf $(command)
		`\brm\s+-rf\s+\*\*`,       // rm -rf **
		`\brm\s+.*\|\|.*\brm\s+`,  // rm with fallback rm
	}

	for _, pattern := range dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}
	return false
}

// isDdToPhysicalDevice checks if dd command targets physical devices
func (v *Validator) isDdToPhysicalDevice(cmd string) bool {
	devicePatterns := []string{
		`of=/dev/sd[a-z]`,
		`of=/dev/nvme[0-9]`,
		`of=/dev/hd[a-z]`,
		`of=/dev/mmcblk[0-9]`,
	}

	for _, pattern := range devicePatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}
	return false
}

// isChmodOnSystemFiles checks if chmod is being applied to system files
func (v *Validator) isChmodOnSystemFiles(cmd string) bool {
	systemFilePatterns := []string{
		`\bchmod\s+.*/(bin|boot|dev|etc|lib|lib64|proc|root|sbin|sys|usr|var)/`,
		`\bchmod\s+.*/etc/passwd`,
		`\bchmod\s+.*/etc/shadow`,
		`\bchmod\s+.*/etc/sudoers`,
	}

	for _, pattern := range systemFilePatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}
	return false
}

// isNetworkServiceCommand checks if command affects network services
func (v *Validator) isNetworkServiceCommand(cmd string) bool {
	networkPatterns := []string{
		`\bsystemctl\s+(stop|disable|mask)\s+(ssh|sshd|network|networkmanager|firewall)`,
		`\bservice\s+(ssh|sshd|network|networking|iptables)\s+(stop|disable)`,
		`\bufw\s+(disable|reset)`,
		`\biptables\s+-f`,
		`\biptables\s+-x`,
		`\biptables\s+--flush`,
		`\bip6tables\s+(-f|-x|--flush)`,
		`\bifconfig\s+.*down`,
		`\bip\s+link\s+set\s+.*down`,
	}

	for _, pattern := range networkPatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}
	return false
}
