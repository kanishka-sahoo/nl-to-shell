package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	credentialsFileName = "credentials.enc"
	saltSize            = 32
	keySize             = 32
	nonceSize           = 12
	iterations          = 100000
)

// CredentialStore defines the interface for credential storage
type CredentialStore interface {
	Store(service, account, secret string) error
	Retrieve(service, account string) (string, error)
	Delete(service, account string) error
	List(service string) ([]string, error)
}

// credentialManager manages secure credential storage
type credentialManager struct {
	store       CredentialStore
	fallbackDir string
}

// NewCredentialManager creates a new credential manager
func NewCredentialManager(configDir string) *credentialManager {
	var store CredentialStore

	// Try to use system keychain first
	systemStore := newSystemCredentialStore()
	if systemStore != nil {
		// Test if system store is actually working
		testErr := systemStore.Store("nl-to-shell-test", "test", "test")
		if testErr == nil {
			// Clean up test credential
			systemStore.Delete("nl-to-shell-test", "test")
			store = systemStore
		}
	}

	// Fallback to encrypted file storage if system store is not available or not working
	if store == nil {
		store = newFileCredentialStore(configDir)
	}

	return &credentialManager{
		store:       store,
		fallbackDir: configDir,
	}
}

// Store stores a credential securely
func (cm *credentialManager) Store(service, account, secret string) error {
	return cm.store.Store(service, account, secret)
}

// Retrieve retrieves a credential securely
func (cm *credentialManager) Retrieve(service, account string) (string, error) {
	return cm.store.Retrieve(service, account)
}

// Delete deletes a credential
func (cm *credentialManager) Delete(service, account string) error {
	return cm.store.Delete(service, account)
}

// List lists all accounts for a service
func (cm *credentialManager) List(service string) ([]string, error) {
	return cm.store.List(service)
}

// fileCredentialStore implements credential storage using encrypted files
type fileCredentialStore struct {
	credentialsPath string
	masterKey       []byte
}

// credentialData represents the structure of stored credentials
type credentialData struct {
	Service string `json:"service"`
	Account string `json:"account"`
	Secret  string `json:"secret"`
}

// encryptedFile represents the encrypted credentials file structure
type encryptedFile struct {
	Salt      string `json:"salt"`
	Nonce     string `json:"nonce"`
	Encrypted string `json:"encrypted"`
}

// newFileCredentialStore creates a new file-based credential store
func newFileCredentialStore(configDir string) *fileCredentialStore {
	credentialsPath := filepath.Join(configDir, credentialsFileName)

	// Generate or load master key
	masterKey := generateMasterKey()

	return &fileCredentialStore{
		credentialsPath: credentialsPath,
		masterKey:       masterKey,
	}
}

// Store stores a credential in encrypted file
func (fs *fileCredentialStore) Store(service, account, secret string) error {
	credentials, err := fs.loadCredentials()
	if err != nil {
		credentials = make(map[string]credentialData)
	}

	key := fmt.Sprintf("%s:%s", service, account)
	credentials[key] = credentialData{
		Service: service,
		Account: account,
		Secret:  secret,
	}

	return fs.saveCredentials(credentials)
}

// Retrieve retrieves a credential from encrypted file
func (fs *fileCredentialStore) Retrieve(service, account string) (string, error) {
	credentials, err := fs.loadCredentials()
	if err != nil {
		return "", fmt.Errorf("failed to load credentials: %w", err)
	}

	key := fmt.Sprintf("%s:%s", service, account)
	if cred, exists := credentials[key]; exists {
		return cred.Secret, nil
	}

	return "", fmt.Errorf("credential not found for service %s, account %s", service, account)
}

// Delete deletes a credential from encrypted file
func (fs *fileCredentialStore) Delete(service, account string) error {
	credentials, err := fs.loadCredentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	key := fmt.Sprintf("%s:%s", service, account)
	delete(credentials, key)

	return fs.saveCredentials(credentials)
}

// List lists all accounts for a service
func (fs *fileCredentialStore) List(service string) ([]string, error) {
	credentials, err := fs.loadCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	var accounts []string
	for _, cred := range credentials {
		if cred.Service == service {
			accounts = append(accounts, cred.Account)
		}
	}

	return accounts, nil
}

// loadCredentials loads and decrypts credentials from file
func (fs *fileCredentialStore) loadCredentials() (map[string]credentialData, error) {
	// Check if credentials file exists
	if _, err := os.Stat(fs.credentialsPath); os.IsNotExist(err) {
		return make(map[string]credentialData), nil
	}

	// Read encrypted file
	data, err := os.ReadFile(fs.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse encrypted structure
	var encrypted encryptedFile
	if err := json.Unmarshal(data, &encrypted); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	// Decode salt and nonce
	salt, err := base64.StdEncoding.DecodeString(encrypted.Salt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Decode encrypted data
	encryptedBytes, err := base64.StdEncoding.DecodeString(encrypted.Encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	// Derive key from master key and salt
	key := pbkdf2.Key(fs.masterKey, salt, iterations, keySize, sha256.New)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	decryptedData, err := aesGCM.Open(nil, nonce, encryptedBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Parse decrypted credentials
	var credentials map[string]credentialData
	if err := json.Unmarshal(decryptedData, &credentials); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted credentials: %w", err)
	}

	return credentials, nil
}

// saveCredentials encrypts and saves credentials to file
func (fs *fileCredentialStore) saveCredentials(credentials map[string]credentialData) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fs.credentialsPath), 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Generate salt and nonce
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Derive key from master key and salt
	key := pbkdf2.Key(fs.masterKey, salt, iterations, keySize, sha256.New)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Marshal credentials to JSON
	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Encrypt credentials
	encryptedData := aesGCM.Seal(nil, nonce, credentialsJSON, nil)

	// Create encrypted structure
	encrypted := encryptedFile{
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Nonce:     base64.StdEncoding.EncodeToString(nonce),
		Encrypted: base64.StdEncoding.EncodeToString(encryptedData),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(encrypted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted credentials: %w", err)
	}

	// Write to file with restricted permissions
	if err := os.WriteFile(fs.credentialsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// generateMasterKey generates a master key for encryption
func generateMasterKey() []byte {
	// In a real implementation, this should be derived from user input
	// or stored securely. For now, we'll use a machine-specific key.
	hostname, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME") // Windows
	}

	keyMaterial := fmt.Sprintf("nl-to-shell-%s-%s-%s", hostname, user, runtime.GOOS)
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:]
}

// GetCredentialFromEnv retrieves a credential from environment variables
func GetCredentialFromEnv(service, account string) string {
	// Convert to uppercase for environment variable names
	serviceUpper := strings.ToUpper(service)
	accountUpper := strings.ToUpper(account)

	// Try common environment variable patterns
	envVars := []string{
		fmt.Sprintf("%s_%s_API_KEY", serviceUpper, accountUpper),
		fmt.Sprintf("%s_API_KEY", serviceUpper),
		fmt.Sprintf("%s_%s_KEY", serviceUpper, accountUpper),
		fmt.Sprintf("%s_KEY", serviceUpper),
		fmt.Sprintf("%s_%s_TOKEN", serviceUpper, accountUpper),
		fmt.Sprintf("%s_TOKEN", serviceUpper),
	}

	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			return value
		}
	}

	return ""
}
