package positions

import "time"

type realTicker struct {
	*time.Ticker
}

func NewTicker(interval time.Duration) Ticker {
	return realTicker{time.NewTicker(interval)}
}

func (t realTicker) C() <-chan time.Time {
	return t.Ticker.C
}
