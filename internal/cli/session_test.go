package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestNewSessionState(t *testing.T) {
	// Reset global flags
	resetGlobalFlags()

	session, err := NewSessionState()
	if err != nil {
		// This might fail due to missing configuration, which is expected in tests
		if !strings.Contains(err.Error(), "provider") && !strings.Contains(err.Error(), "config") {
			t.Errorf("unexpected error: %v", err)
		}
		return
	}

	if session == nil {
		t.Error("expected session to be created")
		return
	}

	// Check session properties
	sessionID, startTime, history := session.GetSessionState()

	if sessionID == "" {
		t.Error("expected non-empty session ID")
	}

	if startTime.IsZero() {
		t.Error("expected valid start time")
	}

	if len(history) != 0 {
		t.Error("expected empty history for new session")
	}

	if !strings.HasPrefix(sessionID, "session_") {
		t.Errorf("expected session ID to start with 'session_', got %s", sessionID)
	}
}

func TestSessionState_HandleSpecialCommand(t *testing.T) {
	session := &SessionState{
		sessionID:      "test_session",
		startTime:      time.Now(),
		commandHistory: []string{"test command 1", "test command 2"},
		config: &types.Config{
			DefaultProvider: "test",
			UserPreferences: types.UserPreferences{
				MaxFileListSize: 100,
				DefaultTimeout:  30 * time.Second,
				EnablePlugins:   true,
				AutoUpdate:      false,
			},
		},
	}

	tests := []struct {
		name         string
		input        string
		expectHandle bool
		expectExit   bool
	}{
		{
			name:         "help command",
			input:        "help",
			expectHandle: true,
			expectExit:   false,
		},
		{
			name:         "history command",
			input:        "history",
			expectHandle: true,
			expectExit:   false,
		},
		{
			name:         "clear command",
			input:        "clear",
			expectHandle: true,
			expectExit:   false,
		},
		{
			name:         "config command",
			input:        "config",
			expectHandle: true,
			expectExit:   false,
		},
		{
			name:         "exit command",
			input:        "exit",
			expectHandle: true,
			expectExit:   true,
		},
		{
			name:         "quit command",
			input:        "quit",
			expectHandle: true,
			expectExit:   true,
		},
		{
			name:         "q command",
			input:        "q",
			expectHandle: true,
			expectExit:   true,
		},
		{
			name:         "case insensitive help",
			input:        "HELP",
			expectHandle: true,
			expectExit:   false,
		},
		{
			name:         "case insensitive exit",
			input:        "EXIT",
			expectHandle: true,
			expectExit:   true,
		},
		{
			name:         "regular command",
			input:        "list files",
			expectHandle: false,
			expectExit:   false,
		},
		{
			name:         "empty command",
			input:        "",
			expectHandle: false,
			expectExit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled, shouldExit := session.handleSpecialCommand(tt.input)

			if handled != tt.expectHandle {
				t.Errorf("expected handled=%v, got %v", tt.expectHandle, handled)
			}

			if shouldExit != tt.expectExit {
				t.Errorf("expected shouldExit=%v, got %v", tt.expectExit, shouldExit)
			}
		})
	}
}

func TestSessionState_ClearHistory(t *testing.T) {
	session := &SessionState{
		commandHistory: []string{"command1", "command2", "command3"},
	}

	// Verify history has items
	if len(session.commandHistory) != 3 {
		t.Errorf("expected 3 items in history, got %d", len(session.commandHistory))
	}

	// Clear history
	session.clearHistory()

	// Verify history is empty
	if len(session.commandHistory) != 0 {
		t.Errorf("expected empty history after clear, got %d items", len(session.commandHistory))
	}
}

func TestSessionState_GetSessionState(t *testing.T) {
	expectedID := "test_session_123"
	expectedTime := time.Now()
	expectedHistory := []string{"cmd1", "cmd2"}

	session := &SessionState{
		sessionID:      expectedID,
		startTime:      expectedTime,
		commandHistory: expectedHistory,
	}

	id, startTime, history := session.GetSessionState()

	if id != expectedID {
		t.Errorf("expected session ID %s, got %s", expectedID, id)
	}

	if !startTime.Equal(expectedTime) {
		t.Errorf("expected start time %v, got %v", expectedTime, startTime)
	}

	if len(history) != len(expectedHistory) {
		t.Errorf("expected history length %d, got %d", len(expectedHistory), len(history))
	}

	for i, cmd := range expectedHistory {
		if history[i] != cmd {
			t.Errorf("expected history[%d] = %s, got %s", i, cmd, history[i])
		}
	}
}

func TestSessionCommands(t *testing.T) {
	// Test that session commands are properly defined
	if sessionCmd == nil {
		t.Error("sessionCmd should not be nil")
	}

	if sessionCmd.Use != "session" {
		t.Errorf("expected session command use to be 'session', got %s", sessionCmd.Use)
	}

	if sessionCmd.Short == "" {
		t.Error("session command should have a short description")
	}

	if sessionCmd.Long == "" {
		t.Error("session command should have a long description")
	}

	if sessionCmd.RunE == nil {
		t.Error("session command should have a RunE function")
	}
}

func TestSessionCommandExecution(t *testing.T) {
	// Test that the session command can be executed (though it will likely fail due to missing config)
	// This tests the command structure and basic error handling

	resetGlobalFlags()

	// Create a test command
	cmd := sessionCmd
	cmd.SetArgs([]string{})

	// We expect this to fail due to missing configuration in test environment
	// but it should fail gracefully
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		// If it doesn't fail, that's actually fine - it means the session started successfully
		return
	}

	// Check that the error is related to configuration or provider setup
	if !strings.Contains(err.Error(), "provider") &&
		!strings.Contains(err.Error(), "config") &&
		!strings.Contains(err.Error(), "session") {
		t.Errorf("expected configuration/provider/session error, got: %v", err)
	}
}

// Test helper functions
func TestSessionStateCreationWithFlags(t *testing.T) {
	// Test session creation with different flag combinations
	tests := []struct {
		name        string
		setupFlags  func()
		expectError bool
	}{
		{
			name: "default flags",
			setupFlags: func() {
				resetGlobalFlags()
			},
			expectError: false, // NewSessionState creates default config when loading fails
		},
		{
			name: "with provider flag",
			setupFlags: func() {
				resetGlobalFlags()
				provider = "openai"
			},
			expectError: false, // NewSessionState creates default config when loading fails
		},
		{
			name: "with verbose flag",
			setupFlags: func() {
				resetGlobalFlags()
				verbose = true
			},
			expectError: false, // NewSessionState creates default config when loading fails
		},
		{
			name: "with dry-run flag",
			setupFlags: func() {
				resetGlobalFlags()
				dryRun = true
			},
			expectError: false, // NewSessionState creates default config when loading fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFlags()

			session, err := NewSessionState()

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else {
					// In test environment, we expect configuration-related errors
					if !strings.Contains(err.Error(), "provider") && !strings.Contains(err.Error(), "config") {
						t.Errorf("expected configuration error, got: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if session == nil {
					t.Error("expected session to be created")
				}
			}
		})
	}
}
