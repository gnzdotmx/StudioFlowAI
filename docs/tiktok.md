# TikTok Integration

## Overview
The TikTok Integration module provides functionality to automatically upload and schedule TikTok videos with tags, descriptions, and related video integration. This module is designed to work seamlessly with the video processing pipeline to create and manage TikTok content.

## Setup Instructions

### 1. TikTok Developer Account Setup
1. Visit [TikTok for Developers](https://developers.tiktok.com/)
2. Click "Manage Apps" in the top right corner
3. Sign in with your TikTok account or create one if needed
4. Complete the developer verification process if required

### 2. Create a TikTok App
1. In the TikTok Developer Portal:
   - Click "Create App" button
   - Select "Web" as the platform
   - Choose "Sandbox" mode for testing
   - Fill in the required app information:
     * App Name: Choose a descriptive name
     * App Description: Brief description of your app
     * App Icon: Upload an icon if desired

2. Configure App Settings:
   - Under "Platform" tab:
     * Add Redirect URI: `http://localhost:8080/callback`
   - Under "Scopes" tab, enable these permissions:
     * `user.info.basic`
     * `video.upload`
     * `video.list`
     * `video.publish`

3. Get API Credentials:
   - From the app dashboard, copy:
     * Client Key
     * Client Secret

### 3. Environment Setup
1. If .env is not present, create a `~/.studioflowai/.env` file in your project root:
```bash
touch ~/.studioflowai/.env
```

2. Add your TikTok credentials to `~/.studioflowai/.env`:
```
TIKTOK_CLIENT_KEY=your_client_key_here
TIKTOK_CLIENT_SECRET=your_client_secret_here
```

### 4. Verify Setup
1. Run a test upload:
```bash
./studioflowaibin run -w examples/gnz_interviews_tiktok.yaml -i tests/output/shorts_suggestions.yaml
```

2. The first run will:
   - Open your browser for TikTok authorization
   - Ask you to log in to TikTok if needed
   - Request permission for the specified scopes
   - Store the access token in `~/.studioflowai/tiktok_token.json`

### Troubleshooting
- If you get "client_key" errors:
  * Verify your Client Key in `.env`
  * Check that all required scopes are enabled
  * Ensure redirect URI matches exactly
- If authorization fails:
  * Delete `~/.studioflowai/tiktok_token.json` to force reauthorization
  * Check your app is in the correct mode (Sandbox/Production)
  * Verify your TikTok account has the necessary permissions

## Prerequisites
- TikTok Developer Account
- TikTok API credentials
- TikTok Business Account
- Video content in the correct format (vertical video, 9:16 aspect ratio)


## Configuration
The module is configured through the workflow YAML file:

```yaml
- name: UploadTikTokShorts
  module: tiktok
  params:
    input: ${output}/shorts_suggestions.yaml
    output: ${output}/tiktok_uploads
    storedShortsPath: ${output}/shorts
    credentials: ${config}/tiktok_credentials.json
    privacyStatus: "public"  # Options: public, private, draft
    scheduleTime: "15:00"    # UTC time in HH:MM format
    maxAttempts: 3
    startDate: "2024-03-20"  # YYYY-MM-DD format
    relatedVideoID: "video_id"  # Optional: ID of related video
```

### Parameters
- `input`: Path to the shorts suggestions YAML file
- `output`: Directory for storing upload logs and metadata
- `storedShortsPath`: Path where processed short videos are stored
- `credentials`: Path to TikTok API credentials file
- `privacyStatus`: Video privacy setting
- `scheduleTime`: UTC time for scheduled uploads
- `maxAttempts`: Maximum number of upload retry attempts
- `startDate`: Date to start scheduling uploads
- `relatedVideoID`: Optional ID of a related video for cross-promotion

## Features

### Video Upload
- Automatic video upload to TikTok
- Support for scheduled publishing
- Privacy status management
- Tag and description handling
- Related video integration

### Scheduling
- Flexible scheduling options
- UTC time zone support
- Automatic date progression
- Conflict resolution

### Content Management
- Tag management
- Description formatting
- Related video linking
- File organization

## Processing Flow
1. **Initialization**
   - Load TikTok API credentials
   - Initialize TikTok service client
   - Validate input parameters

2. **Content Processing**
   - Read shorts suggestions file
   - Process video metadata
   - Prepare upload information

3. **Scheduling**
   - Calculate available time slots
   - Schedule uploads
   - Handle time zone conversions

4. **Upload**
   - Upload video files
   - Set metadata (title, description, tags)
   - Configure privacy settings
   - Handle related video linking

## Error Handling
The module implements comprehensive error handling for:
- Authentication failures
- Upload failures
- Scheduling conflicts
- File access issues
- API rate limits
- Network problems

## Logging
The module provides detailed logging for:
- Upload operations
- Scheduling decisions
- Error conditions
- API interactions
- File operations

## Security
- Secure credential handling
- API key protection
- Input validation
- Output sanitization

## Best Practices
1. **Video Preparation**
   - Use vertical format (9:16)
   - Optimize file size
   - Include engaging thumbnails
   - Add appropriate tags

2. **Scheduling**
   - Consider time zones
   - Space uploads appropriately
   - Monitor upload status
   - Handle failures gracefully

3. **Content Management**
   - Use descriptive titles
   - Include relevant tags
   - Write engaging descriptions
   - Link related content

## Troubleshooting
Common issues and solutions:

1. **Upload Failures**
   - Check file format
   - Verify credentials
   - Check network connection
   - Review API limits

2. **Scheduling Issues**
   - Verify time zone settings
   - Check date format
   - Review schedule conflicts
   - Validate time format

3. **API Errors**
   - Check API credentials
   - Verify permissions
   - Review rate limits
   - Check API status

## API Reference
- [TikTok API Documentation](https://developers.tiktok.com/)
- [TikTok Business API](https://developers.tiktok.com/business/)
- [TikTok Content Guidelines](https://developers.tiktok.com/content-guidelines)

## Updates and Maintenance
- Regular API compatibility checks
- Security updates
- Feature enhancements
- Bug fixes
- Performance optimizations 