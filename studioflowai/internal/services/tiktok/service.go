package tiktok

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"golang.org/x/oauth2"
)

func init() {
}

// OAuthConfig represents the OAuth configuration
type OAuthConfig struct {
	RedirectURI string
	Scopes      []string
}

// DefaultOAuthConfig returns the default OAuth configuration
func DefaultOAuthConfig() OAuthConfig {
	return OAuthConfig{
		RedirectURI: "http://localhost:8080/callback",
		Scopes: []string{
			"user.info.basic",
			"video.upload",
			"video.list",
			"video.publish",
		},
	}
}

// service implements the Service interface
type service struct {
	clientKey    string
	clientSecret string
	accessToken  string
	oauthConfig  OAuthConfig
}

// NewService creates a new TikTok service
func NewService() (Service, error) {
	// Get credentials from environment variables
	clientKey := os.Getenv("TIKTOK_CLIENT_KEY")
	if clientKey == "" {
		return nil, fmt.Errorf("TIKTOK_CLIENT_KEY environment variable is not set")
	}

	clientSecret := os.Getenv("TIKTOK_CLIENT_SECRET")
	if clientSecret == "" {
		return nil, fmt.Errorf("TIKTOK_CLIENT_SECRET environment variable is not set")
	}

	return &service{
		clientKey:    clientKey,
		clientSecret: clientSecret,
		oauthConfig:  DefaultOAuthConfig(),
	}, nil
}

// GetAccessToken returns the current access token
func (s *service) GetAccessToken() string {
	return s.accessToken
}

// Initialize initializes the service with OAuth configuration
func (s *service) Initialize(config interface{}) error {
	// Convert config to OAuthConfig
	oauthConfig, ok := config.(OAuthConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected OAuthConfig")
	}

	s.oauthConfig = oauthConfig

	// Get or refresh token
	token, err := s.getValidToken()
	if err != nil {
		return fmt.Errorf("failed to get valid token: %w", err)
	}

	s.accessToken = token.AccessToken
	return nil
}

// UploadVideo uploads a video to TikTok
func (s *service) UploadVideo(ctx context.Context, videoPath string, title string, description string, privacy string, publishTime time.Time) error {
	// Open and read the video file
	file, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("failed to open video file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			utils.LogWarning("Failed to close video file: %v", err)
		}
	}()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Initialize upload
	initURL := "https://open.tiktokapis.com/v2/post/publish/inbox/video/init/"
	initBody := map[string]interface{}{
		"source_info": map[string]interface{}{
			"source":            "FILE_UPLOAD",
			"video_size":        fileInfo.Size(),
			"chunk_size":        fileInfo.Size(),
			"total_chunk_count": 1,
		},
	}

	initJSON, err := json.Marshal(initBody)
	if err != nil {
		return fmt.Errorf("failed to marshal init request: %w", err)
	}

	initReq, err := http.NewRequestWithContext(ctx, "POST", initURL, bytes.NewBuffer(initJSON))
	if err != nil {
		return fmt.Errorf("failed to create init request: %w", err)
	}

	// Set authorization header with proper format
	initReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.accessToken))
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")

	utils.LogInfo("Authorization header: %s", initReq.Header.Get("Authorization"))
	utils.LogInfo("Init request body: %s", string(initJSON))

	client := &http.Client{}
	initResp, err := client.Do(initReq)
	if err != nil {
		return fmt.Errorf("failed to send init request: %w", err)
	}
	defer func() {
		if err := initResp.Body.Close(); err != nil {
			utils.LogWarning("Failed to close init response body: %v", err)
		}
	}()

	initBodyBytes, err := io.ReadAll(initResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read init response body: %w", err)
	}

	if initResp.StatusCode != http.StatusOK {
		return fmt.Errorf("init API request failed with status: %d, body: %s", initResp.StatusCode, string(initBodyBytes))
	}

	var initResult struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			LogID   string `json:"log_id"`
		} `json:"error"`
	}

	if err := json.NewDecoder(bytes.NewReader(initBodyBytes)).Decode(&initResult); err != nil {
		return fmt.Errorf("failed to decode init response: %w", err)
	}

	if initResult.Error.Code != "" && initResult.Error.Code != "ok" {
		return fmt.Errorf("init API error: %s - %s (log_id: %s)", initResult.Error.Code, initResult.Error.Message, initResult.Error.LogID)
	}

	// Upload the video file
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to start of file: %w", err)
	}
	videoData, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read video file: %w", err)
	}

	uploadReq, err := http.NewRequestWithContext(ctx, "PUT", initResult.Data.UploadURL, bytes.NewReader(videoData))
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	uploadReq.Header.Set("Content-Type", "video/mp4")
	uploadReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(videoData)))
	uploadReq.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(videoData)-1, len(videoData)))

	uploadResp, err := client.Do(uploadReq)
	if err != nil {
		return fmt.Errorf("failed to send upload request: %w", err)
	}
	defer func() {
		if err := uploadResp.Body.Close(); err != nil {
			utils.LogWarning("Failed to close upload response body: %v", err)
		}
	}()

	uploadBodyBytes, err := io.ReadAll(uploadResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read upload response body: %w", err)
	}

	if uploadResp.StatusCode != http.StatusCreated && uploadResp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload API request failed with status: %d, body: %s", uploadResp.StatusCode, string(uploadBodyBytes))
	}

	// Check upload status
	maxRetries := 30 // 5 minutes with 10-second intervals
	for i := 0; i < maxRetries; i++ {
		statusURL := "https://open.tiktokapis.com/v2/post/publish/status/fetch/"
		statusBody := map[string]interface{}{
			"publish_id": initResult.Data.PublishID,
		}

		statusJSON, err := json.Marshal(statusBody)
		if err != nil {
			return fmt.Errorf("failed to marshal status request: %w", err)
		}

		statusReq, err := http.NewRequestWithContext(ctx, "POST", statusURL, bytes.NewBuffer(statusJSON))
		if err != nil {
			return fmt.Errorf("failed to create status request: %w", err)
		}

		statusReq.Header.Set("Authorization", "Bearer "+s.accessToken)
		statusReq.Header.Set("Content-Type", "application/json; charset=UTF-8")

		statusResp, err := client.Do(statusReq)
		if err != nil {
			return fmt.Errorf("failed to send status request: %w", err)
		}
		defer func() {
			if err := statusResp.Body.Close(); err != nil {
				utils.LogWarning("Failed to close status response body: %v", err)
			}
		}()

		statusBodyBytes, err := io.ReadAll(statusResp.Body)
		if err != nil {
			return fmt.Errorf("failed to read status response body: %w", err)
		}

		if statusResp.StatusCode != http.StatusOK {
			return fmt.Errorf("status API request failed with status: %d, body: %s", statusResp.StatusCode, string(statusBodyBytes))
		}

		var statusResult struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				LogID   string `json:"log_id"`
			} `json:"error"`
		}

		if err := json.NewDecoder(bytes.NewReader(statusBodyBytes)).Decode(&statusResult); err != nil {
			return fmt.Errorf("failed to decode status response: %w", err)
		}

		if statusResult.Error.Code != "" && statusResult.Error.Code != "ok" {
			return fmt.Errorf("status API error: %s - %s (log_id: %s)", statusResult.Error.Code, statusResult.Error.Message, statusResult.Error.LogID)
		}

		if statusResult.Data.Status == "UPLOADED" || statusResult.Data.Status == "SEND_TO_USER_INBOX" {
			if statusResult.Data.Status == "SEND_TO_USER_INBOX" {
				return nil // Success - video is in user's inbox
			}
			break // Continue to completion if status is UPLOADED
		}

		if statusResult.Data.Status == "FAILED" {
			return fmt.Errorf("video upload failed")
		}

		if i < maxRetries-1 {
			time.Sleep(10 * time.Second)
		}
	}

	// Complete the upload
	completeURL := fmt.Sprintf("https://open.tiktokapis.com/v2/post/publish/inbox/video/complete/?publish_id=%s", initResult.Data.PublishID)
	completeReq, err := http.NewRequestWithContext(ctx, "POST", completeURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create complete request: %w", err)
	}

	completeReq.Header.Set("Authorization", "Bearer "+s.accessToken)
	completeReq.Header.Set("Content-Type", "application/json; charset=UTF-8")

	completeResp, err := client.Do(completeReq)
	if err != nil {
		return fmt.Errorf("failed to send complete request: %w", err)
	}
	defer func() {
		if err := completeResp.Body.Close(); err != nil {
			utils.LogWarning("Failed to close complete response body: %v", err)
		}
	}()

	completeBodyBytes, err := io.ReadAll(completeResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read complete response body: %w", err)
	}

	if completeResp.StatusCode != http.StatusOK {
		return fmt.Errorf("complete API request failed with status: %d, body: %s", completeResp.StatusCode, string(completeBodyBytes))
	}

	var completeResult struct {
		Data struct {
			VideoID string `json:"video_id"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			LogID   string `json:"log_id"`
		} `json:"error"`
	}

	if err := json.NewDecoder(bytes.NewReader(completeBodyBytes)).Decode(&completeResult); err != nil {
		return fmt.Errorf("failed to decode complete response: %w", err)
	}

	if completeResult.Error.Code != "" && completeResult.Error.Code != "ok" {
		return fmt.Errorf("complete API error: %s - %s (log_id: %s)", completeResult.Error.Code, completeResult.Error.Message, completeResult.Error.LogID)
	}

	return nil
}

// GetUploadedVideos retrieves the list of videos already uploaded to TikTok
func (s *service) GetUploadedVideos(ctx context.Context) ([]VideoInfo, error) {
	// Implementation of GetUploadedVideos...
	return nil, nil
}

// getValidToken gets a valid token, either from storage or through OAuth flow
func (s *service) getValidToken() (*oauth2.Token, error) {
	// Create token storage directory if it doesn't exist
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	tokenDir := filepath.Join(homeDir, ".studioflowai")
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create token directory: %w", err)
	}

	// Try to load existing token
	tokenPath := filepath.Join(tokenDir, "tiktok_token.json")
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.LogWarning("Failed to read token file: %v", err)
		}
		tokenData = nil
	}

	var token *oauth2.Token
	if tokenData != nil {
		if err := json.Unmarshal(tokenData, &token); err != nil {
			utils.LogWarning("Failed to parse token data: %v", err)
			token = nil
		}
	}

	// If no token exists or it's expired, get a new one
	if token == nil || !token.Valid() {
		utils.LogInfo("No valid token found, starting OAuth flow...")
		token, err = s.performOAuthFlow()
		if err != nil {
			return nil, fmt.Errorf("OAuth flow failed: %w", err)
		}

		// Save the new token
		tokenData, err = json.Marshal(token)
		if err != nil {
			utils.LogWarning("Failed to marshal token: %v", err)
		} else {
			if err := os.WriteFile(tokenPath, tokenData, 0600); err != nil {
				utils.LogWarning("Failed to save token: %v", err)
			}
		}
	} else {
		utils.LogInfo("Using existing authorization token")
	}

	return token, nil
}

// performOAuthFlow performs the OAuth authorization flow
func (s *service) performOAuthFlow() (*oauth2.Token, error) {
	// Initialize OAuth callback server with fixed port 8080
	callbackServer := utils.NewOAuthCallbackServer()
	if err := callbackServer.Start(8080); err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	defer func() {
		if err := callbackServer.Stop(); err != nil {
			utils.LogWarning("Failed to stop callback server: %v", err)
		}
	}()

	// Use fixed redirect URI
	redirectURI := "http://localhost:8080/callback"

	// Generate PKCE code verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Generate state parameter for CSRF protection
	state := generateRandomString(32)

	// Construct authorization URL
	authURL := fmt.Sprintf(
		"https://www.tiktok.com/v2/auth/authorize/?client_key=%s&scope=%s&response_type=code&redirect_uri=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape(s.clientKey),
		url.QueryEscape(strings.Join(s.oauthConfig.Scopes, ",")),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
		url.QueryEscape(codeChallenge),
	)

	utils.LogInfo("Opening browser for TikTok authorization...")
	utils.LogInfo("If the browser doesn't open automatically, please visit: %s", authURL)

	// Open browser for user authorization
	if err := callbackServer.OpenURL(authURL); err != nil {
		utils.LogWarning("Failed to open browser automatically: %v", err)
		utils.LogInfo("Please open the following URL in your browser:")
		utils.LogInfo("Authorization URL: %s", authURL)
	}

	utils.LogInfo("Waiting for authorization code from TikTok...")
	code := callbackServer.WaitForCode()
	if code == "" {
		return nil, fmt.Errorf("failed to receive authorization code")
	}

	// Exchange code for access token
	accessToken, err := exchangeCodeForToken(s.clientKey, s.clientSecret, code, codeVerifier, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Create a new token
	token := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		// Set expiry to 1 hour from now as per TikTok API docs
		Expiry: time.Now().Add(1 * time.Hour),
	}

	utils.LogInfo("Successfully obtained new access token")
	return token, nil
}

// generateCodeVerifier generates a random string for use as a PKCE code verifier
func generateCodeVerifier() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	const length = 128
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[mathrand.Intn(len(charset))]
	}
	return string(b)
}

// generateCodeChallenge generates a PKCE code challenge from the verifier
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	challenge := hex.EncodeToString(hash[:])
	utils.LogInfo("Code verifier: %s", verifier)
	utils.LogInfo("Code challenge: %s", challenge)
	return challenge
}

// generateRandomString generates a random string of the specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(err) // This should never happen
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// exchangeCodeForToken exchanges an authorization code for an access token
func exchangeCodeForToken(clientKey, clientSecret, code, codeVerifier, redirectURI string) (string, error) {
	// Create token exchange request
	tokenURL := "https://open.tiktokapis.com/v2/oauth/token/"
	data := url.Values{}
	data.Set("client_key", clientKey)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", redirectURI)

	// Send request
	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to send token request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.LogWarning("Failed to close response body: %v", err)
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if response is successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int    `json:"expires_in"`
		OpenID           string `json:"open_id"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	// Check for error in response
	if result.Error != "" {
		return "", fmt.Errorf("API error: %s - %s", result.Error, result.ErrorDescription)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response: %s", string(body))
	}

	return result.AccessToken, nil
}
