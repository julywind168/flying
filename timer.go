package flying

import "time"

type Timer struct {
	Elapsed  time.Duration
	Interval time.Duration
}

func (t *Timer) Tick(delta time.Duration) *Timer {
	t.Elapsed += delta
	return t
}

func (t *Timer) Reset() {
	t.Elapsed = 0
}

func (t *Timer) JustFinished() bool {
	if t.Elapsed >= t.Interval {
		t.Elapsed -= t.Interval
		return true
	} else {
		return false
	}
}
