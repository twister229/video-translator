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

## Build

```bash
go build -o build/video-translator-cli    ./cmd/cli
go build -o build/video-translator-server ./cmd/server
```

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

```bash
./build/video-translator-server
```

Then open `http://127.0.0.1:8888` (host/port configurable under `[server]`).

## License

GPL-3.0, inherited from KrillinAI. See `LICENSE`.
