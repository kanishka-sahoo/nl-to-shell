package safety

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// FileAuditLogger implements AuditLogger interface using file-based storage
type FileAuditLogger struct {
	logPath string
	mutex   sync.Mutex
}

// NewFileAuditLogger creates a new file-based audit logger
func NewFileAuditLogger(logPath string) (*FileAuditLogger, error) {
	// Ensure the directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	return &FileAuditLogger{
		logPath: logPath,
	}, nil
}

// LogAuditEvent logs an audit event to the file
func (f *FileAuditLogger) LogAuditEvent(entry *types.AuditEntry) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Open file for appending
	file, err := os.OpenFile(f.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Write JSON line
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	return nil
}

// GetAuditLog retrieves audit log entries based on filter criteria
func (f *FileAuditLogger) GetAuditLog(filter *types.AuditFilter) ([]*types.AuditEntry, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	file, err := os.Open(f.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.AuditEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	var entries []*types.AuditEntry
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var entry types.AuditEntry
		if err := decoder.Decode(&entry); err != nil {
			// Skip malformed entries but continue processing
			continue
		}

		// Apply filters
		if f.matchesFilter(&entry, filter) {
			entries = append(entries, &entry)
		}
	}

	return entries, nil
}

// matchesFilter checks if an audit entry matches the given filter criteria
func (f *FileAuditLogger) matchesFilter(entry *types.AuditEntry, filter *types.AuditFilter) bool {
	if filter == nil {
		return true
	}

	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}

	if filter.UserID != "" && entry.UserID != filter.UserID {
		return false
	}

	if filter.Action != nil && entry.Action != *filter.Action {
		return false
	}

	if filter.DangerLevel != nil && entry.DangerLevel != *filter.DangerLevel {
		return false
	}

	return true
}

// NoOpAuditLogger implements AuditLogger interface as a no-op for testing
type NoOpAuditLogger struct{}

// NewNoOpAuditLogger creates a new no-op audit logger
func NewNoOpAuditLogger() *NoOpAuditLogger {
	return &NoOpAuditLogger{}
}

// LogAuditEvent does nothing
func (n *NoOpAuditLogger) LogAuditEvent(entry *types.AuditEntry) error {
	return nil
}

// GetAuditLog returns empty slice
func (n *NoOpAuditLogger) GetAuditLog(filter *types.AuditFilter) ([]*types.AuditEntry, error) {
	return []*types.AuditEntry{}, nil
}
