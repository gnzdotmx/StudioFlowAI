# 📹 Video Processing Modules

## Overview
The Video Processing modules provide functionality for creating short-form video content and adding professional text overlays to videos.

## 🔑 Prerequisites

1. **FFmpeg Installation**
   ```bash
   # macOS
   brew install ffmpeg

   # Ubuntu/Debian
   sudo apt-get install ffmpeg

   # Windows
   # Download from https://ffmpeg.org/download.html
   ```

2. **System Requirements**
   - Sufficient disk space for video processing
   - Adequate RAM for video editing
   - GPU acceleration (optional but recommended)

## ⚙️ Configuration

### 1. Extract Shorts Module
```yaml
name: Extract Shorts
description: Generate short-form video clips

steps:
  - name: Extract Shorts
    module: extractshorts
    parameters:
      input: "./input/video.mp4"
      output: "${output}/shorts"
      suggestions: "${output}/shorts_suggestions.yaml"
      format: "mp4"           # Optional: mp4, mov
      resolution: "1080x1920" # Optional: 1080x1920, 720x1280
      fps: 30                 # Optional: 24, 30, 60
      quality: "high"         # Optional: low, medium, high
```

### 2. Add Text Module
```yaml
name: Add Text Overlay
description: Add professional text overlays to videos

steps:
  - name: Add Text
    module: addtext
    parameters:
      input: "${output}/shorts"
      output: "${output}/shorts_with_text"
      font: "Arial"           # Optional: Arial, Roboto, etc.
      fontSize: 48            # Optional: 24-72
      fontColor: "white"      # Optional: white, black, #FFFFFF
      position: "bottom"      # Optional: top, bottom, center
      backgroundColor: "rgba(0,0,0,0.5)" # Optional: rgba color
      padding: 20             # Optional: padding in pixels
```

## 📋 Features

### Extract Shorts Module
- Multiple format support
- Customizable resolution
- Frame rate control
- Quality settings
- Batch processing
- Timestamp-based extraction
- Audio preservation
- Metadata handling

### Add Text Module
- Multiple font support
- Customizable styling
- Position control
- Background options
- Animation support
- Multi-line text
- Unicode support
- Batch processing

## 🔄 Processing Flow

1. **Shorts Extraction**
   - Video file validation
   - Timestamp parsing
   - Clip extraction
   - Format conversion
   - Quality optimization
   - Output generation

2. **Text Overlay**
   - Video file validation
   - Text processing
   - Style application
   - Position calculation
   - Overlay rendering
   - Output generation

## 🚨 Error Handling

The modules include comprehensive error handling for:
- File access issues
- Format compatibility
- Processing failures
- Resource limitations
- Invalid parameters
- Text rendering issues
- Memory constraints

## 📝 Logging

- Processing status
- Error reporting
- Performance metrics
- Quality checks
- Success confirmations
- Resource usage
- Processing time

## 🔒 Security

- Input validation
- Output sanitization
- Resource limits
- Error masking
- File permissions
- Text sanitization
- Path validation

## 🎯 Best Practices

1. **Video Quality**
   - Use appropriate resolution
   - Choose correct frame rate
   - Maintain aspect ratio
   - Consider file size
   - Preserve audio quality

2. **Text Overlay**
   - Choose readable fonts
   - Use appropriate size
   - Ensure contrast
   - Consider readability
   - Test on different devices

3. **File Management**
   - Organize output structure
   - Use consistent naming
   - Maintain backups
   - Clean temporary files
   - Monitor disk space

## 🔍 Troubleshooting

### Common Issues

1. **Extraction Issues**
   - Check file permissions
   - Verify format support
   - Check disk space
   - Review FFmpeg installation
   - Validate timestamps

2. **Text Overlay Issues**
   - Verify font installation
   - Check text encoding
   - Review position settings
   - Validate color values
   - Check memory usage

3. **Performance Issues**
   - Monitor CPU usage
   - Check GPU acceleration
   - Review batch size
   - Optimize settings
   - Check disk I/O

## 📚 References

- [FFmpeg Documentation](https://ffmpeg.org/documentation.html)
- [Video Format Specifications](https://en.wikipedia.org/wiki/Video_file_format)
- [Text Rendering Best Practices](https://www.w3.org/TR/css-text-3/)

## 🔄 Updates and Maintenance

- Regular FFmpeg updates
- Font library updates
- Format support updates
- Performance optimization
- Documentation updates
- Security patches
- Bug fixes 