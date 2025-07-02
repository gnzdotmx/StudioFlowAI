package youtube

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/services/youtube"
	youtubemocks "github.com/gnzdotmx/studioflowai/studioflowai/internal/services/youtube/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	youtubeapi "google.golang.org/api/youtube/v3"
)

func TestModule_Name(t *testing.T) {
	module := New()
	assert.Equal(t, "uploadyoutubeshorts", module.Name())
}

func TestModule_Validate(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "youtube_validate_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create test files
	testYamlFile := filepath.Join(tempDir, "test.yaml")
	testCredentialsFile := filepath.Join(tempDir, "credentials.json")
	testShortsPath := filepath.Join(tempDir, "shorts")

	// Create test files
	if err := os.WriteFile(testYamlFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testCredentialsFile, []byte("test credentials"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(testShortsPath, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				"input":            testYamlFile,
				"output":           tempDir,
				"storedShortsPath": testShortsPath,
				"credentials":      testCredentialsFile,
				"privacyStatus":    "private",
			},
			wantErr: false,
		},
		{
			name: "missing required input",
			params: map[string]interface{}{
				"output":           tempDir,
				"storedShortsPath": testShortsPath,
				"credentials":      testCredentialsFile,
			},
			wantErr: true,
		},
		{
			name: "missing required storedShortsPath",
			params: map[string]interface{}{
				"input":       testYamlFile,
				"output":      tempDir,
				"credentials": testCredentialsFile,
			},
			wantErr: true,
		},
		{
			name: "missing required credentials",
			params: map[string]interface{}{
				"input":            testYamlFile,
				"output":           tempDir,
				"storedShortsPath": testShortsPath,
			},
			wantErr: true,
		},
		{
			name: "invalid privacy status",
			params: map[string]interface{}{
				"input":            testYamlFile,
				"output":           tempDir,
				"storedShortsPath": testShortsPath,
				"credentials":      testCredentialsFile,
				"privacyStatus":    "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := New()
			err := module.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestModule_Execute(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "youtube_execute_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create test files
	testYamlFile := filepath.Join(tempDir, "test.yaml")
	testCredentialsFile := filepath.Join(tempDir, "credentials.json")
	testShortsPath := filepath.Join(tempDir, "shorts")

	// Create test files with valid YAML content
	testYamlContent := `shorts:
  - title: "Test Short"
    description: "Test Description"
    tags: "test,tags"
    duration: 60
`
	if err := os.WriteFile(testYamlFile, []byte(testYamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testCredentialsFile, []byte("test credentials"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(testShortsPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mock YouTube service
	mockService := youtubemocks.NewMockYouTubeService(t)
	mockYouTubeService := &youtubeapi.Service{}

	// Set up mock expectations
	mockService.On("InitializeYouTubeService", mock.Anything, testCredentialsFile).Return(mockYouTubeService, nil)
	mockService.On("ReadScheduledVideos", mock.Anything, mockYouTubeService).Return([]youtube.ScheduledVideo{}, nil)
	mockService.On("FindAvailability", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]youtube.VideoUpload{
		{
			FileName:    "test.mp4",
			ShortTitle:  "Test Video",
			Description: "Test Description",
			PublishTime: time.Now(),
			Tags:        "test,tags",
		},
	}, nil)
	mockService.On("ListAvailableTimes", mock.Anything).Return(nil)
	mockService.On("UploadVideo", mock.Anything, mockYouTubeService, mock.Anything, "private", "", testShortsPath).Return(nil)

	// Create module with mock service
	module := &Module{
		youtubeService: mockService,
	}

	// Test parameters
	params := map[string]interface{}{
		"input":            testYamlFile,
		"output":           tempDir,
		"storedShortsPath": testShortsPath,
		"credentials":      testCredentialsFile,
		"privacyStatus":    "private",
	}

	// Execute module
	result, err := module.Execute(context.Background(), params)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.Statistics["uploadedVideos"])
	assert.Equal(t, 60, result.Statistics["scheduleSpan"])
	assert.Contains(t, result.Outputs, "uploadStatus")

	// Verify mock expectations
	mockService.AssertExpectations(t)
}

func TestModule_GetIO(t *testing.T) {
	module := New()
	io := module.GetIO()

	// Verify required inputs
	assert.Len(t, io.RequiredInputs, 3)
	assert.Equal(t, "input", io.RequiredInputs[0].Name)
	assert.Equal(t, "storedShortsPath", io.RequiredInputs[1].Name)
	assert.Equal(t, "credentials", io.RequiredInputs[2].Name)

	// Verify optional inputs
	assert.Len(t, io.OptionalInputs, 5)
	optionalInputNames := []string{"playlistId", "privacyStatus", "categoryId", "scheduleTime", "relatedVideoId"}
	for i, name := range optionalInputNames {
		assert.Equal(t, name, io.OptionalInputs[i].Name)
	}

	// Verify produced outputs
	assert.Len(t, io.ProducedOutputs, 1)
	assert.Equal(t, "uploadStatus", io.ProducedOutputs[0].Name)
}

func TestModule_CollectTagsAndRelatedVideo(t *testing.T) {
	// Create mock YouTube service
	mockService := youtubemocks.NewMockYouTubeService(t)
	mockYouTubeService := &youtubeapi.Service{}

	// Create test data
	videoUploads := []youtube.VideoUpload{
		{
			FileName:    "test.mp4",
			ShortTitle:  "Test Video",
			Description: "Test Description",
			PublishTime: time.Now(),
			Tags:        "test,tags",
		},
	}

	// Create module
	module := &Module{
		youtubeService: mockService,
	}

	// Test with no related video ID
	result, err := module.collectTagsAndRelatedVideo(mockYouTubeService, videoUploads, "")
	assert.NoError(t, err)
	assert.Equal(t, videoUploads, result)

	// Test with related video ID
	// Set up mock expectations for the related video lookup
	mockService.On("GetVideoDetails", mock.Anything, mockYouTubeService, "test-video-id").Return(&youtubeapi.Video{
		Snippet: &youtubeapi.VideoSnippet{
			Tags: []string{"related", "tags"},
		},
	}, nil)

	// Test with related video ID
	result, err = module.collectTagsAndRelatedVideo(mockYouTubeService, videoUploads, "test-video-id")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "test-video-id", result[0].RelatedVideoID)
	assert.Contains(t, result[0].Tags, "related")
	assert.Contains(t, result[0].Tags, "tags")

	// Verify mock expectations
	mockService.AssertExpectations(t)
}
