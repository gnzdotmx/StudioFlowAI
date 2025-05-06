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

Transcribing the audio and generating social media content in one go.

![Shorts](./media/demo-get-shorts.gif)

Generating short video clip suggestions and extracting them in one go.

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
# Run with the input path defined in the first step of the workflow
studioflowai run -w path/to/workflow.yaml

# Override the input file path for the first step
studioflowai run -w path/to/workflow.yaml -i /path/to/input/video.mp4
```

Each workflow run creates a timestamped subfolder within the output directory specified in the workflow file. For example, if your workflow output is set to `./output`, the results will be stored in a folder like `./output/Complete_Video_Processing_Workflow-20231015-120530/`.

### ♻️ Retrying Failed Workflows

If a workflow fails during execution (e.g., because it couldn't find a prompt template), you can retry it from the point of failure:

```bash
# Retry a failed workflow
studioflowai run -w path/to/workflow.yaml --retry --output-folder ./output/Complete_Video_Processing_Workflow-20231015-120530 --workflow-name "Complete Video Processing Workflow"
```

The retry functionality will:
1. Use the same output folder from the previous run
2. Analyze the output folder to determine which steps completed successfully
3. Resume execution from the first step that failed
4. Continue with the remaining steps in the workflow

This is especially useful for long-running workflows where you don't want to restart from the beginning.

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
description: Extract audio, transcribe, format, correct and generate social media content
output: ./output
# Results will be stored in a timestamped subfolder

steps:
  - name: Extract Audio
    module: extract
    parameters:
      input: ./input/video.mp4  # Can be overridden with the --input CLI flag
      outputName: "audio.wav"  # Set consistent output file name
      audioFormat: wav
      sampleRate: 16000
      channels: 1
      
  - name: Transcribe Audio
    module: transcribe
    parameters:
      input: "${output}/audio.wav"  # References the output directory automatically
      outputFileName: "transcript"  # Set consistent output file name
      model: "whisper"
      # Language is optional - if not specified, Whisper will auto-detect
      # language: "Spanish"  # Uncomment to force a specific language
      outputFormat: "srt"
      whisperParams: "--model large-v2 --beam_size 5 --temperature 0.0 --word_timestamps True"

  - name: Format Transcription
    module: format
    parameters:
      input: "${output}/transcript.srt"  # Use the output from transcribe step
      outputFileName: "transcript"  # Set consistent output file name
      removePatterns:  # Will remove these patterns from the transcript
        - "Transcribed by gnzdotmx\\.ai"
        - "Subtítulos realizados por la comunidad de Amara\\.org"
      cleanFileSuffix: "_clean"

  - name: Correct With ChatGPT
    module: chatgpt
    parameters:
      input: "${output}/transcript_clean.txt"  # Use the output from format step
      outputFileName: "transcript_corrected"  # Specific output file name
      promptTemplate: "./prompts/transcription_correction.yaml"  # Using YAML prompt template
      targetLanguage: "auto"  # Auto-detect language
      model: "gpt-4o"
      temperature: 0.1
      maxTokens: 4000
      
  - name: Generate Social Media Content
    module: sns
    parameters:
      input: "${output}/transcript_corrected.txt"  # Use the output from chatgpt step
      outputFileName: "social_media_content"  # Set descriptive output file name
      model: "gpt-4o"
      temperature: 0.7  # Higher temperature for creative content
      maxTokens: 8000  # Increased for more detailed content
      language: "auto"  # Auto-detect or specify a language like "Spanish"
```

You can find more example workflows in the `examples/` directory.

## 🧩 Modules

### 🔊 Extract

Extracts audio from video files.

Parameters:
- `input`: Path to input video file or directory
- `output`: Path to output directory
- `audioFormat`: Output audio format (default: wav)
- `sampleRate`: Sample rate in Hz (default: 16000)
- `channels`: Number of audio channels (default: 1)

### ✂️ Split

Splits audio files into smaller segments.

Parameters:
- `input`: Path to input audio file or directory
- `output`: Path to output directory
- `segmentTime`: Segment duration in seconds (default: 1800 = 30 minutes)
- `filePattern`: Output file pattern (default: "splited%03d")
- `audioFormat`: Output audio format (default: "wav")

### 🧹 Format

Formats transcription files by removing unwanted patterns and timestamps.

Parameters:
- `input`: Path to input transcript file or directory
- `output`: Path to output directory
- `inputFileName`: Name of input file when using a directory path (for selecting a specific file)
- `outputFileName`: Custom output file name (without extension)
- `removePatterns`: Patterns to remove from each line
- `combineOutput`: Whether to combine all transcripts (default: true) - deprecated
- `cleanFileSuffix`: Suffix for formatted files (default: "_clean")

### 🤖 ChatGPT

Corrects transcriptions using the OpenAI API.

Parameters:
- `input`: Path to input transcript file or directory
- `output`: Path to output directory
- `filePattern`: File pattern to match (default: "*_clean.txt")
- `promptTemplate`: Path to prompt template file
- `outputSuffix`: Suffix for corrected files (default: "_corrected")
- `model`: OpenAI model to use (default: "gpt-4o")
- `temperature`: Model temperature (default: 0.1)
- `maxTokens`: Maximum tokens for the response (default: 4000)
- `targetLanguage`: Target language for corrections (default: "English")
- `requestTimeoutMs`: API request timeout in milliseconds (default: 60000)

### 🎬 Shorts

Generates high-quality suggestions for short video clips based on intelligent transcript analysis.

Parameters:
- `input`: Path to input transcript file or directory
- `output`: Path to output directory
- `filePattern`: File pattern to match (default: "*_corrected.txt")
- `outputFileName`: Custom output file name (without extension) (default: "shorts_suggestions")
- `minDuration`: Minimum duration of shorts in seconds (default: 15)
- `maxDuration`: Maximum duration of shorts in seconds (default: 60)
- `model`: OpenAI model to use (default: "gpt-4o")
- `temperature`: Model temperature for creativity (default: 0.7)
- `maxTokens`: Maximum tokens for the response (default: 4000)
- `promptFilePath`: Path to custom prompt template file
- `requestTimeoutMs`: API request timeout in milliseconds (default: 60000)

Unlike other modules, the Shorts module doesn't limit suggestions to a fixed number. Instead, it uses AI to intelligently analyze the entire transcript and identify only the most compelling segments that would make excellent standalone short-form videos. The output is a YAML file that can be used with ffmpeg commands like:

```bash
ffmpeg -ss 00:15:00 -to 00:16:00 -i input_video.mp4 -c copy output-from001500to001600.mp4
```

### 📹 Extract Shorts

Automatically extracts short video clips based on the YAML suggestions generated by the Shorts module.

Parameters:
- `input`: Path to shorts_suggestions.yaml file
- `output`: Path to output directory
- `videoFile`: Path to the source video file (IMPORTANT: should match the original input video)
- `outputFormat`: Output video format (default: "mp4")
- `ffmpegParams`: Additional parameters for FFmpeg (e.g., "-c:v libx264 -c:a aac -b:a 128k")
- `inputFileName`: Specific input file name when using a directory path
- `quietFlag`: Suppress verbose ffmpeg output (default: true)

This module completes the shorts generation workflow by converting AI suggestions into actual video clips ready for social media platforms. It reads the timestamps and other metadata from the YAML file and uses ffmpeg to extract each suggested clip as a separate video file with an optimized filename.

Example workflow integration:
```yaml
- name: Generate Shorts Suggestions
  module: shorts
  parameters:
    input: "${output}/transcript_corrected.txt"
    outputFileName: "shorts_suggestions"
    model: "gpt-4o"
    temperature: 0.8
    maxTokens: 16000
    minDuration: 30
    maxDuration: 60
    promptFilePath: "./prompts/shorts_prompts.yaml"
    
- name: Extract Shorts Clips
  module: extractshorts
  parameters:
    input: "${output}/shorts_suggestions.yaml"
    videoFile: "./input/original_video.mp4"
    outputFormat: "mp4"
    ffmpegParams: "-c:v libx264 -c:a aac -b:a 128k"
    quietFlag: true
```

The extracted clips are named according to their title and timestamps for easy identification and management.

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
6. 🎬 Generate short video clip suggestions with the Shorts module

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
| promptFilePath | Path to custom prompt YAML file | "./prompts/sns_content.yaml" |

The output is saved as a Markdown file that can be easily copied and used across platforms.

## 🎞️ Short Video Clips Workflow

StudioFlowAI includes a complete workflow for generating and extracting short-form video clips (shorts) from your longer content - perfect for platforms like TikTok, Instagram Reels, YouTube Shorts, and more.

### 🧠 Intelligent Clip Selection

The system uses a two-stage process to create high-quality short clips:

1. **Analysis & Suggestion**: The `shorts` module uses AI to analyze your video transcript and identify the most engaging segments that would work well as standalone clips. It considers factors like:
   - Hook factor and ability to capture attention quickly
   - Viral potential and shareability
   - Emotional impact and viewer engagement
   - Self-contained narratives that make sense without additional context
   - Memorable quotes or compelling insights

2. **Automatic Extraction**: The `extractshorts` module takes these suggestions and automatically extracts the actual video clips using FFmpeg, handling all the technical aspects of cutting and encoding.

### 🚀 Running the Shorts Workflow

To generate and extract short clips from your video:

```bash
studioflowai run examples/complete_workflow.yaml --input ./input/your_video.mp4
```

This will:
1. Process the video through transcription and correction
2. Generate AI-powered suggestions for short clips (with titles, descriptions, and tags)
3. Automatically extract each suggested clip as a separate video file

### 🎯 Key Features

- **Balanced Distribution**: Ensures clips are selected from throughout the video (beginning, middle, and end)
- **Content-Aware Selection**: Identifies clips based on content quality, not arbitrary time intervals
- **Complete Metadata**: Generates compelling titles, descriptions, and hashtags for each clip
- **Production-Ready Output**: Creates properly encoded video files ready for immediate upload
- **Format Flexibility**: Supports customization of output format and encoding parameters

### 🔧 Customization

You can customize the shorts generation process by:
- Adjusting the minimum and maximum clip duration
- Modifying the selection criteria in the prompt template
- Changing the output format and encoding parameters
- Setting language preferences

For non-English content, the system can analyze transcripts in any language and generate metadata in the specified language (default is Spanish).

The result is a collection of high-quality, strategically selected short clips that maximize engagement potential while maintaining the original video's key messages and insights.