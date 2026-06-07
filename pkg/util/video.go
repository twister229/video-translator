package util

import (
	"fmt"
	"krillin-ai/internal/storage"
	"os/exec"
)

func ReplaceAudioInVideo(videoFile string, audioFile string, outputFile string) error {
	cmd := exec.Command(storage.FfmpegPath, "-i", videoFile, "-i", audioFile, "-c:v", "copy", "-map", "0:v:0", "-map", "1:a:0", outputFile)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error replacing audio in video: %v", err)
	}

	return nil
}

// MixDubbingIntoVideo overlays a dubbing track on top of the video's original
// audio, ducking (lowering) the original so music and sound effects stay
// audible underneath the voice. originalVolume is a linear gain applied to the
// original track (e.g. 0.15 keeps it at 15%). The dubbing track plays at full
// volume. Falls back gracefully if the video has no original audio track.
func MixDubbingIntoVideo(videoFile string, dubbingFile string, outputFile string, originalVolume float64) error {
	// Build a filter graph:
	//   [0:a] original audio, scaled down by originalVolume -> [bg]
	//   [1:a] dubbing audio at full volume                  -> [vo]
	//   amix the two, keeping the longest stream            -> [aout]
	filter := fmt.Sprintf(
		"[0:a]volume=%.3f[bg];[1:a]volume=1.0[vo];[bg][vo]amix=inputs=2:duration=longest:dropout_transition=0:normalize=0[aout]",
		originalVolume,
	)

	cmd := exec.Command(storage.FfmpegPath,
		"-y",
		"-i", videoFile,
		"-i", dubbingFile,
		"-filter_complex", filter,
		"-map", "0:v:0",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "aac",
		"-shortest",
		outputFile,
	)

	if err := cmd.Run(); err != nil {
		// The original may have no audio stream; fall back to a straight replace
		// so dubbing still works on silent-source videos.
		return ReplaceAudioInVideo(videoFile, dubbingFile, outputFile)
	}

	return nil
}

