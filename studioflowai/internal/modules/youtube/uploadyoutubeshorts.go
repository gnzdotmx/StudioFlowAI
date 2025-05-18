package youtube

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"google.golang.org/api/youtube/v3"
	"gopkg.in/yaml.v3"
)

// UploadYouTubeShortsModule implements YouTube shorts upload functionality
type UploadYouTubeShortsModule struct{}

// UploadYouTubeShortsParams contains the parameters for YouTube shorts upload operations
type UploadYouTubeShortsParams struct {
	Input               string `json:"input"`               // Path to shorts suggestions YAML file
	Output              string `json:"output"`              // Path to output directory
	StoredShortsPath    string `json:"storedShortsPath"`    // Path where the short videos are stored
	Credentials         string `json:"credentials"`         // Path to Google credentials file
	PlaylistID          string `json:"playlistId"`          // Optional: YouTube playlist ID
	PrivacyStatus       string `json:"privacyStatus"`       // Video privacy status (private, unlisted, public)
	CategoryID          string `json:"categoryId"`          // Video category ID
	SchedulePeriodicity int    `json:"schedulePeriodicity"` // Schedule videos every N days
	ScheduleTime        string `json:"scheduleTime"`        // Time to schedule videos (24-hour format)
	MaxAttempts         int    `json:"maxAttempts"`         // Maximum number of days to search for available slots
	StartDate           string `json:"startDate"`           // Start date for scheduling (YYYY-MM-DD)
	RelatedVideoID      string `json:"relatedVideoId"`      // ID of the related video to link with shorts
}

// NewUploadYouTubeShorts creates a new YouTube shorts upload module
func NewUploadYouTubeShorts() modules.Module {
	return &UploadYouTubeShortsModule{}
}

// Name returns the module name
func (m *UploadYouTubeShortsModule) Name() string {
	return "uploadyoutubeshorts"
}

// Validate checks if the parameters are valid
func (m *UploadYouTubeShortsModule) Validate(params map[string]interface{}) error {
	var p UploadYouTubeShortsParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, ""); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Validate credentials file
	if p.Credentials == "" {
		p.Credentials = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if p.Credentials == "" {
			return fmt.Errorf("credentials file path is required")
		}
	}

	// Expand home directory if present
	expandedCredentials, err := utils.ExpandHomeDir(p.Credentials)
	if err != nil {
		return fmt.Errorf("failed to expand home directory: %w", err)
	}
	p.Credentials = expandedCredentials

	if _, err := os.Stat(p.Credentials); os.IsNotExist(err) {
		return fmt.Errorf("credentials file does not exist: %s", p.Credentials)
	}

	// Validate privacy status
	if p.PrivacyStatus == "" {
		p.PrivacyStatus = "private" // Default to private
	}
	if p.PrivacyStatus != "private" && p.PrivacyStatus != "unlisted" && p.PrivacyStatus != "public" {
		return fmt.Errorf("invalid privacy status: %s", p.PrivacyStatus)
	}

	return nil
}

// Execute performs YouTube operations
func (m *UploadYouTubeShortsModule) Execute(ctx context.Context, params map[string]interface{}) error {
	var p UploadYouTubeShortsParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default maxAttempts if not provided
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 60 // Default to 60 days if not specified
	}

	// Set default startDate if not provided
	if p.StartDate == "" {
		p.StartDate = time.Now().UTC().Format("2006-01-02")
	}

	// Expand home directory if present
	expandedCredentials, err := utils.ExpandHomeDir(p.Credentials)
	if err != nil {
		return fmt.Errorf("failed to expand home directory: %w", err)
	}
	p.Credentials = expandedCredentials

	// Initialize YouTube service
	service, err := m.initializeYouTubeService(ctx, p.Credentials)
	if err != nil {
		return fmt.Errorf("failed to initialize YouTube service: %w", err)
	}

	// Read and list scheduled videos
	scheduledVideos, err := m.readScheduledVideos(ctx, service)
	if err != nil {
		return fmt.Errorf("failed to read scheduled videos: %w", err)
	}

	// if err := m.listScheduledVideos(scheduledVideos); err != nil {
	// 	return fmt.Errorf("failed to list scheduled videos: %w", err)
	// }

	// Read shorts suggestions file
	shortsData, err := m.readShortsFile(p.Input)
	if err != nil {
		return fmt.Errorf("failed to read shorts suggestions file: %w", err)
	}

	// // List available shorts
	// if err := m.listShorts(ctx, service, shortsData); err != nil {
	// 	return fmt.Errorf("failed to list available shorts: %w", err)
	// }

	// Find available times for each short
	var videoUploads []VideoUpload
	videoUploads, err = m.findAvailability(scheduledVideos, shortsData, p.SchedulePeriodicity, p.ScheduleTime, p.MaxAttempts, p.StartDate, p.PlaylistID)
	if err != nil {
		return fmt.Errorf("failed to find availability: %w", err)
	}

	// Collect tags and related video ID
	videoUploads, err = m.collectTagsAndRelatedVideo(ctx, service, videoUploads, p.RelatedVideoID)
	if err != nil {
		return fmt.Errorf("failed to collect tags and related video: %w", err)
	}

	// List available times
	if err := m.listAvailableTimes(videoUploads); err != nil {
		return fmt.Errorf("failed to list available times: %w", err)
	}

	// Upload the videos
	if err := m.uploadVideo(ctx, service, videoUploads, p.PrivacyStatus, p.CategoryID, p.StoredShortsPath); err != nil {
		return fmt.Errorf("failed to upload videos: %w", err)
	}

	return nil
}

// readShortsFile reads and parses the shorts_suggestions.yaml file
func (m *UploadYouTubeShortsModule) readShortsFile(filePath string) (*ShortsData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var shortsData ShortsData
	if err := yaml.Unmarshal(data, &shortsData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &shortsData, nil
}

// listShorts lists available shorts that can be uploaded
func (m *UploadYouTubeShortsModule) listShorts(ctx context.Context, service *youtube.Service, shortsData *ShortsData) error {
	utils.LogInfo("Available shorts for upload:")
	for i, short := range shortsData.Shorts {
		utils.LogInfo("%d. Title: %s", i+1, short.ShortTitle)
		utils.LogInfo("   Duration: %s - %s", short.StartTime, short.EndTime)
		utils.LogInfo("   Description: %s", short.Description)
		utils.LogInfo("   Tags: %s", short.Tags)
		utils.LogInfo("---")
	}
	return nil
}

// collectTagsAndRelatedVideo adds tags from the related video and adds related video ID to the video uploads
func (m *UploadYouTubeShortsModule) collectTagsAndRelatedVideo(ctx context.Context, service *youtube.Service, videoUploads []VideoUpload, relatedVideoID string) ([]VideoUpload, error) {
	// If no related video ID is provided, just return the uploads as is
	if relatedVideoID == "" {
		return videoUploads, nil
	}

	// Get the related video details to extract tags
	videoResponse, err := service.Videos.List([]string{"snippet"}).Id(relatedVideoID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get related video details: %w", err)
	}

	if len(videoResponse.Items) == 0 {
		return nil, fmt.Errorf("no video found with ID: %s", relatedVideoID)
	}

	// Get tags from the related video
	relatedVideoTags := videoResponse.Items[0].Snippet.Tags
	if len(relatedVideoTags) == 0 {
		utils.LogWarning("No tags found in related video: %s", relatedVideoID)
	}

	// Add related video ID and tags to each video upload
	for i := range videoUploads {
		videoUploads[i].RelatedVideoID = relatedVideoID

		// Combine existing tags with related video tags
		existingTags := strings.Split(videoUploads[i].Tags, ",")
		allTags := make([]string, 0, len(existingTags)+len(relatedVideoTags))

		// Add existing tags
		for _, tag := range existingTags {
			if tag = strings.TrimSpace(tag); tag != "" {
				allTags = append(allTags, tag)
			}
		}

		// Add related video tags
		for _, tag := range relatedVideoTags {
			if tag = strings.TrimSpace(tag); tag != "" {
				allTags = append(allTags, tag)
			}
		}

		// Join all tags back into a string
		videoUploads[i].Tags = strings.Join(allTags, ",")
	}

	return videoUploads, nil
}
