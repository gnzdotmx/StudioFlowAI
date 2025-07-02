package youtube

import (
	"context"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"
	"google.golang.org/api/youtube/v3"
)

// YouTubeService defines the interface for YouTube service operations
type YouTubeService interface {
	// InitializeYouTubeService creates a YouTube service client
	InitializeYouTubeService(ctx context.Context, credentialsPath string) (*youtube.Service, error)

	// ReadScheduledVideos retrieves all scheduled videos from the channel
	ReadScheduledVideos(ctx context.Context, service *youtube.Service) ([]ScheduledVideo, error)

	// ListScheduledVideos displays the list of scheduled videos
	ListScheduledVideos(videos []ScheduledVideo) error

	// UploadVideo uploads videos to YouTube
	UploadVideo(ctx context.Context, service *youtube.Service, videoUploads []VideoUpload, privacyStatus string, categoryID string, storedShortsPath string) error

	// FindAvailability finds available time slots for video uploads
	FindAvailability(scheduledVideos []ScheduledVideo, shortsData *utils.ShortsData, periodicity int, scheduleTime string, maxAttempts int, startDate string, playlistID string) ([]VideoUpload, error)

	// ListAvailableTimes displays the list of available time slots
	ListAvailableTimes(videoUploads []VideoUpload) error

	// GetVideoDetails retrieves details of a specific video
	GetVideoDetails(ctx context.Context, service *youtube.Service, videoID string) (*youtube.Video, error)
}

// ScheduledVideo represents a scheduled video on YouTube
type ScheduledVideo struct {
	Title       string
	PublishAt   string
	Description string
	Privacy     string
	VideoID     string
}

// VideoUpload represents the information needed to upload a video
type VideoUpload struct {
	FileName       string    // The video file name (HHMMSS-HHMMSS-withtext.mp4 format)
	ShortTitle     string    // The title of the short video
	Description    string    // The description of the video
	PublishTime    time.Time // The scheduled publish time
	PlaylistID     string    // The YouTube playlist ID where the video will be published
	Tags           string    // The tags for the video
	RelatedVideoID string    // The ID of the related video to link with
}
