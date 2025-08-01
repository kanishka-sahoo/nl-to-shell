package safety

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	if validator == nil {
		t.Fatal("NewValidator() returned nil")
	}

	patterns := validator.GetDangerousPatterns()
	if len(patterns) == 0 {
		t.Error("Expected dangerous patterns to be initialized, got empty slice")
	}
}

func TestValidateCommand_NilCommand(t *testing.T) {
	validator := NewValidator()
	result, err := validator.ValidateCommand(nil)

	if err != nil {
		t.Errorf("ValidateCommand(nil) returned error: %v", err)
	}

	if result.IsSafe {
		t.Error("Expected nil command to be unsafe")
	}

	if result.DangerLevel != types.Critical {
		t.Errorf("Expected Critical danger level for nil command, got %v", result.DangerLevel)
	}

	if !result.RequiresConfirmation {
		t.Error("Expected nil command to require confirmation")
	}
}

func TestValidateCommand_EmptyCommand(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "",
		Original:  "",
		Timestamp: time.Now(),
	}

	result, err := validator.ValidateCommand(cmd)

	if err != nil {
		t.Errorf("ValidateCommand() returned error: %v", err)
	}

	if !result.IsSafe {
		t.Error("Expected empty command to be safe")
	}

	if result.DangerLevel != types.Safe {
		t.Errorf("Expected Safe danger level for empty command, got %v", result.DangerLevel)
	}

	if result.RequiresConfirmation {
		t.Error("Expected empty command to not require confirmation")
	}
}

func TestValidateCommand_SafeCommands(t *testing.T) {
	validator := NewValidator()
	safeCommands := []string{
		"ls -la",
		"pwd",
		"echo hello",
		"cat file.txt",
		"grep pattern file.txt",
		"find . -name '*.go'",
		"ps aux",
		"top",
		"df -h",
		"du -sh",
		"whoami",
		"date",
		"history",
	}

	for _, cmdStr := range safeCommands {
		t.Run(cmdStr, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: cmdStr,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", cmdStr, err)
			}

			if !result.IsSafe {
				t.Errorf("Expected command %q to be safe, but got unsafe with warnings: %v", cmdStr, result.Warnings)
			}

			if result.DangerLevel != types.Safe {
				t.Errorf("Expected Safe danger level for %q, got %v", cmdStr, result.DangerLevel)
			}

			if result.RequiresConfirmation {
				t.Errorf("Expected safe command %q to not require confirmation", cmdStr)
			}
		})
	}
}

func TestValidateCommand_CriticalCommands(t *testing.T) {
	validator := NewValidator()
	criticalCommands := []struct {
		command     string
		description string
	}{
		{"rm -rf /", "root directory deletion"},
		{"rm -f /etc/passwd", "system file deletion"},
		{"dd if=/dev/zero of=/dev/sda", "disk wiping"},
		{"shutdown now", "system shutdown"},
		{"reboot", "system reboot"},
		{"halt", "system halt"},
		{"mkfs.ext4 /dev/sda1", "file system creation"},
		{"fdisk /dev/sda", "disk partitioning"},
	}

	for _, tc := range criticalCommands {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.IsSafe {
				t.Errorf("Expected command %q (%s) to be unsafe", tc.command, tc.description)
			}

			if result.DangerLevel != types.Critical {
				t.Errorf("Expected Critical danger level for %q, got %v", tc.command, result.DangerLevel)
			}

			if !result.RequiresConfirmation {
				t.Errorf("Expected critical command %q to require confirmation", tc.command)
			}

			if len(result.Warnings) == 0 {
				t.Errorf("Expected warnings for critical command %q", tc.command)
			}
		})
	}
}

func TestValidateCommand_DangerousCommands(t *testing.T) {
	validator := NewValidator()
	dangerousCommands := []struct {
		command     string
		description string
	}{
		{"rm -rf *.log", "wildcard file deletion"},
		{"rmdir /tmp/test", "directory removal"},
		{"chmod 777 file.txt", "overly permissive permissions"},
		{"su root", "switch user"},
		{"sudo apt update", "elevated privilege command"},
	}

	for _, tc := range dangerousCommands {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.IsSafe {
				t.Errorf("Expected command %q (%s) to be unsafe", tc.command, tc.description)
			}

			if result.DangerLevel != types.Dangerous {
				t.Errorf("Expected Dangerous danger level for %q, got %v", tc.command, result.DangerLevel)
			}

			if !result.RequiresConfirmation {
				t.Errorf("Expected dangerous command %q to require confirmation", tc.command)
			}

			if len(result.Warnings) == 0 {
				t.Errorf("Expected warnings for dangerous command %q", tc.command)
			}
		})
	}
}

func TestValidateCommand_WarningCommands(t *testing.T) {
	validator := NewValidator()
	warningCommands := []struct {
		command     string
		description string
	}{
		{"mv file.txt /etc/", "moving file to system directory"},
		{"cp backup.tar /usr/", "copying file to system directory"},
		{"chown user:group file.txt", "changing file ownership"},
		{"chmod 644 file.txt", "changing file permissions"},
		{"echo 'test' > /dev/null", "writing to device file"},
	}

	for _, tc := range warningCommands {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.IsSafe {
				t.Errorf("Expected command %q (%s) to be unsafe", tc.command, tc.description)
			}

			if result.DangerLevel != types.Warning {
				t.Errorf("Expected Warning danger level for %q, got %v", tc.command, result.DangerLevel)
			}

			if !result.RequiresConfirmation {
				t.Errorf("Expected warning command %q to require confirmation", tc.command)
			}

			if len(result.Warnings) == 0 {
				t.Errorf("Expected warnings for warning command %q", tc.command)
			}
		})
	}
}

func TestIsDangerous(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command   string
		dangerous bool
	}{
		{"ls -la", false},
		{"rm -rf /", true},
		{"echo hello", false},
		{"shutdown now", true},
		{"cat file.txt", false},
		{"sudo rm file.txt", true},
		{"pwd", false},
		{"chmod 777 file.txt", true},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			result := validator.IsDangerous(tc.command)
			if result != tc.dangerous {
				t.Errorf("IsDangerous(%q) = %v, expected %v", tc.command, result, tc.dangerous)
			}
		})
	}
}

func TestGetDangerousPatterns(t *testing.T) {
	validator := NewValidator()
	patterns := validator.GetDangerousPatterns()

	if len(patterns) == 0 {
		t.Error("Expected non-empty dangerous patterns")
	}

	// Check that all patterns have required fields
	for i, pattern := range patterns {
		if pattern.Pattern == nil {
			t.Errorf("Pattern %d has nil regex", i)
		}

		if pattern.Description == "" {
			t.Errorf("Pattern %d has empty description", i)
		}

		if pattern.Level < types.Safe || pattern.Level > types.Critical {
			t.Errorf("Pattern %d has invalid danger level: %v", i, pattern.Level)
		}
	}
}

func TestValidateCommand_CaseInsensitive(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command string
		level   types.DangerLevel
	}{
		{"RM -RF /", types.Critical},
		{"Shutdown NOW", types.Critical},
		{"SUDO apt update", types.Dangerous},
		{"ChMod 777 file.txt", types.Dangerous},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.DangerLevel != tc.level {
				t.Errorf("Expected danger level %v for %q, got %v", tc.level, tc.command, result.DangerLevel)
			}
		})
	}
}

func TestValidateCommand_MultiplePatterns(t *testing.T) {
	validator := NewValidator()

	// Command that matches multiple patterns should use the highest danger level
	cmd := &types.Command{
		ID:        "test",
		Generated: "sudo rm -rf /etc/*",
		Timestamp: time.Now(),
	}

	result, err := validator.ValidateCommand(cmd)

	if err != nil {
		t.Errorf("ValidateCommand() returned error: %v", err)
	}

	// Should be Critical due to rm -rf pattern, even though sudo is only Dangerous
	if result.DangerLevel != types.Critical {
		t.Errorf("Expected Critical danger level for command matching multiple patterns, got %v", result.DangerLevel)
	}

	// Should have multiple warnings
	if len(result.Warnings) < 2 {
		t.Errorf("Expected multiple warnings for command matching multiple patterns, got %d", len(result.Warnings))
	}
}

func TestValidateCommand_UsesOriginalWhenGeneratedEmpty(t *testing.T) {
	validator := NewValidator()

	cmd := &types.Command{
		ID:        "test",
		Generated: "",
		Original:  "rm -rf /",
		Timestamp: time.Now(),
	}

	result, err := validator.ValidateCommand(cmd)

	if err != nil {
		t.Errorf("ValidateCommand() returned error: %v", err)
	}

	if result.DangerLevel != types.Critical {
		t.Errorf("Expected Critical danger level when using Original field, got %v", result.DangerLevel)
	}
}
func TestValidateCommand_EnhancedCriticalCommands(t *testing.T) {
	validator := NewValidator()
	criticalCommands := []struct {
		command     string
		description string
	}{
		{"rm -rf /bin", "deletion of critical system directory"},
		{"rm -rf /etc/", "deletion of configuration directory"},
		{"dd if=/dev/zero of=/dev/sda", "disk wiping"},
		{"dd bs=1M if=/dev/zero of=/dev/nvme0n1", "NVMe disk wiping"},
		{"shutdown now", "immediate shutdown"},
		{"poweroff +0", "immediate poweroff"},
		{"reboot now", "immediate reboot"},
		{"mkfs.ext4 /dev/sda1", "filesystem creation"},
		{"fdisk /dev/sda", "disk partitioning"},
		{"parted /dev/nvme0n1", "parted disk operations"},
		{"cfdisk /dev/sdb", "cfdisk operations"},
		{"kill 1", "killing init process"},
		{"killall init", "killing init by name"},
		{"killall systemd", "killing systemd"},
	}

	for _, tc := range criticalCommands {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.IsSafe {
				t.Errorf("Expected command %q (%s) to be unsafe", tc.command, tc.description)
			}

			if result.DangerLevel != types.Critical {
				t.Errorf("Expected Critical danger level for %q, got %v", tc.command, result.DangerLevel)
			}
		})
	}
}

func TestValidateCommand_EnhancedDangerousCommands(t *testing.T) {
	validator := NewValidator()
	dangerousCommands := []struct {
		command     string
		description string
	}{
		{"rm -rf ~/Documents/*", "wildcard deletion in home"},
		{"rm -rf $HOME", "home directory deletion via variable"},
		{"rmdir /usr", "system directory removal"},
		{"chmod 777 /etc/passwd", "dangerous permissions on system file"},
		{"chmod 766 file.txt", "world-writable permissions"},
		{"su root", "switch to root"},
		{"doas rm file.txt", "elevated privilege with doas"},
		{"pkill sshd", "killing SSH daemon"},
		{"systemctl stop ssh", "stopping SSH service"},
		{"service networking stop", "stopping network service"},
		{"ufw reset", "resetting firewall"},
		{"iptables -F", "flushing firewall rules"},
		{"find /home -name '*.txt' -delete", "find with delete"},
		{"find . -name 'temp*' -exec rm {} \\;", "find with rm execution"},
	}

	for _, tc := range dangerousCommands {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.IsSafe {
				t.Errorf("Expected command %q (%s) to be unsafe", tc.command, tc.description)
			}

			if result.DangerLevel != types.Dangerous {
				t.Errorf("Expected Dangerous danger level for %q, got %v", tc.command, result.DangerLevel)
			}
		})
	}
}

func TestValidateCommand_EnhancedWarningCommands(t *testing.T) {
	validator := NewValidator()
	warningCommands := []struct {
		command     string
		description string
	}{
		{"mv file.txt /etc/config", "moving to system directory"},
		{"cp backup.tar /usr/local/", "copying to system directory"},
		{"chown root:root file.txt", "changing ownership to root"},
		{"echo 'data' > /dev/sda", "writing to disk device"},
		{"wget http://example.com/script.sh | bash", "download and execute"},
		{"curl -s http://example.com/install | sh", "curl and execute"},
		{"mount /dev/sdb1 /mnt", "mounting filesystem"},
		{"umount /home", "unmounting filesystem"},
		{"kill 1234", "killing process by PID"},
		{"killall firefox", "killing processes by name"},
		{"pkill chrome", "killing by pattern"},
		{"crontab -r", "removing all cron jobs"},
		{"history -c", "clearing command history"},
		{"export PATH=/tmp", "modifying PATH"},
		{"unset PATH", "unsetting PATH"},
	}

	for _, tc := range warningCommands {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.IsSafe {
				t.Errorf("Expected command %q (%s) to be unsafe", tc.command, tc.description)
			}

			if result.DangerLevel != types.Warning {
				t.Errorf("Expected Warning danger level for %q, got %v", tc.command, result.DangerLevel)
			}
		})
	}
}

func TestContextAwareAnalysis_RmCommands(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command       string
		expectedLevel types.DangerLevel
		description   string
	}{
		{"rm -rf /tmp/test", types.Dangerous, "rm in temp directory should be dangerous but not critical"},
		{"rm -rf ./build", types.Dangerous, "rm in current directory should be dangerous"},
		{"rm -rf /etc/passwd", types.Critical, "rm of system file should remain critical"},
		{"rm -rf /*", types.Critical, "rm of root should remain critical"},
		{"rm -rf $HOME", types.Dangerous, "rm of home via variable should be dangerous"},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.DangerLevel != tc.expectedLevel {
				t.Errorf("Expected %v danger level for %q (%s), got %v",
					tc.expectedLevel, tc.command, tc.description, result.DangerLevel)
			}
		})
	}
}

func TestContextAwareAnalysis_DdCommands(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command       string
		expectedLevel types.DangerLevel
		description   string
	}{
		{"dd if=/dev/zero of=/dev/sda", types.Critical, "dd to physical device should be critical"},
		{"dd if=/dev/zero of=/dev/nvme0n1", types.Critical, "dd to NVMe should be critical"},
		{"dd if=file.img of=/tmp/output", types.Safe, "dd to regular file should be safe"},
		{"dd if=/dev/zero of=test.img bs=1M count=100", types.Safe, "dd creating image file should be safe"},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.DangerLevel != tc.expectedLevel {
				t.Errorf("Expected %v danger level for %q (%s), got %v",
					tc.expectedLevel, tc.command, tc.description, result.DangerLevel)
			}
		})
	}
}

func TestContextAwareAnalysis_ChmodCommands(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command       string
		expectedLevel types.DangerLevel
		description   string
	}{
		{"chmod 644 file.txt", types.Warning, "normal chmod should be warning"},
		{"chmod 777 /etc/passwd", types.Dangerous, "chmod on system file should be dangerous"},
		{"chmod 755 /usr/bin/myapp", types.Dangerous, "chmod on system directory should be dangerous"},
		{"chmod +x script.sh", types.Warning, "chmod +x should be warning"},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.DangerLevel != tc.expectedLevel {
				t.Errorf("Expected %v danger level for %q (%s), got %v",
					tc.expectedLevel, tc.command, tc.description, result.DangerLevel)
			}
		})
	}
}

func TestContextAwareAnalysis_NetworkCommands(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command       string
		expectedLevel types.DangerLevel
		description   string
	}{
		{"systemctl stop ssh", types.Dangerous, "stopping SSH should be dangerous"},
		{"service networking stop", types.Dangerous, "stopping networking should be dangerous"},
		{"ufw disable", types.Dangerous, "disabling firewall should be dangerous"},
		{"iptables -F", types.Dangerous, "flushing iptables should be dangerous"},
		{"ifconfig eth0 down", types.Dangerous, "bringing interface down should be dangerous"},
		{"ip link set eth0 down", types.Dangerous, "ip link down should be dangerous"},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.DangerLevel != tc.expectedLevel {
				t.Errorf("Expected %v danger level for %q (%s), got %v",
					tc.expectedLevel, tc.command, tc.description, result.DangerLevel)
			}
		})
	}
}

func TestValidateCommand_ProtectedOperations(t *testing.T) {
	validator := NewValidator()

	// Test all protected operations mentioned in requirements 4.5
	protectedCommands := []string{
		"rm -rf /",
		"rmdir /usr",
		"dd if=/dev/zero of=/dev/sda",
		"mkfs.ext4 /dev/sda1",
		"shutdown now",
		"reboot",
		"halt",
	}

	for _, cmdStr := range protectedCommands {
		t.Run(cmdStr, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: cmdStr,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", cmdStr, err)
			}

			if result.IsSafe {
				t.Errorf("Expected protected command %q to be unsafe", cmdStr)
			}

			if !result.RequiresConfirmation {
				t.Errorf("Expected protected command %q to require confirmation", cmdStr)
			}

			if result.DangerLevel < types.Dangerous {
				t.Errorf("Expected protected command %q to have at least Dangerous level, got %v",
					cmdStr, result.DangerLevel)
			}
		})
	}
}

func TestValidateCommand_UserConfirmationRequirements(t *testing.T) {
	validator := NewValidator()

	testCases := []struct {
		command              string
		requiresConfirmation bool
		description          string
	}{
		{"ls -la", false, "safe command should not require confirmation"},
		{"rm file.txt", true, "dangerous command should require confirmation"},
		{"sudo apt update", true, "elevated privilege should require confirmation"},
		{"chmod 777 file.txt", true, "dangerous permissions should require confirmation"},
		{"echo hello", false, "safe command should not require confirmation"},
		{"shutdown now", true, "system control should require confirmation"},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := &types.Command{
				ID:        "test",
				Generated: tc.command,
				Timestamp: time.Now(),
			}

			result, err := validator.ValidateCommand(cmd)

			if err != nil {
				t.Errorf("ValidateCommand(%q) returned error: %v", tc.command, err)
			}

			if result.RequiresConfirmation != tc.requiresConfirmation {
				t.Errorf("Expected RequiresConfirmation=%v for %q (%s), got %v",
					tc.requiresConfirmation, tc.command, tc.description, result.RequiresConfirmation)
			}
		})
	}
}

func TestValidateCommandWithOptions_NoOptions(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "rm file.txt",
		Timestamp: time.Now(),
	}

	// Test with nil options
	result, err := validator.ValidateCommandWithOptions(cmd, nil)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	// Should behave like normal validation
	if result.Bypassed {
		t.Error("Expected command not to be bypassed when options are nil")
	}

	if result.AuditEntry != nil {
		t.Error("Expected no audit entry when options are nil")
	}
}

func TestValidateCommandWithOptions_BypassWarningLevel(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "rm file.txt", // Warning level command
		Timestamp: time.Now(),
	}

	auditLogger := NewNoOpAuditLogger()
	opts := &types.ValidationOptions{
		SkipConfirmation: true,
		BypassLevel:      types.Warning,
		AuditLogger:      auditLogger,
		UserID:           "test-user",
		Reason:           "Test bypass",
	}

	result, err := validator.ValidateCommandWithOptions(cmd, opts)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	if !result.Bypassed {
		t.Error("Expected command to be bypassed for Warning level")
	}

	if result.RequiresConfirmation {
		t.Error("Expected bypassed command to not require confirmation")
	}

	if result.AuditEntry == nil {
		t.Error("Expected audit entry to be created")
	} else {
		if result.AuditEntry.Action != types.AuditActionBypassed {
			t.Errorf("Expected audit action to be Bypassed, got %v", result.AuditEntry.Action)
		}
		if result.AuditEntry.UserID != "test-user" {
			t.Errorf("Expected user ID to be 'test-user', got %q", result.AuditEntry.UserID)
		}
	}

	// Check for bypass warning
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "bypassed") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected bypass warning in result warnings")
	}
}

func TestValidateCommandWithOptions_BypassDangerousLevel(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "sudo rm -rf /tmp/*", // Dangerous level command
		Timestamp: time.Now(),
	}

	auditLogger := NewNoOpAuditLogger()
	opts := &types.ValidationOptions{
		SkipConfirmation: true,
		BypassLevel:      types.Dangerous,
		AuditLogger:      auditLogger,
		UserID:           "admin-user",
		Reason:           "Maintenance task",
	}

	result, err := validator.ValidateCommandWithOptions(cmd, opts)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	if !result.Bypassed {
		t.Error("Expected dangerous command to be bypassed when bypass level is Dangerous")
	}

	if result.RequiresConfirmation {
		t.Error("Expected bypassed dangerous command to not require confirmation")
	}
}

func TestValidateCommandWithOptions_NoBypassCriticalLevel(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "rm -rf /", // Critical level command
		Timestamp: time.Now(),
	}

	auditLogger := NewNoOpAuditLogger()
	opts := &types.ValidationOptions{
		SkipConfirmation: true,
		BypassLevel:      types.Warning, // Lower than command's danger level
		AuditLogger:      auditLogger,
		UserID:           "test-user",
		Reason:           "Test bypass",
	}

	result, err := validator.ValidateCommandWithOptions(cmd, opts)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	if result.Bypassed {
		t.Error("Expected critical command not to be bypassed when bypass level is Warning")
	}

	if !result.RequiresConfirmation {
		t.Error("Expected critical command to still require confirmation")
	}

	if result.AuditEntry == nil {
		t.Error("Expected audit entry to be created")
	} else {
		if result.AuditEntry.Action != types.AuditActionValidated {
			t.Errorf("Expected audit action to be Validated, got %v", result.AuditEntry.Action)
		}
	}
}

func TestValidateCommandWithOptions_SafeCommandNoBypass(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "ls -la", // Safe command
		Timestamp: time.Now(),
	}

	auditLogger := NewNoOpAuditLogger()
	opts := &types.ValidationOptions{
		SkipConfirmation: true,
		BypassLevel:      types.Warning,
		AuditLogger:      auditLogger,
		UserID:           "test-user",
		Reason:           "Test bypass",
	}

	result, err := validator.ValidateCommandWithOptions(cmd, opts)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	// Safe commands should not be "bypassed" since they don't need confirmation anyway
	if result.Bypassed {
		t.Error("Expected safe command not to be marked as bypassed")
	}

	if result.RequiresConfirmation {
		t.Error("Expected safe command to not require confirmation")
	}

	if result.AuditEntry == nil {
		t.Error("Expected audit entry to be created")
	} else {
		if result.AuditEntry.Action != types.AuditActionValidated {
			t.Errorf("Expected audit action to be Validated, got %v", result.AuditEntry.Action)
		}
	}
}

func TestValidateCommandWithOptions_NoSkipConfirmation(t *testing.T) {
	validator := NewValidator()
	cmd := &types.Command{
		ID:        "test",
		Generated: "rm file.txt", // Warning level command
		Timestamp: time.Now(),
	}

	auditLogger := NewNoOpAuditLogger()
	opts := &types.ValidationOptions{
		SkipConfirmation: false, // Don't skip confirmation
		BypassLevel:      types.Warning,
		AuditLogger:      auditLogger,
		UserID:           "test-user",
		Reason:           "Test no bypass",
	}

	result, err := validator.ValidateCommandWithOptions(cmd, opts)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	if result.Bypassed {
		t.Error("Expected command not to be bypassed when SkipConfirmation is false")
	}

	if !result.RequiresConfirmation {
		t.Error("Expected command to still require confirmation")
	}
}

func TestValidateCommandWithOptions_NilCommand(t *testing.T) {
	validator := NewValidator()

	auditLogger := NewNoOpAuditLogger()
	opts := &types.ValidationOptions{
		SkipConfirmation: true,
		BypassLevel:      types.Critical,
		AuditLogger:      auditLogger,
		UserID:           "test-user",
		Reason:           "Test bypass",
	}

	result, err := validator.ValidateCommandWithOptions(nil, opts)
	if err != nil {
		t.Errorf("ValidateCommandWithOptions() returned error: %v", err)
	}

	// Nil command should still be critical and not bypassed
	if result.Bypassed {
		t.Error("Expected nil command not to be bypassed")
	}

	if !result.RequiresConfirmation {
		t.Error("Expected nil command to require confirmation")
	}

	if result.DangerLevel != types.Critical {
		t.Errorf("Expected Critical danger level for nil command, got %v", result.DangerLevel)
	}
}

// Test audit logging functionality
func TestAuditLogger(t *testing.T) {
	// Create temporary file for testing
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	auditLogger, err := NewFileAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}

	// Create test audit entries
	entry1 := &types.AuditEntry{
		Timestamp:   time.Now(),
		Command:     "rm file.txt",
		UserID:      "user1",
		Action:      types.AuditActionBypassed,
		DangerLevel: types.Warning,
		Reason:      "Test bypass",
		SessionID:   "session1",
	}

	entry2 := &types.AuditEntry{
		Timestamp:   time.Now().Add(time.Minute),
		Command:     "ls -la",
		UserID:      "user2",
		Action:      types.AuditActionValidated,
		DangerLevel: types.Safe,
		Reason:      "",
		SessionID:   "session2",
	}

	// Log entries
	if err := auditLogger.LogAuditEvent(entry1); err != nil {
		t.Errorf("Failed to log audit entry 1: %v", err)
	}

	if err := auditLogger.LogAuditEvent(entry2); err != nil {
		t.Errorf("Failed to log audit entry 2: %v", err)
	}

	// Retrieve all entries
	entries, err := auditLogger.GetAuditLog(nil)
	if err != nil {
		t.Errorf("Failed to get audit log: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 audit entries, got %d", len(entries))
	}

	// Test filtering by user
	userFilter := &types.AuditFilter{
		UserID: "user1",
	}

	filteredEntries, err := auditLogger.GetAuditLog(userFilter)
	if err != nil {
		t.Errorf("Failed to get filtered audit log: %v", err)
	}

	if len(filteredEntries) != 1 {
		t.Errorf("Expected 1 filtered audit entry, got %d", len(filteredEntries))
	}

	if filteredEntries[0].UserID != "user1" {
		t.Errorf("Expected filtered entry to be for user1, got %s", filteredEntries[0].UserID)
	}

	// Test filtering by action
	actionFilter := &types.AuditFilter{
		Action: &[]types.AuditAction{types.AuditActionBypassed}[0],
	}

	actionFilteredEntries, err := auditLogger.GetAuditLog(actionFilter)
	if err != nil {
		t.Errorf("Failed to get action-filtered audit log: %v", err)
	}

	if len(actionFilteredEntries) != 1 {
		t.Errorf("Expected 1 action-filtered audit entry, got %d", len(actionFilteredEntries))
	}

	if actionFilteredEntries[0].Action != types.AuditActionBypassed {
		t.Errorf("Expected filtered entry to be bypassed action, got %v", actionFilteredEntries[0].Action)
	}
}

func TestNoOpAuditLogger(t *testing.T) {
	logger := NewNoOpAuditLogger()

	entry := &types.AuditEntry{
		Timestamp:   time.Now(),
		Command:     "test command",
		UserID:      "test-user",
		Action:      types.AuditActionValidated,
		DangerLevel: types.Safe,
	}

	// Should not return error
	if err := logger.LogAuditEvent(entry); err != nil {
		t.Errorf("NoOpAuditLogger.LogAuditEvent() returned error: %v", err)
	}

	// Should return empty slice
	entries, err := logger.GetAuditLog(nil)
	if err != nil {
		t.Errorf("NoOpAuditLogger.GetAuditLog() returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected NoOpAuditLogger to return empty slice, got %d entries", len(entries))
	}
}
