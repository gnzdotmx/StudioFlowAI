# 🎥 StudioFlowAI

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)

</div>

## 📝 Overview

StudioFlowAI is a powerful video processing workflow automation tool that helps content creators streamline their video production process. It provides an end-to-end solution for video processing, transcription, content generation, and social media optimization.

<div align="center">
<img src="./docs/media/demo.gif" alt="StudioFlowAI" width="70%" height="70%">
</div>

## ✨ Features

- 🎵 **Audio Extraction**: Convert video to high-quality audio
- 🗣️ **AI Transcription**: Powered by Whisper for accurate speech-to-text
- 📝 **Transcription Formatting**: Clean and format transcriptions automatically
- 🤖 **AI-Powered Correction**: Enhance transcriptions using ChatGPT
- 📱 **Social Media Content**: Generate optimized content to post on social media
- 📹 **Shorts Generation**: Create engaging short-form video content
- 🎨 **Text Overlay**: Add professional text overlays to videos
- 🔄 **Workflow Automation**: Define and execute complex video processing workflows
- 🎥 **YouTube Integration**: Automatically upload and schedule YouTube Shorts with tags, descriptions, and playlist management

## 🚀 Quick Start

### Prerequisites

- 🔸 Go 1.19 or higher
- 🔸 [FFmpeg](https://ffmpeg.org/download.html)
- 🔸 [Whisper](https://github.com/openai/whisper?tab=readme-ov-file#setup)
- 🔸 OpenAI API key (for ChatGPT correction and social media content generation)

For better performance on M chips, you can install Whisper-cli:
- 🔸 [Whisper-cli](https://github.com/ggml-org/whisper.cpp)

### Installation

#### Option 1: Direct Installation (Recommended)

You can install StudioFlowAI directly using Go's install command:

```bash
go install https://github.com/gnzdotmx/StudioFlowAI/studioflowai@latest
```

This will download, build, and install the binary to your `$GOPATH/bin` directory. Make sure this directory is in your system PATH to run the `studioflowai` command from anywhere.

#### Option 2: Manual Build

If you prefer to build from source:

```bash
git clone https://github.com/gnzdotmx/StudioFlowAI.git
cd StudioFlowAI/studioflowai
go build -o studioflowai main.go
# Optional: Move the binary to a directory in your PATH
mv ./studioflowai $GOPATH/bin/
```


### 🔑 Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required for the ChatGPT module)

#### ⚙️ Setting Up Environment Variables

You can set up environment variables using a `.env` file in the root directory of the project or under `~/.studioflowai/` :

1. Create a `.env`:
   ```bash
   mkdir -p ~/.studioflowai
   touch ~/.studioflowai/.env
   ```

2. Add your API tokens and other configuration:
   ```
   OPENAI_API_KEY=your_openai_api_key_here
   # Add any other environment variables as needed
   ```

3. The application will automatically load these variables when it starts. If you prefer to set them manually:
   ```bash
   export OPENAI_API_KEY=your_openai_api_key_here
   ```


### Basic Usage
#### ✅ Validating Your Environment

Before running workflows, you can check if your environment is properly set up:

```bash
studioflowai validate
```

This will verify that all required external tools (like FFmpeg) are installed and that necessary environment variables are set.

#### 🚀 Running a Workflow

To run a workflow defined in a YAML file:

```bash
# Run with the input path defined in the workflow file
studioflowai run -w path/to/workflow.yaml

# Override the input directory or file
studioflowai run -w path/to/workflow.yaml -i /path/to/input/video.mp4
```

Each workflow run creates a timestamped subfolder within the output directory specified in the workflow file. For example, if your workflow output is set to `./output`, the results will be stored in a folder like `./output/Complete_Video_Processing_Workflow-20231015-120530/`.

### ♻️ Retrying Failed Workflows

If a workflow fails during execution (e.g., because it couldn't find a prompt template), you can retry it from the point of failure:

```bash
# Retry a failed workflow
studioflowai run -w path/to/workflow.yaml --retry --output-folder ./output/Complete_Video_Processing_Workflow-20231015-120530 --workflow-name "Step Name"
```

The retry functionality will:
1. Use the same output folder from the previous run
2. Resume execution from the specified step
3. Continue with the remaining steps in the workflow

### 🧹 Cleaning Up Old Workflow Runs

You can clean up old workflow run directories with the cleanup command:

```bash
# Delete all workflow run directories older than 30 days
studioflowai cleanup -d ./output --older-than 30

# Keep only the 5 most recent run directories
studioflowai cleanup -d ./output --keep-latest 5

# See what would be deleted without actually deleting (dry run)
studioflowai cleanup -d ./output --older-than 7 --dry-run
```

## 📋 Workflow Configuration

StudioFlowAI uses YAML configuration files to define processing workflows. Here's an example of a workflow:

```yaml
name: Complete Video Processing Workflow
description: Extract audio, split into segments, transcribe, format and correct transcription

steps:
  - name: Extract Audio
    module: extract
    parameters:
      input: ./input/video.mp4
      outputName: "audio.wav"
      
  - name: Transcribe Audio
    module: transcribe
    parameters:
      input: "${output}/audio.wav"
      model: "whisper"
```

For more examples, check the [examples folder](examples).

## 🛠️ Modules

### Audio Processing
- **Extract**: Convert video to audio
- **Transcribe**: Speech-to-text using Whisper
- **Format**: Clean and format transcriptions

### AI Integration
- **ChatGPT**: Enhance and correct transcriptions
- **SNS**: Generate social media content
- **Shorts**: Create short-form video suggestions

### Video Processing
- **ExtractShorts**: Generate video clips
- **AddText**: Add text overlays to videos

### YouTube Integration
- **UploadYouTubeShorts**: Automatically upload and schedule YouTube Shorts with tags, descriptions, and playlist management

### TikTok Integration
- **UploadTikTokShorts**: Automatically upload and schedule TikTok videos with tags, descriptions, and related video integration

> 📚 For detailed documentation of each module, including setup instructions, configuration options, and best practices, please refer to the [./docs](./docs) folder.

### Output Structure

```
output/
├── Complete_Video_Processing_Workflow-YYYYMMDD-HHMMSS/
│   ├── audio.wav
│   ├── transcript.srt
│   ├── transcript_clean.txt
│   ├── transcript_corrected.txt
│   ├── social_media_content.txt
│   ├── shorts_suggestions.yaml
│   ├── shorts/
│   └── shorts_with_text/
```



## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Whisper](https://github.com/openai/whisper) for transcription
- [OpenAI](https://openai.com) for ChatGPT integration
- [FFmpeg](https://ffmpeg.org) for video processing

---