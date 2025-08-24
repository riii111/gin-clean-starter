package clock

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func NewRealClock() Clock {
	return &RealClock{}
}

func (c *RealClock) Now() time.Time {
	return time.Now()
}

type MockClock struct {
	currentTime time.Time
}

func NewMockClock(t time.Time) *MockClock {
	return &MockClock{currentTime: t}
}

func (c *MockClock) Now() time.Time {
	return c.currentTime
}

func (c *MockClock) Set(t time.Time) {
	c.currentTime = t
}

func (c *MockClock) Add(d time.Duration) {
	c.currentTime = c.currentTime.Add(d)
}
