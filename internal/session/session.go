package session

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateSessionDir creates a unique session directory under /tmp/wisp/
func GenerateSessionDir() (string, error) {
	// Generate a random UUID-like string
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	
	// Create the wisp session directory
	sessionDir := filepath.Join("/tmp", "wisp", sessionID)
	
	// Create the directory with proper permissions
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}
	
	return sessionDir, nil
}

// CleanupSessionDir removes the session directory and all its contents
func CleanupSessionDir(sessionDir string) error {
	if sessionDir == "" {
		return nil
	}
	
	// Safety check - only remove directories under /tmp/wisp/
	if !filepath.HasPrefix(sessionDir, "/tmp/wisp/") {
		return fmt.Errorf("invalid session directory path: %s", sessionDir)
	}
	
	return os.RemoveAll(sessionDir)
}

// generateSessionID creates a random 8-character session identifier
func generateSessionID() (string, error) {
	bytes := make([]byte, 4) // 4 bytes = 8 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%08x", bytes), nil
}
