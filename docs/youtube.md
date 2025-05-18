# 🎥 YouTube Integration Module

## Overview
The YouTube Integration module provides functionality to automatically upload and schedule YouTube Shorts with advanced features like tag management, playlist integration, and scheduling capabilities.

## 🔑 Prerequisites

1. **Google Cloud Project**
   - Create a project in the [Google Cloud Console](https://console.cloud.google.com)
   - Enable the YouTube Data API v3
   - Configure OAuth consent screen

2. **OAuth Credentials**
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - Select "Desktop application" as the application type
   - Download the credentials JSON file

3. **Test Users (if using restricted access)**
   - Add test users in the OAuth consent screen
   - Each test user must have a Google account
   - Test users will need to verify their email

## ⚙️ Configuration

### 1. Environment Setup
```bash
# Set the path to your OAuth credentials
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/credentials.json"
```

### 2. Workflow Configuration
```yaml
name: YouTube Shorts Upload
description: Upload and schedule YouTube Shorts

steps:
  - name: Upload Shorts
    module: uploadyoutubeshorts
    parameters:
      input: "${output}/shorts_suggestions.yaml"
      output: "${output}"
      storedShortsPath: "${output}/shorts_with_text"
      credentials: "${GOOGLE_APPLICATION_CREDENTIALS}"
      playlistId: "YOUR_PLAYLIST_ID"  # Optional
      privacyStatus: "private"        # private, unlisted, or public
      categoryId: "22"                # People & Blogs
      schedulePeriodicity: 1          # Days between uploads
      scheduleTime: "15:00"           # 24-hour format
      maxAttempts: 60                 # Days to search for available slots
      startDate: "2024-03-20"        # YYYY-MM-DD format
      relatedVideoId: "VIDEO_ID"      # Optional: Link to original video
```

## 🔄 OAuth Flow

1. **First Run**
   - The module will open your default browser
   - Sign in with your Google account
   - Grant the requested permissions
   - The token will be stored locally for future use

2. **Token Storage**
   - Tokens are stored securely in `~/.studioflowai/tokens`
   - Automatic token refresh when expired
   - No need to re-authenticate unless tokens are invalid

## 📋 Features

### Video Upload
- Automatic file handling
- Custom titles and descriptions
- Tag management
- Privacy settings
- Category assignment
- Made for Kids flag

### Scheduling
- Flexible scheduling options
- Periodicity control
- Time slot management
- Conflict resolution
- Start date specification

### Playlist Management
- Automatic playlist addition
- Support for multiple playlists
- Playlist item ordering

### Related Video Integration
- Tag inheritance from related videos
- Description linking
- Cross-promotion support

## 🚨 Error Handling

The module includes comprehensive error handling for:
- Authentication failures
- File access issues
- API rate limits
- Network problems
- Invalid parameters

## 📝 Logging

- Detailed operation logs
- Upload status tracking
- Scheduling information
- Error reporting
- Success confirmations

## 🔒 Security

- Secure token storage
- OAuth 2.0 implementation
- No API keys in code
- Environment variable usage
- Minimal permission scope

## 🎯 Best Practices

1. **File Organization**
   - Keep shorts in dedicated directories
   - Use consistent naming conventions
   - Maintain backup copies

2. **Scheduling**
   - Plan uploads during peak hours
   - Consider timezone differences
   - Maintain consistent intervals

3. **Content Management**
   - Use descriptive titles
   - Include relevant tags
   - Write engaging descriptions
   - Link related content

## 🔍 Troubleshooting

### Common Issues

1. **Authentication Errors**
   - Verify credentials file path
   - Check OAuth consent screen
   - Ensure test user status
   - Clear token storage if needed

2. **Upload Failures**
   - Check file permissions
   - Verify file format
   - Ensure sufficient quota
   - Check network connection

3. **Scheduling Issues**
   - Verify time format
   - Check timezone settings
   - Ensure future dates
   - Review periodicity settings

## 📚 API References

- [YouTube Data API v3](https://developers.google.com/youtube/v3)
- [OAuth 2.0 for Desktop Apps](https://developers.google.com/identity/protocols/oauth2/native-app)
- [YouTube API Quotas](https://developers.google.com/youtube/v3/getting-started#quota)

## 🔄 Updates and Maintenance

- Regular token refresh
- API quota monitoring
- Error log review
- Configuration updates
- Security patches 