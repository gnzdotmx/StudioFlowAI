package tiktok

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/services/tiktok"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
)

// UploadTikTokShortsModule implements TikTok shorts upload functionality
type UploadTikTokShortsModule struct {
	serviceFactory func() (tiktok.Service, error)
}

// GetIO returns the module's input/output specification
func (m *UploadTikTokShortsModule) GetIO() modules.ModuleIO {
	return modules.ModuleIO{
		RequiredInputs: []modules.ModuleInput{
			{
				Name:        "input",
				Description: "Path to shorts suggestions YAML file",
				Patterns:    []string{"*.yaml"},
				Type:        string(modules.InputTypeFile),
			},
			{
				Name:        "storedShortsPath",
				Description: "Path where the short videos are stored",
				Patterns:    []string{"*.mp4"},
				Type:        string(modules.InputTypeDirectory),
			},
		},
		OptionalInputs: []modules.ModuleInput{
			{
				Name:        "privacyStatus",
				Description: "Video privacy status (private, public)",
				Type:        string(modules.InputTypeData),
			},
		},
		ProducedOutputs: []modules.ModuleOutput{
			{
				Name:        "uploadStatus",
				Description: "JSON file containing upload status for each video",
				Patterns:    []string{"*.json"},
				Type:        string(modules.OutputTypeFile),
			},
		},
	}
}

// UploadTikTokShortsParams contains the parameters for TikTok shorts upload operations
type UploadTikTokShortsParams struct {
	Input            string `json:"input"`            // Path to shorts suggestions YAML file
	Output           string `json:"output"`           // Path to output directory
	StoredShortsPath string `json:"storedShortsPath"` // Path where the short videos are stored
	PrivacyStatus    string `json:"privacyStatus"`    // Video privacy status (private, public)
}

// VideoUploadStatus represents the status of a video upload
type VideoUploadStatus struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // "uploaded", "failed", "pending"
	Error       string    `json:"error,omitempty"`
	UploadTime  time.Time `json:"uploadTime,omitempty"`
}

// UploadStatusResult represents the collection of video upload statuses
type UploadStatusResult struct {
	Videos []VideoUploadStatus `json:"videos"`
}

// TikTokService represents the TikTok API service
type TikTokService struct {
	ClientKey    string
	ClientSecret string
	AccessToken  string
	OAuthConfig  OAuthConfig
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

// VideoUpload represents a video to be uploaded to TikTok
type VideoUpload struct {
	FileName       string
	ShortTitle     string
	Description    string
	Tags           string
	RelatedVideoID string
}

// NewUploadTikTokShorts creates a new TikTok shorts upload module
func NewUploadTikTokShorts() modules.Module {
	return &UploadTikTokShortsModule{
		serviceFactory: tiktok.NewService,
	}
}

// NewUploadTikTokShortsWithService creates a new TikTok shorts upload module with a custom service factory
func NewUploadTikTokShortsWithService(factory func() (tiktok.Service, error)) modules.Module {
	return &UploadTikTokShortsModule{
		serviceFactory: factory,
	}
}

// Name returns the module name
func (m *UploadTikTokShortsModule) Name() string {
	return "uploadtiktokshorts"
}

// Validate checks if the parameters are valid
func (m *UploadTikTokShortsModule) Validate(params map[string]interface{}) error {
	var p UploadTikTokShortsParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Print params
	utils.LogInfo("Params: %+v", p)

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, ""); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Validate privacy status
	if p.PrivacyStatus == "" {
		p.PrivacyStatus = "private" // Default to private
	}
	if p.PrivacyStatus != "private" && p.PrivacyStatus != "public" {
		return fmt.Errorf("invalid privacy status: %s", p.PrivacyStatus)
	}

	return nil
}

// Execute performs TikTok operations
func (m *UploadTikTokShortsModule) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p UploadTikTokShortsParams
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
	}

	// Initialize TikTok service
	service, err := m.serviceFactory()
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to create TikTok service: %w", err)
	}

	// Initialize service with default OAuth config
	if err := service.Initialize(tiktok.DefaultOAuthConfig()); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to initialize TikTok service: %w", err)
	}

	// Read shorts suggestions file
	shortsData, err := utils.ReadShortsFile(p.Input)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to read shorts suggestions file: %w", err)
	}

	// Create video uploads from shorts data
	var videoUploads []VideoUpload
	for _, short := range shortsData.Shorts {
		videoUpload := VideoUpload{
			FileName:    fmt.Sprintf("%s-%s-withtext.mp4", convertToHHMMSS(short.StartTime), convertToHHMMSS(short.EndTime)),
			ShortTitle:  short.ShortTitle,
			Description: short.Description,
			Tags:        short.Tags,
		}
		videoUploads = append(videoUploads, videoUpload)
	}

	utils.LogInfo("--------------------------------")
	// Upload each video
	for _, upload := range videoUploads {
		videoPath := filepath.Join(p.StoredShortsPath, upload.FileName)
		if err := service.UploadVideo(ctx, videoPath, upload.ShortTitle, upload.Description, p.PrivacyStatus, time.Now()); err != nil {
			return modules.ModuleResult{}, fmt.Errorf("failed to upload video %s: %w", upload.FileName, err)
		}
		utils.LogInfo("\t Uploaded video: %s", upload.ShortTitle)
	}
	utils.LogInfo("--------------------------------")

	// Prepare result
	result := modules.ModuleResult{
		Outputs: map[string]string{
			"uploadStatus": fmt.Sprintf("%s/tiktok_upload_status.json", p.Output),
		},
		Metadata: map[string]interface{}{
			"totalVideos": len(videoUploads),
		},
		Statistics: map[string]interface{}{
			"uploadedVideos": len(videoUploads),
		},
	}

	return result, nil
}

// convertToHHMMSS converts a timestamp to HHMMSS format
func convertToHHMMSS(timestamp string) string {
	// Remove colons
	return strings.ReplaceAll(timestamp, ":", "")
}
