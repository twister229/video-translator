# Video Translator

A focused video translation and dubbing tool with Vietnamese output. It does two
things:

1. **Subtitles** — transcribe a video's speech, translate it, and produce SRT
   subtitle files (original, Vietnamese, and bilingual).
2. **Dubbing** — generate Vietnamese voice-over from the translated subtitles and
   mix it over the original audio. The original track is **ducked** (lowered, not
   erased) so background music and sound effects stay audible under the voice.

This is a stripped-down derivative of [KrillinAI](https://github.com/krillinai/KrillinAI)
(GPL-3.0), keeping only the subtitle and TTS dubbing pipeline plus a CLI and an
optional web server. The desktop GUI, cover generation, Aliyun/voice-cloning
backends, and unused transcription backends were removed.

## Pipeline

```
input video ──► transcribe (Whisper) ──► translate (LLM) ──► SRT subtitles
                                                                  │
                                                                  ▼
                                          dubbing (edge-tts) + duck original ──► dubbed video
```

- **Transcription:** OpenAI Whisper API (default). Local backends
  (fasterwhisper, whisperkit, whisper.cpp) are available via config.
- **Translation:** any OpenAI-compatible LLM API. The translation prompt is tuned
  for natural spoken Vietnamese (register-appropriate pronouns, correct diacritics,
  idiomatic phrasing instead of literal English calques).
- **Dubbing:** edge-tts (default, free, no API key) with Vietnamese voices
  `vi-VN-HoaiMyNeural` (female) and `vi-VN-NamMinhNeural` (male). OpenAI TTS is
  also supported. The original audio is mixed back in at
  `app.dub_background_volume` (default 0.15 = 15%); set it to 0 to fully silence
  the original or 1 to disable ducking.

### Dub timing (bounded drift + resync)

Vietnamese voice lines are often longer than the original subtitle window. Rather
than always speeding the voice up to fit (which causes a chipmunk effect on dense
lines), the dubber:

- lets a long line borrow the silent gap before the next subtitle,
- only speeds up when it still overflows, capped at `app.dub_max_speed` (default 1.3x),
- tracks cumulative drift and claws it back whenever a later line has slack,
- forces harder catch-up if drift ever exceeds `app.dub_max_drift_ms` (default 2000ms),

so the dub stays in sync with the video over long runtimes instead of falling behind.

If some voice lines fail to synthesize, they are replaced with short silence and
the CLI response reports the affected line numbers in `failed_indexes` plus a
warning, rather than silently shipping a gapped dub as a clean success.

`ffmpeg`, `ffprobe`, `yt-dlp`, and `edge-tts` are downloaded automatically on first
run into `./bin`.

## Configuration

Copy the example config and fill in your API key:

```bash
mkdir -p config
cp config/config-example.toml config/config.toml
```

For the simplest setup (OpenAI transcription + translation, free Vietnamese
dubbing) set in `config/config.toml`:

```toml
[llm]
    api_key = "sk-..."          # your OpenAI-compatible key
    model   = "gpt-4o-mini"

[transcribe]
    provider = "openai"
    [transcribe.openai]
        api_key = "sk-..."      # OpenAI key for Whisper

[tts]
    provider = "edge-tts"       # free, no key needed
```

### Custom LLM provider (OpenAI-compatible)

The translation step (`[llm]`) works with any OpenAI-compatible chat API —
DeepSeek, Qwen/DashScope, Groq, OpenRouter, Together, a local Ollama/vLLM server,
or a gateway. Set `base_url`, `api_key`, and `model`:

```toml
[llm]
    base_url = "https://api.deepseek.com/v1"   # custom endpoint, include the /v1 path
    api_key  = "your-provider-key"
    model    = "deepseek-chat"
```

Common endpoints (note the path prefix — the client appends `/chat/completions`):

| Provider   | `base_url`                          |
|------------|-------------------------------------|
| OpenAI     | leave empty (default)               |
| DeepSeek   | `https://api.deepseek.com/v1`       |
| Groq       | `https://api.groq.com/openai/v1`    |
| OpenRouter | `https://openrouter.ai/api/v1`      |
| Ollama     | `http://localhost:11434/v1`         |

Three things to know:

- **`[llm]`, `[transcribe]`, and `[tts]` are independent** and each has its own
  `base_url`/`api_key`. A custom LLM provider only changes translation.
- **Transcription needs a Whisper-compatible `/audio/transcriptions` endpoint**,
  which most LLM-only providers do not offer. Keep `[transcribe]` on real OpenAI,
  or use a local backend (`fasterwhisper`, `whisperkit`, `whisper.cpp`).
- **The chat call uses streaming.** Nearly all OpenAI-compatible providers support
  it; if a specific one does not, translation will error on the stream request.

## Build

```bash
go build -o build/video-translator-cli    ./cmd/cli
go build -o build/video-translator-server ./cmd/server
```

## Test

```bash
go test ./config/... ./internal/... ./pkg/util/...
go vet  ./config/... ./internal/... ./pkg/util/...
```

The dubbing timing model (`internal/service/dubbing_timing.go`) is pure and unit
tested in isolation from ffmpeg — see `dubbing_timing_test.go` for the drift,
resync, and speed-cap cases.

## CLI usage

Generate subtitles (English source → Vietnamese):

```bash
./build/video-translator-cli subtitle "https://www.youtube.com/watch?v=VIDEO_ID" \
  --origin-lang en \
  --target-lang vi \
  --workdir tasks/demo \
  --caption-source any
```

Outputs: `origin_language_srt.srt`, `target_language_srt.srt`,
`bilingual_srt.srt`.

Generate Vietnamese dubbing from the translated subtitles:

```bash
./build/video-translator-cli tts \
  --workdir tasks/demo \
  --input-srt tasks/demo/target_language_srt.srt \
  --line-mode target-only \
  --video tasks/demo/origin_video.mp4 \
  --voice vi-VN-HoaiMyNeural
```

Outputs: `tts_final_audio.wav`, `video_with_tts.mp4`.

The CLI prints a single JSON line on stdout and writes `krillinai_manifest.json`
to the working directory, so stages can chain and reuse prior artifacts.

A local video file works in place of a URL:

```bash
./build/video-translator-cli subtitle ./my_video.mp4 --origin-lang en --target-lang vi --workdir tasks/local
```

## Web server

Build and start the server, then use the browser UI:

```bash
go build -o build/video-translator-server ./cmd/server
./build/video-translator-server
```

Open `http://127.0.0.1:8888` (host/port configurable under `[server]` in
`config/config.toml`). The server requires a valid `config/config.toml` at startup
— it exits immediately if none is found.

In the browser you:

1. Upload a video (or paste a URL).
2. Choose target language (Vietnamese), bilingual on/off, and whether to generate dubbing.
3. Start the task and watch progress, then download the SRT and dubbed video.

It exposes a small JSON API under `/api` if you prefer to drive it directly:

| Method | Path                          | Purpose                          |
|--------|-------------------------------|----------------------------------|
| POST   | `/api/capability/subtitleTask`| Start a subtitle/dub task        |
| GET    | `/api/capability/subtitleTask`| Poll task status                 |
| POST   | `/api/file`                   | Upload a source video            |
| GET    | `/api/file/*`                 | Download a result file           |
| GET    | `/api/config`                 | Read current config              |
| POST   | `/api/config`                 | Update config at runtime         |

> Note: the web UI is inherited from KrillinAI and runs an older task flow that is
> separate from the CLI pipeline. The CLI is the primary, fully-tested entrypoint;
> the web server is not yet verified end-to-end for this stripped-down build.

## License

GPL-3.0, inherited from KrillinAI. See `LICENSE`.
