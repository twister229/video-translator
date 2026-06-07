package service

// Dubbing timing model: bounded drift + resync.
//
// The TTS clips are concatenated sequentially, so a clip's real position in the
// final track is the sum of all prior clip durations — NOT its SRT timestamp.
// If we let a long Vietnamese line "run long" without bound, every later clip is
// pushed back and the dub drifts behind the video. If we always speed clips up to
// fit (the old atempo-to-window behavior), dense lines sound chipmunked.
//
// planClipTiming balances the two: prefer natural speed, borrow the silent gap
// before the next line, speed up only when needed and only up to maxSpeed, and
// claw back accumulated drift whenever there is slack. A hard ceiling
// (maxDriftSec) forces catch-up so a pathological all-dense video degrades to
// "slightly fast" instead of "seconds behind".
//
//	timeline:  [--- window ---][- gap -][--- next window ---]
//	clip fits in window+gap          -> place, pad/trim slack, resync drift down
//	clip longer, speed <= maxSpeed   -> atempo to fit, no new drift
//	clip longer, speed >  maxSpeed   -> cap speed, overflow adds to drift
//	drift over ceiling               -> push speed past cap (<=2.0) to catch up

const (
	// ffmpeg's atempo filter is stable within [0.5, 2.0]; never exceed 2.0 with
	// a single filter pass. This is the absolute upper bound for catch-up.
	maxAtempoSingle = 2.0
)

// clipPlan is the decision for one dubbed clip: how fast to play it, what total
// duration to pad it out to (0 = no padding), and the resulting accumulated drift.
type clipPlan struct {
	Speed      float64 // atempo factor, 1.0 = unchanged, never < 1.0
	PadToSec   float64 // pad clip with trailing silence up to this total duration; 0 = no pad
	NewDriftSec float64 // accumulated drift after this clip (>= 0, dub behind video)
}

// planClipTiming computes the timing decision for a single clip.
//
//	windowSec   subtitle end - start
//	gapSec      silence until the next subtitle starts (0 if last line)
//	audioSec    natural (unmodified) TTS clip duration
//	driftSec    accumulated drift before this clip (>= 0)
//	maxSpeed    soft cap on speed-up (e.g. 1.3); speeds above this only happen for ceiling catch-up
//	maxDriftSec hard ceiling on drift; above it we force harder catch-up
func planClipTiming(windowSec, gapSec, audioSec, driftSec, maxSpeed, maxDriftSec float64) clipPlan {
	if maxSpeed < 1.0 {
		maxSpeed = 1.0
	}
	if driftSec < 0 {
		driftSec = 0
	}
	budget := windowSec + gapSec

	// Degenerate slot (zero/negative window and gap): place at natural speed,
	// the whole clip becomes overflow drift.
	if budget <= 0 {
		return clipPlan{Speed: 1.0, PadToSec: 0, NewDriftSec: driftSec + audioSec}
	}

	// Case 1: clip fits within window+gap at natural speed.
	if audioSec <= budget {
		slack := budget - audioSec // silence we would otherwise insert
		// Resync: spend slack to claw back accumulated drift by shortening the slot.
		recover := driftSec
		if recover > slack {
			recover = slack
		}
		padTo := budget - recover
		// padTo could equal audioSec exactly (all slack spent on catch-up) -> no pad.
		if padTo <= audioSec {
			padTo = 0
		}
		return clipPlan{Speed: 1.0, PadToSec: padTo, NewDriftSec: driftSec - recover}
	}

	// Case 2: clip is longer than budget. Try speeding up within the soft cap.
	requiredSpeed := audioSec / budget
	if requiredSpeed <= maxSpeed {
		// Fits exactly into budget at an acceptable speed; no drift added.
		return clipPlan{Speed: requiredSpeed, PadToSec: 0, NewDriftSec: driftSec}
	}

	// Case 3: even maxSpeed can't fit it. Play at maxSpeed; overflow becomes drift.
	cappedLen := audioSec / maxSpeed
	overflow := cappedLen - budget
	newDrift := driftSec + overflow

	// Case 4: drift ceiling. If we've blown past the ceiling, push speed beyond
	// the soft cap (still <= 2.0) to claw the slot back down toward the ceiling.
	if newDrift > maxDriftSec {
		// Target slot so that resulting drift == maxDriftSec: slot = budget + (maxDriftSec - driftSec).
		targetSlot := budget + (maxDriftSec - driftSec)
		if targetSlot < audioSec/maxAtempoSingle {
			// Can't catch up fully even at 2.0x; clamp to fastest safe speed.
			targetSlot = audioSec / maxAtempoSingle
		}
		if targetSlot <= 0 {
			targetSlot = budget
		}
		speed := audioSec / targetSlot
		if speed > maxAtempoSingle {
			speed = maxAtempoSingle
		}
		actualSlot := audioSec / speed
		return clipPlan{Speed: speed, PadToSec: 0, NewDriftSec: driftSec + (actualSlot - budget)}
	}

	return clipPlan{Speed: maxSpeed, PadToSec: 0, NewDriftSec: newDrift}
}
