package reservation

import (
	"strings"
	"time"

	"gin-clean-starter/internal/pkg/errs"
)

type TimeSlot struct {
	start time.Time
	end   time.Time
}

func NewTimeSlot(start, end time.Time) (TimeSlot, error) {
	if start.After(end) || start.Equal(end) {
		return TimeSlot{}, errs.New("start time must be before end time")
	}

	return TimeSlot{
		start: start,
		end:   end,
	}, nil
}

func (ts TimeSlot) Start() time.Time {
	return ts.start
}

func (ts TimeSlot) End() time.Time {
	return ts.end
}

func (ts TimeSlot) Duration() time.Duration {
	return ts.end.Sub(ts.start)
}

func (ts TimeSlot) MeetsLeadTimeAt(now time.Time, leadTimeMinutes int) bool {
	requiredTime := now.Add(time.Duration(leadTimeMinutes) * time.Minute)
	return ts.start.After(requiredTime)
}

func (ts TimeSlot) ValidateLeadTimeAt(now time.Time, leadTimeMinutes int) error {
	if !ts.MeetsLeadTimeAt(now, leadTimeMinutes) {
		return errs.New("insufficient lead time")
	}
	return nil
}

type Money struct {
	cents int
}

func NewMoney(cents int64) Money {
	return Money{cents: int(cents)}
}

func NewMoneyFromInt(cents int) (Money, error) {
	if cents < 0 {
		return Money{}, errs.New("money cannot be negative")
	}
	return Money{cents: cents}, nil
}

func (m Money) Cents() int {
	return m.cents
}

func (m Money) Dollars() float64 {
	return float64(m.cents) / 100.0
}

func (m Money) Add(other Money) Money {
	return Money{cents: m.cents + other.cents}
}

const MaxNoteLength = 1000

var ErrNoteTooLong = errs.New("note exceeds maximum length")

type Note struct {
	value string
}

func NewNote(value string) (Note, error) {
	v := strings.TrimSpace(value)
	if len(v) > MaxNoteLength {
		return Note{}, ErrNoteTooLong
	}
	return Note{value: v}, nil
}

func (n Note) String() string {
	return n.value
}

func (n Note) IsEmpty() bool {
	return n.value == ""
}
