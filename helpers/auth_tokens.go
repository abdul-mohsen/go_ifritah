package helpers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"afrita/config"
	"afrita/models"
)

// SaveTokenToFile persists token data to a JSON file
func SaveTokenToFile(sessionID string, token *models.Token) error {
	if sessionID == "" || token == nil {
		return fmt.Errorf("invalid sessionID or token")
	}

	// Create a base64-encoded filename for security
	filename := base64.StdEncoding.EncodeToString([]byte(sessionID)) + ".json"
	filepath := filepath.Join(config.TokenStoreDir, filename)

	// Marshal token to JSON
	jsonData, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %v", err)
	}

	// Write to file with restricted permissions
	if err := os.WriteFile(filepath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %v", err)
	}

	log.Printf("✅ Token persisted to disk for session: %s (expires: %s)", sessionID, token.ExpiresAt.Format("2006-01-02 15:04:05"))
	return nil
}

// LoadTokenFromFile retrieves persisted token data from file
func LoadTokenFromFile(sessionID string) (*models.Token, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("invalid sessionID")
	}

	filename := base64.StdEncoding.EncodeToString([]byte(sessionID)) + ".json"
	filepath := filepath.Join(config.TokenStoreDir, filename)

	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("token file not found: %v", err)
	}

	var token models.Token
	if err := json.Unmarshal(jsonData, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %v", err)
	}

	return &token, nil
}

// DeleteTokenFile removes persisted token file
func DeleteTokenFile(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("invalid sessionID")
	}

	filename := base64.StdEncoding.EncodeToString([]byte(sessionID)) + ".json"
	filepath := filepath.Join(config.TokenStoreDir, filename)

	if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token file: %v", err)
	}

	log.Printf("🗑️  Token file deleted for session: %s", sessionID)
	return nil
}

// LoadPersistedTokens loads all tokens from disk into memory on app startup
func LoadPersistedTokens() {
	dirEntries, err := os.ReadDir(config.TokenStoreDir)
	if err != nil {
		log.Printf("⚠️  No persisted tokens found or error reading token directory: %v", err)
		return
	}

	loadCount := 0
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filepath := filepath.Join(config.TokenStoreDir, entry.Name())
		jsonData, err := os.ReadFile(filepath)
		if err != nil {
			log.Printf("⚠️  Failed to read token file %s: %v", entry.Name(), err)
			continue
		}

		var token models.Token
		if err := json.Unmarshal(jsonData, &token); err != nil {
			log.Printf("⚠️  Failed to unmarshal token file %s: %v", entry.Name(), err)
			continue
		}

		// Decode sessionID from filename
		sessionIDEncoded := strings.TrimSuffix(entry.Name(), ".json")
		sessionIDBytes, err := base64.StdEncoding.DecodeString(sessionIDEncoded)
		if err != nil {
			log.Printf("⚠️  Failed to decode session ID from filename %s: %v", entry.Name(), err)
			continue
		}

		sessionID := string(sessionIDBytes)

		// Only load non-expired tokens
		if time.Now().Before(token.ExpiresAt) {
			config.SessionTokensMutex.Lock()
			config.SessionTokens[sessionID] = token.AccessToken
			config.SessionRefreshTokens[sessionID] = token.RefreshToken
			config.SessionTokenExpiry[sessionID] = token.ExpiresAt
			config.SessionTokensMutex.Unlock()
			loadCount++
			log.Printf("✅ Loaded persisted token for session: %s (expires: %s)", sessionID, token.ExpiresAt.Format("2006-01-02 15:04:05"))
		} else {
			// Delete expired token file
			_ = DeleteTokenFile(sessionID)
		}
	}

	log.Printf("🔄 Loaded %d persisted tokens from disk", loadCount)
}

// PeriodicTokenCleanup removes tokens older than 7 days (runs every 1 hour)
func PeriodicTokenCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expiredSessions := []string{}

		// Find sessions where access token expired more than 7 days ago
		config.SessionTokensMutex.RLock()
		for sessionID, expiryTime := range config.SessionTokenExpiry {
			if now.After(expiryTime.Add(7 * 24 * time.Hour)) {
				expiredSessions = append(expiredSessions, sessionID)
			}
		}
		config.SessionTokensMutex.RUnlock()

		// Remove expired tokens
		if len(expiredSessions) > 0 {
			config.SessionTokensMutex.Lock()
			for _, sessionID := range expiredSessions {
				delete(config.SessionTokens, sessionID)
				delete(config.SessionRefreshTokens, sessionID)
				delete(config.SessionTokenExpiry, sessionID)
				_ = DeleteTokenFile(sessionID)
			}
			config.SessionTokensMutex.Unlock()
			log.Printf("🧹 Cleanup: Removed %d expired tokens", len(expiredSessions))
		}
	}
}

// ShouldRefreshToken checks if token is near expiry (within 2 minutes) or already expired
func ShouldRefreshToken(sessionID string) bool {
	config.SessionTokensMutex.RLock()
	expiryTime, exists := config.SessionTokenExpiry[sessionID]
	config.SessionTokensMutex.RUnlock()

	if !exists {
		return false
	}

	// Refresh if expired or will expire in next 2 minutes
	return time.Now().Add(2 * time.Minute).After(expiryTime)
}

// RefreshTokenIfNeeded attempts to refresh the token if it's near expiry
func RefreshTokenIfNeeded(w http.ResponseWriter, r *http.Request, sessionID string) bool {
	config.SessionTokensMutex.RLock()
	refreshToken, hasRefresh := config.SessionRefreshTokens[sessionID]
	config.SessionTokensMutex.RUnlock()

	if !hasRefresh || refreshToken == "" || !ShouldRefreshToken(sessionID) {
		return true // Token is still valid
	}

	log.Printf("🔄 Token near expiry for session %s, attempting refresh...", sessionID)

	// Try to refresh the token
	refreshReq, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/refresh", nil)
	refreshReq.Header.Set("Authorization", "Bearer "+refreshToken)

	httpClient := HttpClient
	refreshResp, err := httpClient.Do(refreshReq)
	if err != nil {
		log.Printf("❌ Token refresh failed: %v", err)
		return false
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		log.Printf("❌ Token refresh failed with status: %d", refreshResp.StatusCode)
		return false
	}

	var authResp models.AuthResponse
	if err := json.NewDecoder(refreshResp.Body).Decode(&authResp); err != nil {
		log.Printf("❌ Failed to decode refresh response: %v", err)
		return false
	}

	if authResp.AccessToken == "" {
		log.Printf("❌ Refresh response missing access token")
		return false
	}

	// Update session with new tokens
	newExpiryTime := time.Now().Add(15 * time.Minute)
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = authResp.AccessToken
	if authResp.RefreshToken != "" {
		config.SessionRefreshTokens[sessionID] = authResp.RefreshToken
	}
	config.SessionTokenExpiry[sessionID] = newExpiryTime
	config.SessionTokensMutex.Unlock()

	// Persist updated tokens
	token := &models.Token{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresAt:    newExpiryTime,
		CreatedAt:    time.Now(),
	}
	if err := SaveTokenToFile(sessionID, token); err != nil {
		log.Printf("⚠️  Failed to persist refreshed token: %v", err)
	}

	log.Printf("✅ Token refreshed successfully for session: %s", sessionID)
	return true
}
