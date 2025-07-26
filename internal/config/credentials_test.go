package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCredentialManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewCredentialManager(tempDir)
	if manager == nil {
		t.Fatal("NewCredentialManager() returned nil")
	}

	if manager.store == nil {
		t.Fatal("CredentialManager store is nil")
	}

	if manager.fallbackDir != tempDir {
		t.Errorf("Expected fallbackDir %s, got %s", tempDir, manager.fallbackDir)
	}
}

func TestFileCredentialStore_StoreAndRetrieve(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := newFileCredentialStore(tempDir)

	// Test storing a credential
	err = store.Store("openai", "api_key", "test-secret-key")
	if err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// Verify credentials file was created
	credentialsPath := filepath.Join(tempDir, credentialsFileName)
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Fatal("Credentials file was not created")
	}

	// Test retrieving the credential
	secret, err := store.Retrieve("openai", "api_key")
	if err != nil {
		t.Fatalf("Retrieve() failed: %v", err)
	}

	if secret != "test-secret-key" {
		t.Errorf("Expected secret 'test-secret-key', got '%s'", secret)
	}

	// Test retrieving non-existent credential
	_, err = store.Retrieve("nonexistent", "api_key")
	if err == nil {
		t.Error("Expected error when retrieving non-existent credential")
	}
}

func TestFileCredentialStore_MultipleCredentials(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := newFileCredentialStore(tempDir)

	// Store multiple credentials
	credentials := map[string]string{
		"openai:api_key":    "openai-secret",
		"anthropic:api_key": "anthropic-secret",
		"openai:org_id":     "openai-org-123",
		"anthropic:version": "2023-06-01",
	}

	for key, secret := range credentials {
		parts := splitKey(key)
		if len(parts) != 2 {
			t.Fatalf("Invalid test key format: %s", key)
		}
		err = store.Store(parts[0], parts[1], secret)
		if err != nil {
			t.Fatalf("Store() failed for %s: %v", key, err)
		}
	}

	// Retrieve and verify all credentials
	for key, expectedSecret := range credentials {
		parts := splitKey(key)
		secret, err := store.Retrieve(parts[0], parts[1])
		if err != nil {
			t.Fatalf("Retrieve() failed for %s: %v", key, err)
		}
		if secret != expectedSecret {
			t.Errorf("Expected secret '%s' for %s, got '%s'", expectedSecret, key, secret)
		}
	}
}

func TestFileCredentialStore_Delete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := newFileCredentialStore(tempDir)

	// Store credentials
	err = store.Store("openai", "api_key", "test-secret")
	if err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	err = store.Store("anthropic", "api_key", "another-secret")
	if err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// Verify both credentials exist
	secret1, err := store.Retrieve("openai", "api_key")
	if err != nil {
		t.Fatalf("Retrieve() failed: %v", err)
	}
	if secret1 != "test-secret" {
		t.Errorf("Expected 'test-secret', got '%s'", secret1)
	}

	secret2, err := store.Retrieve("anthropic", "api_key")
	if err != nil {
		t.Fatalf("Retrieve() failed: %v", err)
	}
	if secret2 != "another-secret" {
		t.Errorf("Expected 'another-secret', got '%s'", secret2)
	}

	// Delete one credential
	err = store.Delete("openai", "api_key")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify deleted credential is gone
	_, err = store.Retrieve("openai", "api_key")
	if err == nil {
		t.Error("Expected error when retrieving deleted credential")
	}

	// Verify other credential still exists
	secret2, err = store.Retrieve("anthropic", "api_key")
	if err != nil {
		t.Fatalf("Retrieve() failed for remaining credential: %v", err)
	}
	if secret2 != "another-secret" {
		t.Errorf("Expected 'another-secret', got '%s'", secret2)
	}
}

func TestFileCredentialStore_List(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := newFileCredentialStore(tempDir)

	// Store credentials for different services
	testData := []struct {
		service string
		account string
		secret  string
	}{
		{"openai", "api_key", "openai-secret"},
		{"openai", "org_id", "openai-org"},
		{"anthropic", "api_key", "anthropic-secret"},
		{"anthropic", "version", "2023-06-01"},
		{"google", "api_key", "google-secret"},
	}

	for _, data := range testData {
		err = store.Store(data.service, data.account, data.secret)
		if err != nil {
			t.Fatalf("Store() failed: %v", err)
		}
	}

	// Test listing accounts for OpenAI
	openaiAccounts, err := store.List("openai")
	if err != nil {
		t.Fatalf("List() failed for openai: %v", err)
	}

	expectedOpenAI := []string{"api_key", "org_id"}
	if len(openaiAccounts) != len(expectedOpenAI) {
		t.Errorf("Expected %d OpenAI accounts, got %d", len(expectedOpenAI), len(openaiAccounts))
	}

	for _, expected := range expectedOpenAI {
		found := false
		for _, account := range openaiAccounts {
			if account == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected OpenAI account '%s' not found in list", expected)
		}
	}

	// Test listing accounts for Anthropic
	anthropicAccounts, err := store.List("anthropic")
	if err != nil {
		t.Fatalf("List() failed for anthropic: %v", err)
	}

	expectedAnthropic := []string{"api_key", "version"}
	if len(anthropicAccounts) != len(expectedAnthropic) {
		t.Errorf("Expected %d Anthropic accounts, got %d", len(expectedAnthropic), len(anthropicAccounts))
	}

	// Test listing accounts for non-existent service
	nonExistentAccounts, err := store.List("nonexistent")
	if err != nil {
		t.Fatalf("List() failed for nonexistent service: %v", err)
	}

	if len(nonExistentAccounts) != 0 {
		t.Errorf("Expected 0 accounts for nonexistent service, got %d", len(nonExistentAccounts))
	}
}

func TestFileCredentialStore_EmptyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := newFileCredentialStore(tempDir)

	// Test loading from non-existent file
	credentials, err := store.loadCredentials()
	if err != nil {
		t.Fatalf("loadCredentials() failed on non-existent file: %v", err)
	}

	if len(credentials) != 0 {
		t.Errorf("Expected empty credentials map, got %d entries", len(credentials))
	}

	// Test retrieving from empty store
	_, err = store.Retrieve("openai", "api_key")
	if err == nil {
		t.Error("Expected error when retrieving from empty store")
	}

	// Test listing from empty store
	accounts, err := store.List("openai")
	if err != nil {
		t.Fatalf("List() failed on empty store: %v", err)
	}

	if len(accounts) != 0 {
		t.Errorf("Expected empty accounts list, got %d entries", len(accounts))
	}
}

func TestFileCredentialStore_Persistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create first store and add credentials
	store1 := newFileCredentialStore(tempDir)
	err = store1.Store("openai", "api_key", "persistent-secret")
	if err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// Create second store (simulating app restart)
	store2 := newFileCredentialStore(tempDir)
	secret, err := store2.Retrieve("openai", "api_key")
	if err != nil {
		t.Fatalf("Retrieve() failed after restart: %v", err)
	}

	if secret != "persistent-secret" {
		t.Errorf("Expected 'persistent-secret', got '%s'", secret)
	}
}

func TestGetCredentialFromEnv(t *testing.T) {
	// Set up test environment variables
	testEnvVars := map[string]string{
		"OPENAI_API_KEY":         "env-openai-key",
		"ANTHROPIC_DEFAULT_KEY":  "env-anthropic-key",
		"GOOGLE_TOKEN":           "env-google-token",
		"CUSTOM_SERVICE_API_KEY": "env-custom-key",
	}

	// Set environment variables
	for key, value := range testEnvVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	// Test cases
	testCases := []struct {
		service  string
		account  string
		expected string
	}{
		{"openai", "default", "env-openai-key"},
		{"anthropic", "default", "env-anthropic-key"},
		{"google", "default", "env-google-token"},
		{"custom_service", "default", "env-custom-key"},
		{"nonexistent", "default", ""},
	}

	for _, tc := range testCases {
		result := GetCredentialFromEnv(tc.service, tc.account)
		if result != tc.expected {
			t.Errorf("GetCredentialFromEnv(%s, %s): expected '%s', got '%s'",
				tc.service, tc.account, tc.expected, result)
		}
	}
}

func TestGenerateMasterKey(t *testing.T) {
	key1 := generateMasterKey()
	key2 := generateMasterKey()

	// Keys should be consistent across calls
	if len(key1) != len(key2) {
		t.Errorf("Master key length inconsistent: %d vs %d", len(key1), len(key2))
	}

	for i := range key1 {
		if key1[i] != key2[i] {
			t.Error("Master key should be consistent across calls")
			break
		}
	}

	// Key should be the expected length
	if len(key1) != 32 {
		t.Errorf("Expected master key length 32, got %d", len(key1))
	}

	// Key should not be all zeros
	allZeros := true
	for _, b := range key1 {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("Master key should not be all zeros")
	}
}

func TestCredentialManager_Integration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nl-to-shell-cred-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewCredentialManager(tempDir)

	// Test storing and retrieving through manager
	err = manager.Store("openai", "api_key", "manager-test-secret")
	if err != nil {
		t.Fatalf("Manager Store() failed: %v", err)
	}

	secret, err := manager.Retrieve("openai", "api_key")
	if err != nil {
		t.Fatalf("Manager Retrieve() failed: %v", err)
	}

	if secret != "manager-test-secret" {
		t.Errorf("Expected 'manager-test-secret', got '%s'", secret)
	}

	// Test listing through manager
	accounts, err := manager.List("openai")
	if err != nil {
		t.Fatalf("Manager List() failed: %v", err)
	}

	if len(accounts) != 1 || accounts[0] != "api_key" {
		t.Errorf("Expected ['api_key'], got %v", accounts)
	}

	// Test deleting through manager
	err = manager.Delete("openai", "api_key")
	if err != nil {
		t.Fatalf("Manager Delete() failed: %v", err)
	}

	_, err = manager.Retrieve("openai", "api_key")
	if err == nil {
		t.Error("Expected error after deleting credential")
	}
}

// Helper function to split key for testing
func splitKey(key string) []string {
	for i, char := range key {
		if char == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key}
}
