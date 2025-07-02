package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if _, err := fmt.Fprintf(w, `
		<html>
			<head>
				<title>Authorization Successful</title>
				<style>
					body {
						font-family: Arial, sans-serif;
						display: flex;
						justify-content: center;
						align-items: center;
						height: 100vh;
						margin: 0;
						background-color: #f0f2f5;
					}
					.container {
						text-align: center;
						padding: 2rem;
						background-color: white;
						border-radius: 8px;
						box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
					}
					h1 {
						color: #1a73e8;
						margin-bottom: 1rem;
					}
					p {
						color: #5f6368;
						margin-bottom: 0.5rem;
					}
				</style>
			</head>
			<body>
				<div class="container">
					<h1>Authorization Successful</h1>
					<p>You can now close this window and return to the application.</p>
				</div>
			</body>
		</html>
	`); err != nil {
		LogWarning("Failed to write response: %v", err)
	}
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

// GetServerAddr returns the server's address
func (s *OAuthCallbackServer) GetServerAddr() string {
	return s.server.Addr
}

// openURL opens the specified URL in the default browser
func (s *OAuthCallbackServer) OpenURL(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("cannot open URL %s on this platform", url)
	}
	return err
}
