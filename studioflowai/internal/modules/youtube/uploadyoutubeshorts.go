package youtube

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	modules "github.com/gnzdotmx/studioflowai/studioflowai/internal/mod"
	youtubesvc "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/youtube"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"google.golang.org/api/youtube/v3"
)

// Module implements YouTube shorts upload functionality
type Module struct {
	youtubeService youtubesvc.YouTubeService
}

// Params contains the parameters for YouTube shorts upload operations
type Params struct {
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

// New creates a new YouTube shorts upload module
func New() modules.Module {
	return &Module{
		youtubeService: &youtubesvc.Service{},
	}
}

// Name returns the module name
func (m *Module) Name() string {
	return "uploadyoutubeshorts"
}

// Validate checks if the parameters are valid
func (m *Module) Validate(params map[string]interface{}) error {
	var p Params
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

	// Validate storedShortsPath
	if p.StoredShortsPath == "" {
		return fmt.Errorf("storedShortsPath is required")
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
func (m *Module) Execute(ctx context.Context, params map[string]interface{}) (modules.ModuleResult, error) {
	var p Params
	if err := modules.ParseParams(params, &p); err != nil {
		return modules.ModuleResult{}, err
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
		return modules.ModuleResult{}, fmt.Errorf("failed to expand home directory: %w", err)
	}
	p.Credentials = expandedCredentials

	// Initialize YouTube service
	service, err := m.youtubeService.InitializeYouTubeService(ctx, p.Credentials)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to initialize YouTube service: %w", err)
	}

	// Read and list scheduled videos
	scheduledVideos, err := m.youtubeService.ReadScheduledVideos(ctx, service)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to read scheduled videos: %w", err)
	}

	// Read shorts suggestions file
	shortsData, err := utils.ReadShortsFile(p.Input)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to read shorts suggestions file: %w", err)
	}

	// Find available times for each short
	videoUploads, err := m.youtubeService.FindAvailability(scheduledVideos, shortsData, p.SchedulePeriodicity, p.ScheduleTime, p.MaxAttempts, p.StartDate, p.PlaylistID)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to find availability: %w", err)
	}

	// Collect tags and related video ID
	videoUploads, err = m.collectTagsAndRelatedVideo(service, videoUploads, p.RelatedVideoID)
	if err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to collect tags and related video: %w", err)
	}

	// List available times
	if err := m.youtubeService.ListAvailableTimes(videoUploads); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to list available times: %w", err)
	}

	// Upload the videos
	if err := m.youtubeService.UploadVideo(ctx, service, videoUploads, p.PrivacyStatus, p.CategoryID, p.StoredShortsPath); err != nil {
		return modules.ModuleResult{}, fmt.Errorf("failed to upload videos: %w", err)
	}

	// Prepare result
	result := modules.ModuleResult{
		Outputs: map[string]string{
			"uploadStatus": fmt.Sprintf("%s/youtube_upload_status.json", p.Output),
		},
		Metadata: map[string]interface{}{
			"totalVideos": len(videoUploads),
			"startDate":   p.StartDate,
			"endDate":     time.Now().UTC().Format("2006-01-02"),
		},
		Statistics: map[string]interface{}{
			"uploadedVideos": len(videoUploads),
			"scheduleSpan":   p.MaxAttempts,
		},
		NextModules: []string{}, // No next modules for this terminal operation
	}

	return result, nil
}

// collectTagsAndRelatedVideo adds tags from the related video and adds related video ID to the video uploads
func (m *Module) collectTagsAndRelatedVideo(service *youtube.Service, videoUploads []youtubesvc.VideoUpload, relatedVideoID string) ([]youtubesvc.VideoUpload, error) {
	// If no related video ID is provided, just return the uploads as is
	if relatedVideoID == "" {
		return videoUploads, nil
	}

	// Get the related video details to extract tags
	video, err := m.youtubeService.GetVideoDetails(context.Background(), service, relatedVideoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get related video details: %w", err)
	}

	if video == nil || video.Snippet == nil {
		return nil, fmt.Errorf("no video found with ID: %s", relatedVideoID)
	}

	// Get tags from the related video
	relatedVideoTags := video.Snippet.Tags
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

// GetIO returns the module's input/output specification
func (m *Module) GetIO() modules.ModuleIO {
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
			{
				Name:        "credentials",
				Description: "Path to Google credentials file",
				Patterns:    []string{"*.json"},
				Type:        string(modules.InputTypeFile),
			},
		},
		OptionalInputs: []modules.ModuleInput{
			{
				Name:        "playlistId",
				Description: "YouTube playlist ID",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "privacyStatus",
				Description: "Video privacy status (private, unlisted, public)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "categoryId",
				Description: "Video category ID",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "scheduleTime",
				Description: "Time to schedule videos (24-hour format)",
				Type:        string(modules.InputTypeData),
			},
			{
				Name:        "relatedVideoId",
				Description: "ID of the related video to link with shorts",
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
