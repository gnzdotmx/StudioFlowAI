# 🎬 StudioFlowAI

A powerful command-line tool for automating video processing workflows with AI-powered features.

## ✨ Features

- 🎥 **Video Processing**: Extract audio, transcribe, generate shorts, and add text overlays
- 🤖 **AI Integration**: Powered by Whisper for transcription and GPT-4 for content generation
- 📝 **YAML Workflows**: Define processing pipelines in simple YAML files
- 🔄 **Modular Design**: Easy to extend with new processing modules
- 🎯 **Smart Output**: Automatic output directory management with timestamps
- 🔁 **Retry Support**: Resume failed workflows from any step

## 🚀 Quick Start

1. **Install**:
```bash
git clone https://github.com/gnzdotmx/studioflowai.git
cd studioflowai
go mod download
go build -o studioflowai
```

2. **Required Tools**:
- FFmpeg (video/audio processing)
- Whisper (transcription)

3. **Run a Workflow**:
```bash
# With CLI input
studioflowai run -w examples/complete_workflow.yaml -i ./input/video.mp4

# With workflow-defined input
studioflowai run -w examples/complete_workflow.yaml
```

## 📋 Input/Output Handling

### Input Sources
1. 🖥️ CLI input (`-i` flag)
2. 📄 Workflow file (first step's input)
3. 📂 Previous step's output

### Output Structure
```
input_file_directory/
└── output/
    └── workflow_name-YYYYMMDD-HHMMSS/
        ├── audio.wav
        ├── transcript.srt
        ├── transcript_clean.txt
        └── ...
```

## 🔄 Retry Failed Workflows

```bash
studioflowai run -w workflow.yaml --retry \
  --output-folder /path/to/output \
  --workflow-name "Step Name"
```

## 🧩 Available Modules

### 🎵 Extract Audio
- Extracts audio from video files
- Supports WAV format with customizable sample rate

### 🗣️ Transcribe
- Uses Whisper for high-quality transcription
- Supports multiple languages and formats (SRT)

### 📝 Format
- Cleans and formats transcriptions
- Removes unwanted patterns and text

### 🤖 ChatGPT
- Corrects and enhances transcriptions
- Supports multiple languages
- Configurable temperature and token limits

### 📱 SNS (Social Media)
- Generates messages to post on social media platforms
- Customizable tone and style
- Multi-language support

### 🎥 Shorts
- Generates short video clip suggestions
- Configurable duration limits
- Quality-focused content selection

### ✂️ Extract Shorts
- Extracts video clips based on suggestions
- Supports multiple video formats
- Customizable FFmpeg parameters

### 📺 Add Text
- Adds text overlays to videos
- Customizable font, color, and position
- Supports multiple text styles

## 📚 Example Workflows

### Complete Pipeline
```bash
# Process video with all steps
studioflowai run -w examples/complete_workflow.yaml -i ./input/video.mp4
```

### Shorts Only
```bash
# Extract and process shorts
studioflowai run -w examples/extract_shorts.yaml -i ./input/video.mp4
```

## 📄 License

MIT License - see LICENSE file for details.