package tiktok

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/services/tiktok"
	tiktokmocks "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/tiktok/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUploadTikTokShortsModule_Name(t *testing.T) {
	module := NewUploadTikTokShorts()
	assert.Equal(t, "uploadtiktokshorts", module.Name())
}

type testShort struct {
	ShortTitle  string `yaml:"shortTitle"`
	StartTime   string `yaml:"startTime"`
	EndTime     string `yaml:"endTime"`
	Description string `yaml:"description"`
	Tags        string `yaml:"tags"`
}

type testShortsData struct {
	SourceVideo string      `yaml:"sourceVideo"`
	Shorts      []testShort `yaml:"shorts"`
}

func setupTestFiles(t *testing.T) (string, string, func()) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "tiktok_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test input file
	inputPath := filepath.Join(tempDir, "test_input.yaml")
	testData := testShortsData{
		SourceVideo: "test.mp4",
		Shorts: []testShort{
			{
				ShortTitle:  "Test Short 1",
				StartTime:   "00:00:00",
				EndTime:     "00:00:03",
				Description: "Test Description 1",
				Tags:        "test,video",
			},
			{
				ShortTitle:  "Test Short 2",
				StartTime:   "00:00:04",
				EndTime:     "00:00:07",
				Description: "Test Description 2",
				Tags:        "test,video",
			},
		},
	}

	// Create YAML content
	yamlContent := fmt.Sprintf(`sourceVideo: %s
shorts:
  - shortTitle: "%s"
    startTime: "%s"
    endTime: "%s"
    description: "%s"
    tags: "%s"
  - shortTitle: "%s"
    startTime: "%s"
    endTime: "%s"
    description: "%s"
    tags: "%s"
`,
		testData.SourceVideo,
		testData.Shorts[0].ShortTitle,
		testData.Shorts[0].StartTime,
		testData.Shorts[0].EndTime,
		testData.Shorts[0].Description,
		testData.Shorts[0].Tags,
		testData.Shorts[1].ShortTitle,
		testData.Shorts[1].StartTime,
		testData.Shorts[1].EndTime,
		testData.Shorts[1].Description,
		testData.Shorts[1].Tags,
	)

	if err := os.WriteFile(inputPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test input file: %v", err)
	}

	// Create shorts directory
	shortsPath := filepath.Join(tempDir, "shorts")
	if err := os.MkdirAll(shortsPath, 0755); err != nil {
		t.Fatalf("Failed to create shorts directory: %v", err)
	}

	// Create dummy video files
	for _, short := range testData.Shorts {
		videoName := fmt.Sprintf("%s-%s-withtext.mp4",
			convertTimeFormat(short.StartTime),
			convertTimeFormat(short.EndTime))
		videoPath := filepath.Join(shortsPath, videoName)
		if err := os.WriteFile(videoPath, []byte("dummy video data"), 0644); err != nil {
			t.Fatalf("Failed to create test video file: %v", err)
		}
	}

	cleanup := func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to cleanup test directory: %v", err)
		}
	}

	return inputPath, shortsPath, cleanup
}

func TestUploadTikTokShortsModule_Validate(t *testing.T) {
	inputPath, shortsPath, cleanup := setupTestFiles(t)
	defer cleanup()

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid params",
			params: map[string]interface{}{
				"input":            inputPath,
				"output":           "test_output",
				"storedShortsPath": shortsPath,
				"privacyStatus":    "private",
			},
			wantErr: false,
		},
		{
			name: "invalid privacy status",
			params: map[string]interface{}{
				"input":            inputPath,
				"output":           "test_output",
				"storedShortsPath": shortsPath,
				"privacyStatus":    "invalid",
			},
			wantErr: true,
		},
		{
			name: "missing required params",
			params: map[string]interface{}{
				"input":            "",
				"output":           "",
				"storedShortsPath": "",
				"privacyStatus":    "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewUploadTikTokShorts()
			err := m.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUploadTikTokShortsModule_Execute_ServiceError(t *testing.T) {
	inputPath, shortsPath, cleanup := setupTestFiles(t)
	defer cleanup()

	// Create mock service
	mockService := tiktokmocks.NewMockService(t)

	// Mock Initialize method
	mockService.On("Initialize", mock.MatchedBy(func(config interface{}) bool {
		oauthConfig, ok := config.(tiktok.OAuthConfig)
		return ok && oauthConfig.RedirectURI == "http://localhost:8080/callback"
	})).Return(nil)

	// Mock UploadVideo method
	mockService.On("UploadVideo",
		mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Time"),
	).Return(fmt.Errorf("upload failed"))

	// Create module with mock service
	module := NewUploadTikTokShortsWithService(func() (tiktok.Service, error) {
		return mockService, nil
	})

	// Execute module
	params := map[string]interface{}{
		"input":            inputPath,
		"output":           "test_output",
		"storedShortsPath": shortsPath,
		"privacyStatus":    "private",
	}

	_, err := module.Execute(context.Background(), params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upload failed")

	// Verify all expectations were met
	mockService.AssertExpectations(t)
}

func TestUploadTikTokShortsModule_Execute_Success(t *testing.T) {
	inputPath, shortsPath, cleanup := setupTestFiles(t)
	defer cleanup()

	// Create mock service
	mockService := tiktokmocks.NewMockService(t)

	// Mock Initialize method
	mockService.On("Initialize", mock.MatchedBy(func(config interface{}) bool {
		oauthConfig, ok := config.(tiktok.OAuthConfig)
		return ok && oauthConfig.RedirectURI == "http://localhost:8080/callback"
	})).Return(nil)

	// Mock UploadVideo method
	mockService.On("UploadVideo",
		mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Time"),
	).Return(nil)

	// Create module with mock service
	module := NewUploadTikTokShortsWithService(func() (tiktok.Service, error) {
		return mockService, nil
	})

	// Execute module
	params := map[string]interface{}{
		"input":            inputPath,
		"output":           "test_output",
		"storedShortsPath": shortsPath,
		"privacyStatus":    "private",
	}

	result, err := module.Execute(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Outputs, "uploadStatus")

	// Verify all expectations were met
	mockService.AssertExpectations(t)
}

// Helper function to convert time format
func convertTimeFormat(timestamp string) string {
	return strings.ReplaceAll(timestamp, ":", "")
}
