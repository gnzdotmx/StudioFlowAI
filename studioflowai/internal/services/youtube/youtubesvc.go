package youtube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Required OAuth scopes for YouTube API
var requiredScopes = []string{
	"https://www.googleapis.com/auth/youtube.readonly",
	"https://www.googleapis.com/auth/youtube.upload",
	"https://www.googleapis.com/auth/youtube.force-ssl",
}

// Service implements the Service interface
type Service struct{}

// InitializeYouTubeService creates a YouTube service client
func (m *Service) InitializeYouTubeService(ctx context.Context, credentialsPath string) (*youtube.Service, error) {
	// Read credentials file
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Create OAuth2 config
	config, err := google.ConfigFromJSON(credentials, requiredScopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth config: %w", err)
	}

	// Initialize token storage
	tokenStorage, err := utils.NewTokenStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token storage: %w", err)
	}

	// Try to load existing token
	token, err := tokenStorage.LoadToken("youtube")
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	// If no token exists or it's expired, get a new one
	if token == nil || !token.Valid() {
		// Set up callback server
		callbackServer := utils.NewOAuthCallbackServer()
		if err := callbackServer.Start(8080); err != nil {
			return nil, fmt.Errorf("failed to start callback server: %w", err)
		}
		defer func() {
			if err := callbackServer.Stop(); err != nil {
				utils.LogWarning("Failed to stop callback server: %v", err)
			}
		}()

		// Set redirect URL to localhost
		config.RedirectURL = "http://localhost:8080"

		// Get auth URL
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		if err := callbackServer.OpenURL(authURL); err != nil {
			return nil, fmt.Errorf("failed to open auth URL: %w", err)
		}

		// Wait for the authorization code
		code := callbackServer.WaitForCode()

		// Exchange authorization code for token
		token, err = config.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
		}

		// Save the new token
		if err := tokenStorage.SaveToken("youtube", token); err != nil {
			utils.LogWarning("Failed to save token: %v", err)
		}
	} else {
		utils.LogInfo("Using existing authorization token")
	}

	// Create YouTube service with token
	service, err := youtube.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token)))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return service, nil
}

// ReadScheduledVideos retrieves all scheduled videos from the channel
func (m *Service) ReadScheduledVideos(ctx context.Context, service *youtube.Service) ([]ScheduledVideo, error) {
	// Verify channel access
	channelsResponse, err := service.Channels.List([]string{"id"}).Mine(true).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}

	if len(channelsResponse.Items) == 0 {
		return nil, fmt.Errorf("no channel found")
	}

	// Get videos using the search API
	searchResponse, err := service.Search.List([]string{"id"}).
		ForMine(true).
		Type("video").
		MaxResults(50).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to search for videos: %w", err)
	}

	if len(searchResponse.Items) == 0 {
		return nil, nil
	}

	// Get detailed video information
	var videoIds []string
	for _, item := range searchResponse.Items {
		videoIds = append(videoIds, item.Id.VideoId)
	}

	// Get detailed video information
	videosResponse, err := service.Videos.List([]string{"snippet", "status", "contentDetails"}).
		Id(videoIds...).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to get video details: %w", err)
	}

	var scheduledVideos []ScheduledVideo
	for _, video := range videosResponse.Items {
		// Only include scheduled videos
		if video.Status.PrivacyStatus == "private" && video.Status.PublishAt != "" {
			scheduledVideos = append(scheduledVideos, ScheduledVideo{
				Title:       video.Snippet.Title,
				PublishAt:   video.Status.PublishAt,
				Description: video.Snippet.Description,
				Privacy:     video.Status.PrivacyStatus,
				VideoID:     video.Id,
			})
		}
	}

	return scheduledVideos, nil
}

// ListScheduledVideos displays the list of scheduled videos
func (m *Service) ListScheduledVideos(videos []ScheduledVideo) error {
	utils.LogInfo("\nScheduled Videos:")
	utils.LogInfo("----------------")

	if len(videos) == 0 {
		utils.LogInfo("No scheduled videos found")
		return nil
	}

	for _, video := range videos {
		utils.LogInfo("Title: %s", video.Title)
		utils.LogInfo("Scheduled for: %s", video.PublishAt)
		utils.LogInfo("Description: %s", video.Description)
		utils.LogInfo("Privacy: %s", video.Privacy)
		utils.LogInfo("Video ID: %s", video.VideoID)
		utils.LogInfo("----------------")
	}

	return nil
}

// parseScheduleTime parses the schedule time string (HH:MM) into hours and minutes
func parseScheduleTime(timeStr string) (int, int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format, expected HH:MM")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %s", parts[0])
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %s", parts[1])
	}

	return hour, minute, nil
}

// convertToHHMMSS converts a timestamp to HH:MM:SS format
func convertToHHMMSS(timestamp string) string {
	// Remove colons
	return strings.ReplaceAll(timestamp, ":", "")
}

// cleanTag removes special characters and converts to lowercase
func cleanTag(tag string) string {
	// Remove leading/trailing spaces
	tag = strings.TrimSpace(tag)
	// Convert to lowercase
	tag = strings.ToLower(tag)
	// Replace special characters
	replacements := map[string]string{
		"á": "a", "é": "e", "í": "i", "ó": "o", "ú": "u",
		"ñ": "n", "ü": "u",
	}
	for old, new := range replacements {
		tag = strings.ReplaceAll(tag, old, new)
	}
	return tag
}

// processTags splits and cleans tags, ensuring YouTube compatibility
func processTags(tags string) []string {
	// Split by comma
	rawTags := strings.Split(tags, ",")
	seenTags := make(map[string]bool)
	var cleanedTags []string

	// Clean each tag and ensure uniqueness
	for _, tag := range rawTags {
		cleaned := cleanTag(tag)
		// Skip empty tags or tags that are too long (YouTube has a limit)
		if cleaned != "" && len(cleaned) <= 30 && !seenTags[cleaned] {
			seenTags[cleaned] = true
			cleanedTags = append(cleanedTags, cleaned)
		}
	}

	// YouTube has a limit on the number of tags
	if len(cleanedTags) > 30 {
		cleanedTags = cleanedTags[:30]
	}

	return cleanedTags
}

// UploadVideo uploads videos to YouTube
func (m *Service) UploadVideo(ctx context.Context, service *youtube.Service, videoUploads []VideoUpload, privacyStatus string, categoryID string, storedShortsPath string) error {
	for _, upload := range videoUploads {
		// Construct the full path to the video file
		videoPath := filepath.Join(storedShortsPath, upload.FileName)

		// Open the video file
		file, err := os.Open(videoPath)
		if err != nil {
			utils.LogWarning("Failed to open video file: %v", err)
			continue
		}
		defer func() {
			if err := file.Close(); err != nil {
				utils.LogWarning("Failed to close video file: %v", err)
			}
		}()

		// Process and clean tags
		cleanedTags := processTags(upload.Tags)

		// Create video insert request
		video := &youtube.Video{
			Snippet: &youtube.VideoSnippet{
				Title:       upload.ShortTitle,
				Description: upload.Description,
				CategoryId:  categoryID,
				Tags:        cleanedTags,
			},
			Status: &youtube.VideoStatus{
				PrivacyStatus: privacyStatus,
				PublishAt:     upload.PublishTime.Format(time.RFC3339),
				MadeForKids:   false,
			},
		}

		// Upload the video
		call := service.Videos.Insert([]string{"snippet", "status"}, video)
		call.NotifySubscribers(false) // Don't notify subscribers for shorts
		response, err := call.Media(file).Do()
		if err != nil {
			utils.LogWarning("Failed to upload video: %v", err)
			continue
		}

		utils.LogInfo("Successfully uploaded video: %s", response.Id)
		utils.LogInfo("\t[%s] %s", upload.PublishTime.Format("2006-01-02 15:04:05"), upload.ShortTitle)

		// If playlist ID is provided, add the video to the playlist
		if upload.PlaylistID != "" {
			playlistItem := &youtube.PlaylistItem{
				Snippet: &youtube.PlaylistItemSnippet{
					PlaylistId: upload.PlaylistID,
					ResourceId: &youtube.ResourceId{
						Kind:    "youtube#video",
						VideoId: response.Id,
					},
				},
			}

			_, err = service.PlaylistItems.Insert([]string{"snippet"}, playlistItem).Do()
			if err != nil {
				utils.LogWarning("Failed to add video to playlist: %v", err)
			} else {
				utils.LogInfo("Added video to playlist: %s", upload.PlaylistID)
			}
		}
	}

	return nil
}

// FindAvailability finds available time slots for video uploads
func (m *Service) FindAvailability(scheduledVideos []ScheduledVideo, shortsData *utils.ShortsData, periodicity int, scheduleTime string, maxAttempts int, startDate string, playlistID string) ([]VideoUpload, error) {
	// Parse the schedule time
	scheduleHour, scheduleMinute, err := parseScheduleTime(scheduleTime)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule time format: %w", err)
	}

	// Parse the start date
	startDateTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date format: %w", err)
	}
	startDateTime = startDateTime.UTC()

	// Get current time in UTC
	now := time.Now().UTC()

	// Use the later of start date or current time as the reference point
	referenceTime := now
	if startDateTime.After(now) {
		referenceTime = startDateTime
	}

	// Create a map of scheduled times
	scheduledTimes := make(map[time.Time]bool)
	for _, video := range scheduledVideos {
		publishTime, err := time.Parse(time.RFC3339, video.PublishAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse video publish time: %w", err)
		}
		// Convert to UTC
		publishTime = publishTime.UTC()
		scheduledTimes[publishTime] = true
	}

	// Find available times for each short
	var videoUploads []VideoUpload
	lastScheduledTime := time.Time{}

	for _, short := range shortsData.Shorts {
		// Find next available time for this short
		found := false
		attempts := 0

		for !found && attempts < maxAttempts {
			// Calculate the target date based on periodicity
			var targetDate time.Time
			if lastScheduledTime.IsZero() {
				// For the first video, start from the reference time
				targetDate = referenceTime
			} else {
				// For subsequent videos, add periodicity days from the last scheduled time
				targetDate = lastScheduledTime.AddDate(0, 0, periodicity)
			}

			// Create potential publish time in UTC
			publishTime := time.Date(
				targetDate.Year(),
				targetDate.Month(),
				targetDate.Day(),
				scheduleHour,
				scheduleMinute,
				0,
				0,
				time.UTC,
			)

			// Skip if the time is in the past
			if publishTime.Before(now) {
				attempts++
				continue
			}

			// Check if the time is available
			if !scheduledTimes[publishTime] {
				// Create video upload information
				videoUpload := VideoUpload{
					FileName:       fmt.Sprintf("%s-%s-withtext.mp4", convertToHHMMSS(short.StartTime), convertToHHMMSS(short.EndTime)),
					ShortTitle:     short.ShortTitle,
					Description:    short.Description,
					PublishTime:    publishTime,
					PlaylistID:     playlistID,
					Tags:           short.Tags,
					RelatedVideoID: shortsData.SourceVideo,
				}
				videoUploads = append(videoUploads, videoUpload)
				scheduledTimes[publishTime] = true // Mark this time as scheduled
				lastScheduledTime = publishTime
				found = true
			} else {
				// If this time is not available, try the next periodicity interval
				attempts++
				referenceTime = referenceTime.AddDate(0, 0, periodicity)
			}
		}

		if !found {
			// If we couldn't find a slot within the periodicity, try to find any available slot
			utils.LogWarning("Could not find a slot within periodicity for short: %s. Looking for any available slot...", short.ShortTitle)

			// Try to find any available slot in the next maxAttempts days
			for i := 0; i < maxAttempts; i++ {
				publishTime := time.Date(
					referenceTime.Year(),
					referenceTime.Month(),
					referenceTime.Day(),
					scheduleHour,
					scheduleMinute,
					0,
					0,
					time.UTC,
				)

				if !publishTime.Before(now) && !scheduledTimes[publishTime] {
					// Create video upload information
					videoUpload := VideoUpload{
						FileName:       fmt.Sprintf("%s-%s-withtext.mp4", convertToHHMMSS(short.StartTime), convertToHHMMSS(short.EndTime)),
						ShortTitle:     short.ShortTitle,
						Description:    short.Description,
						PublishTime:    publishTime,
						PlaylistID:     playlistID,
						Tags:           short.Tags,
						RelatedVideoID: shortsData.SourceVideo,
					}
					videoUploads = append(videoUploads, videoUpload)
					scheduledTimes[publishTime] = true
					lastScheduledTime = publishTime
					found = true
					break
				}
				referenceTime = referenceTime.AddDate(0, 0, periodicity)
			}
		}

		if !found {
			return nil, fmt.Errorf("no available time found for short: %s after %d days of searching", short.ShortTitle, maxAttempts)
		}
	}

	return videoUploads, nil
}

// ListAvailableTimes displays the list of available time slots
func (m *Service) ListAvailableTimes(videoUploads []VideoUpload) error {
	utils.LogInfo("\nScheduled publish times (UTC):")
	utils.LogInfo("----------------")
	for _, upload := range videoUploads {
		utils.LogInfo("Short: %s", upload.ShortTitle)
		utils.LogInfo("Description: %s", upload.Description)
		utils.LogInfo("File: %s", upload.FileName)
		utils.LogInfo("Publish at: %s", upload.PublishTime.Format(time.RFC3339))
		utils.LogInfo("Playlist: %s", upload.PlaylistID)
		utils.LogInfo("----------------")
	}
	return nil
}

// GetVideoDetails retrieves details of a specific video
func (m *Service) GetVideoDetails(ctx context.Context, service *youtube.Service, videoID string) (*youtube.Video, error) {
	// Get video details using the videos API
	videoResponse, err := service.Videos.List([]string{"snippet"}).Id(videoID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get video details: %w", err)
	}

	if len(videoResponse.Items) == 0 {
		return nil, fmt.Errorf("no video found with ID: %s", videoID)
	}

	return videoResponse.Items[0], nil
}
