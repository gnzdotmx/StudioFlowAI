# 🎬 StudioFlowAI

A modular Go application for content creators to process videos using AI-powered YAML-defined workflows.

## 🔎 Overview

StudioFlowAI is a flexible, modular video processing tool that allows you to define custom content creation workflows in YAML. It supports:

- 🔊 Extracting audio from video files
- ✂️ Splitting audio into segments
- 🧹 Cleaning and processing transcription files
- 🤖 Correcting transcriptions using ChatGPT
- 📱 Generating social media content for multiple platforms

The application is designed to be extensible, making it easy to add new processing modules in the future.

![Demo](./media/demo.gif)

![Demo get shorts](./media//demo-get-shorts.gif)

## 📋 Requirements

- 🔸 Go 1.19 or higher
- 🔸 [FFmpeg](https://ffmpeg.org/download.html)
- 🔸 [Whisper](https://github.com/openai/whisper?tab=readme-ov-file#setup)
- 🔸 OpenAI API key (for ChatGPT correction and social media content generation)

## 💻 Installation

### Option 1: Direct Installation (Recommended)

You can install StudioFlowAI directly using Go's install command:

```bash
go install https://github.com/gnzdotmx/StudioFlowAI/studioflowai@latest
```

This will download, build, and install the binary to your `$GOPATH/bin` directory. Make sure this directory is in your system PATH to run the `studioflowai` command from anywhere.

### Option 2: Manual Build

If you prefer to build from source:

```bash
git clone https://github.com/gnzdotmx/StudioFlowAI.git
cd StudioFlowAI/studioflowai
go build -o studioflowai main.go
# Optional: Move the binary to a directory in your PATH
mv ./studioflowai $GOPATH/bin/
```

## 🚀 Usage

### ✅ Validating Your Environment

Before running workflows, you can check if your environment is properly set up:

```bash
studioflowai validate
```

This will verify that all required external tools (like FFmpeg) are installed and that necessary environment variables are set.

### ▶️ Running a Workflow

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

### 📝 Example Workflow

A complete workflow might look like this:

```yaml
name: Complete Video Processing Workflow
description: Extract audio, split into segments, transcribe, format and correct transcription

# Input Configuration
# The input video file can be specified in three ways:
# 1. Via CLI using the -i flag: studioflowai run -w complete_workflow.yaml -i ./input/video.mp4
# 2. In the workflow file (first step's input parameter)
# 3. From a previous step's output in the workflow

# Output Configuration
# The output directory will be automatically created with timestamp:
# ./output/Complete_Video_Processing_Workflow-YYYYMMDD-HHMMSS/
# Each module's output will be stored in this directory

steps:
  - name: Extract Audio
    module: extract
    parameters:
      # Input: Original video file
      input: ./input/video.mp4  # Can be overridden with -i flag
      # Output: Audio file in output directory
      outputName: "audio.wav"   # Will be stored in output directory
      sampleRate: 16000
      channels: 1
      
  - name: Transcribe Audio
    module: transcribe
    parameters:
      # Input: Audio file from previous step
      input: "${output}/audio.wav"  # References output directory
      # Output: Transcript file in output directory
      outputFileName: "transcript"  # Will create transcript.srt in output directory
      model: "whisper"
      # Language is optional - if not specified, Whisper will auto-detect
      # language: "Spanish"  # Uncomment to force a specific language
      outputFormat: "srt"    # Validated file extension
      whisperParams: "--model large-v3 --beam_size 5 --temperature 0.0 --best_of 5 --word_timestamps True --threads 16 --patience 1.0 --condition_on_previous_text True"

  - name: Format Transcription
    module: format
    parameters:
      # Input: Transcript from previous step
      input: "${output}/transcript.srt"
      # Output: Cleaned transcript in output directory
      outputFileName: "transcript"
      removePatterns:
        - "Subtítulos realizados por la comunidad de Amara\\.org"
      cleanFileSuffix: "_clean"

  - name: Correct With ChatGPT
    module: chatgpt
    parameters:
      # Input: Cleaned transcript from previous step
      input: "${output}/transcript_clean.txt"
      # Output: Corrected transcript in output directory
      outputFileName: "transcript_corrected"
      promptTemplate: "./prompts/transcription_correction.yaml"
      targetLanguage: "auto"
      model: "gpt-4o"
      temperature: 0.1
      maxTokens: 128000        # Maximum response tokens for GPT-4
      outputSuffix: "_corrected"
      requestTimeoutMs: 300000  # 5 minutes timeout
      chunkSize: 120000        # Process in chunks of 120k tokens
      
  - name: Generate Social Media Content
    module: sns
    parameters:
      # Input: Corrected transcript from previous step
      input: "${output}/transcript_corrected.txt"
      # Output: Social media content in output directory
      outputFileName: "social_media_content"
      model: "gpt-4o"
      temperature: 0.8  # Slightly higher temperature for creative content
      maxTokens: 16000   # Increased for more detailed content
      language: "Spanish"  # Generate content in Spanish
      promptFilePath: "./prompts/sns_content.yaml"  # Custom prompt template file (optional)
      
  - name: Generate Shorts Suggestions
    module: shorts
    parameters:
      # Input: Original transcript for timing information
      input: "${output}/transcript.srt"
      # Output: Shorts suggestions YAML in output directory
      outputFileName: "shorts_suggestions"
      model: "gpt-4o"
      temperature: 0.9                             # Higher temperature for creative suggestions
      maxTokens: 16000
      minDuration: 45                              # Minimum clip duration in seconds
      maxDuration: 75                              # Maximum clip duration in seconds
      promptFilePath: "./prompts/shorts_prompts.yaml"  # Quality-focused prompt template
      requestTimeoutMs: 300000  # 5 minutes
      chunkSize: 120000        # Adjust based on your needs
      
  - name: Extract Shorts Clips
    module: extractshorts
    parameters:
      # Input: Shorts suggestions and original video
      input: "${output}/shorts_suggestions.yaml"
      videoFile: "./tests/video-test.mov"  # Original input video file
      # Output: Shorts clips in output/shorts directory
      ffmpegParams: "-vf scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2,setsar=1 -c:v libx264 -c:a aac -b:a 128k -b:v 2500k"
      quietFlag: true
      
  - name: Add Text Overlay
    module: addtext
    parameters:
      # Input: Shorts suggestions and original video
      input: "${output}/shorts_suggestions.yaml"
      videoFile: "./tests/video-test.mov"  # Original input video file
      # Output: Shorts with text in output/shorts_with_text directory
      fontSize: 50
      fontColor: "white"
      boxColor: "black@0.5"
      boxBorderW: 5
      quietFlag: true
      fontFile: "/System/Library/Fonts/Helvetica.ttc"  # Native macOS font
      textX: "(w-text_w)/2"
      textY: "(h/3)-50"
```

You can find more example workflows in the `examples/` directory.

## 🧩 Modules

### 🔊 Extract

Extracts audio from video files.

Parameters:
- `input`: Path to input video file
- `outputName`: Output file name (will be stored in output directory)
- `sampleRate`: Sample rate in Hz (default: 16000)
- `channels`: Number of audio channels (default: 1)

### 🎙️ Transcribe

Transcribes audio files to text/subtitles.

Parameters:
- `input`: Path to input audio file
- `outputFileName`: Output file name (will create .srt file in output directory)
- `model`: Whisper model to use (default: "whisper")
- `language`: Target language (optional, auto-detected if not specified)
- `outputFormat`: Output format (default: "srt")
- `whisperParams`: Additional Whisper CLI parameters

### 🧹 Format

Formats transcription files by removing unwanted patterns and timestamps.

Parameters:
- `input`: Path to input transcript file
- `outputFileName`: Custom output file name (without extension)
- `removePatterns`: Patterns to remove from each line
- `cleanFileSuffix`: Suffix for formatted files (default: "_clean")

### 🤖 ChatGPT

Corrects transcriptions using the OpenAI API.

Parameters:
- `input`: Path to input transcript file
- `outputFileName`: Custom output file name (without extension)
- `promptTemplate`: Path to prompt template file
- `targetLanguage`: Target language for corrections (default: "auto")
- `model`: OpenAI model to use (default: "gpt-4o")
- `temperature`: Model temperature (default: 0.1)
- `maxTokens`: Maximum tokens for response (default: 128000)
- `outputSuffix`: Suffix for corrected files (default: "_corrected")
- `requestTimeoutMs`: API request timeout in milliseconds (default: 300000)
- `chunkSize`: Size of chunks to process (default: 120000)

### 📱 SNS (Social Network Sharing)

Generates social media content from transcripts.

Parameters:
- `input`: Path to input transcript file
- `outputFileName`: Custom output file name (without extension)
- `model`: OpenAI model to use (default: "gpt-4o")
- `temperature`: Model temperature (default: 0.8)
- `maxTokens`: Maximum tokens for response (default: 16000)
- `language`: Target language for content (default: "Spanish")
- `promptFilePath`: Path to custom prompt template file

### 🎬 Shorts

Generates suggestions for short video clips.

Parameters:
- `input`: Path to input transcript file
- `outputFileName`: Custom output file name (without extension)
- `model`: OpenAI model to use (default: "gpt-4o")
- `temperature`: Model temperature (default: 0.9)
- `maxTokens`: Maximum tokens for response (default: 16000)
- `minDuration`: Minimum clip duration in seconds (default: 45)
- `maxDuration`: Maximum clip duration in seconds (default: 75)
- `promptFilePath`: Path to custom prompt template file
- `requestTimeoutMs`: API request timeout in milliseconds (default: 300000)
- `chunkSize`: Size of chunks to process (default: 120000)

### ✂️ Extract Shorts

Extracts short video clips based on suggestions.

Parameters:
- `input`: Path to shorts suggestions YAML file
- `videoFile`: Path to original input video file
- `ffmpegParams`: FFmpeg parameters for video processing
- `quietFlag`: Suppress FFmpeg output (default: true)

### 📝 Add Text Overlay

Adds text overlay to video clips.

Parameters:
- `input`: Path to shorts suggestions YAML file
- `videoFile`: Path to original input video file
- `fontSize`: Font size for text overlay (default: 50)
- `fontColor`: Color of the text (default: "white")
- `boxColor`: Color of the text background box (default: "black@0.5")
- `boxBorderW`: Width of the box border (default: 5)
- `quietFlag`: Suppress FFmpeg output (default: true)
- `fontFile`: Path to font file
- `textX`: X position of text (default: "(w-text_w)/2")
- `textY`: Y position of text (default: "(h/3)-50")

## 🔑 Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (required for the ChatGPT module)

## ⚙️ Setting Up Environment Variables

You can set up environment variables using a `.env` file in the root directory of the project:

1. Create a `.env` file in the project root:
   ```bash
   touch .env
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

Note: Make sure to add `.env` to your `.gitignore` to prevent accidentally committing sensitive credentials.

## 🔌 Extending StudioFlowAI

You can add new modules by implementing the `Module` interface and registering them in the `registerModules` function in `pkg/workflow/workflow.go`.

## 📱 Video Processing Workflow with Social Media Content Generation

The system includes a powerful Social Network Sharing (SNS) module that generates optimized content for YouTube and other social media platforms. The module analyzes transcripts to create:

- 🎯 Engaging titles optimized for search (SEO)
- 📝 Comprehensive YouTube descriptions with calls to action
- 📣 Social media copies tailored for different platforms (Twitter, Instagram/Facebook, LinkedIn)
- 🔍 SEO keywords for better discoverability
- ⏱️ Detailed timeline with timestamps for video navigation

### 🚀 Running the Complete Workflow

To process a video with social media content generation:

```bash
studioflowai run examples/complete_workflow.yaml --input ./input/your_video.mp4
```

The workflow will:
1. 🔊 Extract audio from the video
2. 🎙️ Transcribe the audio using Whisper
3. 🧹 Format the transcription (remove artifacts, standardize format)
4. 🤖 Correct the transcription with ChatGPT
5. 📱 Generate social media content with the SNS module

### 📊 SNS Module Parameters

The SNS module supports the following parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| input | Path to input transcript file or directory | |
| output | Path to output directory | |
| filePattern | File pattern to match | "*_corrected.txt" |
| inputFileName | Specific input file name to process | |
| outputFileName | Custom output file name (without extension) | |
| model | OpenAI model to use | "gpt-4o" |
| temperature | Model temperature (creativity level) | 0.1 |
| maxTokens | Maximum tokens for the response | 8000 |
| language | Language for the content | "Spanish" |

The output is saved as a Markdown file that can be easily copied and used across platforms.