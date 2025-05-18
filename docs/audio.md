# 🎵 Audio Processing Modules

## Overview
The Audio Processing modules provide comprehensive audio extraction, transcription, and formatting capabilities for video content.

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

2. **Whisper Installation**
   ```bash
   # Install Whisper
   pip install -U openai-whisper

   # Install additional dependencies
   pip install setuptools-rust
   ```

## ⚙️ Configuration

### 1. Extract Module
```yaml
name: Extract Audio
description: Extract audio from video files

steps:
  - name: Extract Audio
    module: extract
    parameters:
      input: "./input/video.mp4"
      outputName: "audio.wav"
      format: "wav"        # Optional: wav, mp3, m4a
      sampleRate: 44100    # Optional: 44100, 48000
      channels: 2          # Optional: 1 (mono), 2 (stereo)
```

### 2. Transcribe Module
```yaml
name: Transcribe Audio
description: Convert audio to text using Whisper

steps:
  - name: Transcribe
    module: transcribe
    parameters:
      input: "${output}/audio.wav"
      model: "whisper"     # Optional: whisper, whisper-large
      language: "en"       # Optional: auto-detect if not specified
      outputFormat: "srt"  # Optional: srt, txt, json
```

### 3. Format Module
```yaml
name: Format Transcription
description: Clean and format transcription

steps:
  - name: Format
    module: format
    parameters:
      input: "${output}/transcript.srt"
      output: "${output}/transcript_clean.txt"
      format: "text"       # Optional: text, srt, json
      removeTimestamps: true
      removeSpeakerLabels: true
```

## 📋 Features

### Extract Module
- Multiple audio format support
- Customizable sample rate
- Channel configuration
- Quality preservation
- Batch processing

### Transcribe Module
- Multiple model options
- Language detection
- Timestamp generation
- Speaker diarization
- Format conversion

### Format Module
- Multiple output formats
- Timestamp removal
- Speaker label handling
- Text cleaning
- Format standardization

## 🔄 Processing Flow

1. **Audio Extraction**
   - Video file validation
   - Audio stream extraction
   - Format conversion
   - Quality preservation
   - Output generation

2. **Transcription**
   - Audio preprocessing
   - Model selection
   - Speech recognition
   - Timestamp generation
   - Format conversion

3. **Formatting**
   - Input parsing
   - Text cleaning
   - Format conversion
   - Quality checks
   - Output generation

## 🚨 Error Handling

The modules include comprehensive error handling for:
- File access issues
- Format compatibility
- Processing failures
- Resource limitations
- Invalid parameters

## 📝 Logging

- Processing status
- Error reporting
- Performance metrics
- Quality checks
- Success confirmations

## 🔒 Security

- Input validation
- Output sanitization
- Resource limits
- Error masking
- File permissions

## 🎯 Best Practices

1. **Audio Quality**
   - Use appropriate sample rates
   - Choose correct channels
   - Maintain format quality
   - Consider file size

2. **Transcription**
   - Select appropriate model
   - Use language detection
   - Consider noise reduction
   - Validate output

3. **Formatting**
   - Choose appropriate format
   - Clean text properly
   - Maintain readability
   - Preserve important information

## 🔍 Troubleshooting

### Common Issues

1. **Extraction Issues**
   - Check file permissions
   - Verify format support
   - Check disk space
   - Review FFmpeg installation

2. **Transcription Issues**
   - Verify Whisper installation
   - Check audio quality
   - Review model selection
   - Check language support

3. **Format Issues**
   - Verify input format
   - Check output path
   - Review formatting options
   - Validate output

## 📚 References

- [FFmpeg Documentation](https://ffmpeg.org/documentation.html)
- [Whisper Documentation](https://github.com/openai/whisper)
- [Audio Format Specifications](https://en.wikipedia.org/wiki/Audio_file_format)

## 🔄 Updates and Maintenance

- Regular FFmpeg updates
- Whisper model updates
- Format support updates
- Performance optimization
- Documentation updates 