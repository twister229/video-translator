package service

import (
	"math"
	"testing"
)

const (
	testMaxSpeed = 1.3
	testMaxDrift = 2.0 // seconds
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

// Lines that fit comfortably in their window add no drift and never speed up.
func TestPlanClipTiming_FitsNaturally(t *testing.T) {
	p := planClipTiming(3.0 /*window*/, 1.0 /*gap*/, 2.0 /*audio*/, 0 /*drift*/, testMaxSpeed, testMaxDrift)
	if p.Speed != 1.0 {
		t.Fatalf("Speed = %.3f, want 1.0 (no speed-up needed)", p.Speed)
	}
	if p.NewDriftSec != 0 {
		t.Fatalf("NewDriftSec = %.3f, want 0", p.NewDriftSec)
	}
	// Pads out to window+gap = 4.0s slot.
	if !approx(p.PadToSec, 4.0) {
		t.Fatalf("PadToSec = %.3f, want 4.0", p.PadToSec)
	}
}

// A long line borrows the gap before speeding up — no atempo, no drift.
func TestPlanClipTiming_BorrowsGap(t *testing.T) {
	// window 2.0, gap 1.5 -> budget 3.5; audio 3.4 fits without speed-up.
	p := planClipTiming(2.0, 1.5, 3.4, 0, testMaxSpeed, testMaxDrift)
	if p.Speed != 1.0 {
		t.Fatalf("Speed = %.3f, want 1.0 (gap absorbs the overflow)", p.Speed)
	}
	if p.NewDriftSec != 0 {
		t.Fatalf("NewDriftSec = %.3f, want 0", p.NewDriftSec)
	}
}

// When the clip exceeds budget but fits within maxSpeed, speed up and add no drift.
func TestPlanClipTiming_SpeedsUpWithinCap(t *testing.T) {
	// budget 2.0, audio 2.4 -> requiredSpeed 1.2 (<= 1.3). No drift.
	p := planClipTiming(2.0, 0, 2.4, 0, testMaxSpeed, testMaxDrift)
	if !approx(p.Speed, 1.2) {
		t.Fatalf("Speed = %.3f, want 1.2", p.Speed)
	}
	if p.NewDriftSec != 0 {
		t.Fatalf("NewDriftSec = %.3f, want 0", p.NewDriftSec)
	}
}

// Beyond the speed cap, the clip plays at maxSpeed and the overflow becomes drift.
func TestPlanClipTiming_CapHitAddsDrift(t *testing.T) {
	// budget 2.0, audio 3.9 -> requiredSpeed 1.95 > 1.3.
	// cappedLen = 3.9/1.3 = 3.0; overflow = 3.0 - 2.0 = 1.0s drift.
	p := planClipTiming(2.0, 0, 3.9, 0, testMaxSpeed, testMaxDrift)
	if !approx(p.Speed, 1.3) {
		t.Fatalf("Speed = %.3f, want 1.3 (soft cap)", p.Speed)
	}
	if !approx(p.NewDriftSec, 1.0) {
		t.Fatalf("NewDriftSec = %.3f, want 1.0", p.NewDriftSec)
	}
}

// Accumulated drift is clawed back when a later line has slack.
// Note: residual drift > 0 and a non-zero pad are mutually exclusive — recovering
// drift shrinks the slot toward the clip length, so any leftover drift means all
// slack was already spent. Here slack (1.0) < drift (1.5), so drift only partially
// recovers and the clip fills the shortened slot exactly (no pad).
func TestPlanClipTiming_ResyncRecoversDrift(t *testing.T) {
	// Start 1.5s behind. window 2.0, gap 0 -> budget 2.0; audio 1.0 -> slack 1.0.
	// recover = min(1.5, 1.0) = 1.0 -> drift drops to 0.5, slot shortened to 1.0.
	p := planClipTiming(2.0, 0, 1.0, 1.5, testMaxSpeed, testMaxDrift)
	if p.Speed != 1.0 {
		t.Fatalf("Speed = %.3f, want 1.0", p.Speed)
	}
	if !approx(p.NewDriftSec, 0.5) {
		t.Fatalf("NewDriftSec = %.3f, want 0.5 (clawed back 1.0s of 1.5s)", p.NewDriftSec)
	}
}

// When slack exceeds drift, drift fully recovers AND the remaining slack pads the clip.
func TestPlanClipTiming_ResyncFullRecoveryWithPad(t *testing.T) {
	// drift 0.5, window 2.0, gap 1.0 -> budget 3.0; audio 1.0 -> slack 2.0.
	// recover = min(0.5, 2.0) = 0.5 -> drift 0, slot = 3.0 - 0.5 = 2.5, pad clip to 2.5.
	p := planClipTiming(2.0, 1.0, 1.0, 0.5, testMaxSpeed, testMaxDrift)
	if p.NewDriftSec != 0 {
		t.Fatalf("NewDriftSec = %.3f, want 0 (fully recovered)", p.NewDriftSec)
	}
	if !approx(p.PadToSec, 2.5) {
		t.Fatalf("PadToSec = %.3f, want 2.5", p.PadToSec)
	}
}

// Once over the ceiling, speed is pushed to the ffmpeg max (2.0) for maximum
// catch-up. A single line denser than 2x-fits-budget still adds unavoidable drift
// (you can't fit 3s of speech into 1s), but the ceiling logic minimizes it vs the
// soft-cap path.
func TestPlanClipTiming_CeilingForcesCatchUp(t *testing.T) {
	// Already at the 2.0s ceiling, another dense line: budget 1.0, audio 3.0.
	p := planClipTiming(1.0, 0, 3.0, 2.0, testMaxSpeed, testMaxDrift)
	if !approx(p.Speed, maxAtempoSingle) {
		t.Fatalf("Speed = %.3f, want %.1f (max catch-up)", p.Speed, maxAtempoSingle)
	}
	// Soft-cap path would give drift 2.0 + (3.0/1.3 - 1.0) = 3.31. Ceiling beats it.
	softCapDrift := 2.0 + (3.0/testMaxSpeed - 1.0)
	if p.NewDriftSec >= softCapDrift {
		t.Fatalf("NewDriftSec = %.3f, ceiling should beat soft-cap drift %.3f", p.NewDriftSec, softCapDrift)
	}
}

// Drift must never go negative (dub must never get AHEAD of the video).
func TestPlanClipTiming_DriftNeverNegative(t *testing.T) {
	// Huge slack, zero starting drift: recovery can't push drift below 0.
	p := planClipTiming(10.0, 5.0, 1.0, 0, testMaxSpeed, testMaxDrift)
	if p.NewDriftSec < 0 {
		t.Fatalf("NewDriftSec = %.3f, must never be negative", p.NewDriftSec)
	}
}

// Realistic sequence: dense lines build small drift, gappy lines claw it back.
// As long as the average line fits within the speed budget, drift stays bounded.
func TestPlanClipTiming_SequenceStaysBounded(t *testing.T) {
	drift := 0.0
	maxSeen := 0.0
	for i := 0; i < 50; i++ {
		var p clipPlan
		if i%2 == 0 {
			// Dense line: 1.5s audio into a 1.0s window, no gap -> small drift.
			p = planClipTiming(1.0, 0, 1.5, drift, testMaxSpeed, testMaxDrift)
		} else {
			// Breathing room: short line with a big gap -> recovers drift.
			p = planClipTiming(1.0, 2.0, 0.5, drift, testMaxSpeed, testMaxDrift)
		}
		drift = p.NewDriftSec
		if drift > maxSeen {
			maxSeen = drift
		}
	}
	// Drift oscillates but never runs away; ends near zero after a recovery line.
	if maxSeen > 1.0 {
		t.Fatalf("max drift %.3f exceeded 1.0 — drift is running away", maxSeen)
	}
	if drift > 0.5 {
		t.Fatalf("final drift %.3f too high — recovery not working", drift)
	}
}
