package youtube

import (
	"time"
)

// ShortsData represents the structure of the shorts_suggestions.yaml file
type ShortsData struct {
	SourceVideo string      `yaml:"sourceVideo"`
	Shorts      []ShortClip `yaml:"shorts"`
}

// ShortClip represents a single short video clip
type ShortClip struct {
	Title       string `yaml:"title"`
	StartTime   string `yaml:"startTime"`
	EndTime     string `yaml:"endTime"`
	Description string `yaml:"description"`
	Tags        string `yaml:"tags"`
	ShortTitle  string `yaml:"short_title"`
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
