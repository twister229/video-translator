# Video Translator

A focused video translation and dubbing tool with Vietnamese output, derived from
[KrillinAI](https://github.com/krillinai/KrillinAI) (GPL-3.0). Two features:

1. **Subtitles** — transcribe speech, translate, emit SRT (origin, Vietnamese, bilingual).
2. **Dubbing** — Vietnamese voice-over mixed over the ducked original audio.

## Stack

- **Language:** Go (module `krillin-ai`, go 1.24)
- **Entrypoints:** `cmd/cli` (primary), `cmd/server` (web UI on :8888)
- **Pipeline:** transcribe (OpenAI Whisper) → translate (OpenAI-compatible LLM) → TTS dubbing (edge-tts)
- **External tools:** ffmpeg, ffprobe, yt-dlp, edge-tts (auto-downloaded to `./bin` on first run)

## Build & test

```bash
go build -o build/video-translator-cli    ./cmd/cli
go build -o build/video-translator-server ./cmd/server
go test ./config/... ./internal/... ./pkg/util/...
go vet ./internal/... ./config/... ./pkg/util/...
```

## Conventions

- Config lives in `config/config.toml` (copy from `config/config-example.toml`); defaults in `config/config.go`.
- Dubbing timing logic is pure and unit-tested in `internal/service/dubbing_timing.go` — keep ffmpeg side effects out of `planClipTiming`.
- Aliyun backends and voice-cloning were removed; do not reintroduce them.
- Removed/inherited dead tests with hardcoded machine paths — do not re-add machine-specific integration tests.

## Skill routing

When the user's request matches an available gstack skill, invoke it via the Skill tool. When in doubt, invoke the skill.

Key routing rules:
- Product ideas/brainstorming → invoke /office-hours
- Strategy/scope → invoke /plan-ceo-review
- Architecture → invoke /plan-eng-review
- Design system/plan review → invoke /design-consultation or /plan-design-review
- Full review pipeline → invoke /autoplan
- Bugs/errors → invoke /investigate
- QA/testing site behavior → invoke /qa or /qa-only
- Code review/diff check → invoke /review
- Visual polish → invoke /design-review
- Ship/deploy/PR → invoke /ship or /land-and-deploy
- Save progress → invoke /context-save
- Resume context → invoke /context-restore

Use the `/browse` skill from gstack for all web browsing. Never use `mcp__claude-in-chrome__*` tools.
