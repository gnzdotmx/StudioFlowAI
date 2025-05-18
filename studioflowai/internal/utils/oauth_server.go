package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

// TokenStorage handles storing and retrieving OAuth tokens
type TokenStorage struct {
	configDir string
}

// NewTokenStorage creates a new token storage instance
func NewTokenStorage() (*TokenStorage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".studioflowai")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &TokenStorage{
		configDir: configDir,
	}, nil
}

// SaveToken saves the OAuth token to disk
func (s *TokenStorage) SaveToken(service string, token *oauth2.Token) error {
	tokenPath := filepath.Join(s.configDir, fmt.Sprintf("%s_token.json", service))

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// LoadToken loads the OAuth token from disk
func (s *TokenStorage) LoadToken(service string) (*oauth2.Token, error) {
	tokenPath := filepath.Join(s.configDir, fmt.Sprintf("%s_token.json", service))

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Token doesn't exist yet
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// OAuthCallbackServer handles the OAuth callback and returns the authorization code
type OAuthCallbackServer struct {
	codeChan chan string
	server   *http.Server
	wg       sync.WaitGroup
}

// NewOAuthCallbackServer creates a new OAuth callback server
func NewOAuthCallbackServer() *OAuthCallbackServer {
	return &OAuthCallbackServer{
		codeChan: make(chan string, 1),
	}
}

// Start starts the callback server on the specified port
func (s *OAuthCallbackServer) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleCallback)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			LogError("Callback server error: %v", err)
		}
	}()

	return nil
}

// handleCallback processes the OAuth callback and extracts the authorization code
func (s *OAuthCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		return
	}

	// Send the code through the channel
	s.codeChan <- code

	// Respond to the browser
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
		<html>
			<body>
				<h1>Authorization Successful!</h1>
				<p>You can close this window and return to the application.</p>
			</body>
		</html>
	`)
}

// WaitForCode waits for the authorization code
func (s *OAuthCallbackServer) WaitForCode() string {
	return <-s.codeChan
}

// Stop stops the callback server
func (s *OAuthCallbackServer) Stop() error {
	if s.server != nil {
		if err := s.server.Close(); err != nil {
			return fmt.Errorf("failed to stop callback server: %w", err)
		}
		s.wg.Wait()
	}
	return nil
}
