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
input: ./input/video.mp4  # Can be overridden with the --input CLI flag
output: ./output
# Results will be stored in a timestamped subfolder

steps:
  - name: Extract Audio
    module: extract
    parameters:
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