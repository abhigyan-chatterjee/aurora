package run

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestOutput_CapturesStdout(t *testing.T) {
	out, err := Output(5*time.Second, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(out); got != "hello\n" {
		t.Errorf("output = %q, want %q", got, "hello\n")
	}
}

func TestOutput_TimeoutIsReported(t *testing.T) {
	start := time.Now()
	_, err := Output(150*time.Millisecond, "sleep", "5")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("timeout took %s; the deadline was not enforced", elapsed)
	}
}

// pacman's documented "no matches" exit code must read as an empty result,
// while every other failure must stay an error. Getting this wrong once meant
// a broken database looked like an empty one.
func TestOutputOrEmpty_TreatsExitOneAsEmpty(t *testing.T) {
	out, err := OutputOrEmpty(5*time.Second, "sh", "-c", "exit 1")
	if err != nil {
		t.Fatalf("exit 1 should be treated as an empty result, got: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected no output, got %q", out)
	}
}

func TestOutputOrEmpty_KeepsRealErrors(t *testing.T) {
	if _, err := OutputOrEmpty(5*time.Second, "sh", "-c", "exit 2"); err == nil {
		t.Fatal("exit 2 must remain an error")
	}
	if _, err := OutputOrEmpty(5*time.Second, "definitely-not-a-real-binary-xyz"); err == nil {
		t.Fatal("a missing binary must remain an error")
	}
}

func TestIsExitCode(t *testing.T) {
	_, err := Output(5*time.Second, "sh", "-c", "exit 3")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsExitCode(err, 3) {
		t.Errorf("expected IsExitCode(err, 3) to be true, got: %v", err)
	}
	if IsExitCode(err, 1) {
		t.Error("expected IsExitCode(err, 1) to be false")
	}
}

func TestParallel_BoundsConcurrency(t *testing.T) {
	const (
		items   = 40
		workers = 4
	)

	var inFlight, peak int32
	if err := Parallel(items, workers, func(i int) error {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if peak > workers {
		t.Errorf("peak concurrency = %d, want at most %d", peak, workers)
	}
	if peak < 2 {
		t.Errorf("peak concurrency = %d; expected the work to actually run in parallel", peak)
	}
}

func TestParallel_VisitsEveryItemOnce(t *testing.T) {
	var seen [64]int32
	if err := Parallel(64, 8, func(i int) error {
		atomic.AddInt32(&seen[i], 1)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, count := range seen {
		if count != 1 {
			t.Errorf("item %d visited %d times, want 1", i, count)
		}
	}
}

func TestParallel_ReturnsFirstError(t *testing.T) {
	err := Parallel(10, 2, func(i int) error {
		if i == 7 {
			return errSentinel
		}
		return nil
	})
	if err != errSentinel {
		t.Errorf("expected the sentinel error, got %v", err)
	}
}

func TestParallel_ZeroAndNegative(t *testing.T) {
	if err := Parallel(0, 4, func(int) error { t.Fatal("should not run"); return nil }); err != nil {
		t.Errorf("unexpected error for zero items: %v", err)
	}
	if err := Parallel(2, 0, func(int) error { return nil }); err != nil {
		t.Errorf("unexpected error for zero workers: %v", err)
	}
}

func TestExists(t *testing.T) {
	if !Exists("sh") {
		t.Error("expected sh to exist on PATH")
	}
	if Exists("definitely-not-a-real-binary-xyz") {
		t.Error("expected a nonexistent binary to report false")
	}
}
