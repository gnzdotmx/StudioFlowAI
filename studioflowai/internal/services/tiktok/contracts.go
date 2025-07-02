package tiktok

import (
	"context"
	"time"
)

// VideoInfo represents a video on TikTok
type VideoInfo struct {
	Title       string
	Description string
	CreateTime  time.Time
}

// Service defines the interface for TikTok API operations
type Service interface {
	// Initialize initializes the service with OAuth configuration
	Initialize(config interface{}) error

	// UploadVideo uploads a video to TikTok
	UploadVideo(ctx context.Context, videoPath string, title string, description string, privacy string, publishTime time.Time) error

	// GetUploadedVideos retrieves the list of videos already uploaded to TikTok
	GetUploadedVideos(ctx context.Context) ([]VideoInfo, error)

	// GetAccessToken returns the current access token
	GetAccessToken() string
}
