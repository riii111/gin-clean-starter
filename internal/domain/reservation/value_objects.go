package reservation

import (
	"errors"
	"fmt"
	"time"
)

type TimeSlot struct {
	start time.Time
	end   time.Time
}

func NewTimeSlot(start, end time.Time) (TimeSlot, error) {
	if start.After(end) || start.Equal(end) {
		return TimeSlot{}, errors.New("start time must be before end time")
	}

	if start.Before(time.Now()) {
		return TimeSlot{}, errors.New("start time cannot be in the past")
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

func (ts TimeSlot) ToTstzrange() string {
	return fmt.Sprintf("[%s,%s)", ts.start.Format(time.RFC3339), ts.end.Format(time.RFC3339))
}

func (ts TimeSlot) MeetsLeadTime(leadTimeMinutes int) bool {
	requiredTime := time.Now().Add(time.Duration(leadTimeMinutes) * time.Minute)
	return ts.start.After(requiredTime)
}

func (ts TimeSlot) ValidateLeadTime(leadTimeMinutes int) error {
	if !ts.MeetsLeadTime(leadTimeMinutes) {
		return errors.New("insufficient lead time")
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
		return Money{}, errors.New("money cannot be negative")
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

func (m Money) ApplyDiscount(discount Discount) Money {
	if discount.IsPercentage() {
		discountAmount := float64(m.cents) * (discount.PercentOff() / 100.0)
		return Money{cents: m.cents - int(discountAmount)}
	}

	remaining := m.cents - discount.AmountOffCents()
	if remaining < 0 {
		remaining = 0
	}
	return Money{cents: remaining}
}

type Discount struct {
	amountOffCents *int
	percentOff     *float64
}

func NewFixedDiscount(amountOffCents int) (Discount, error) {
	if amountOffCents < 0 {
		return Discount{}, errors.New("discount amount cannot be negative")
	}
	return Discount{amountOffCents: &amountOffCents}, nil
}

func NewPercentageDiscount(percentOff float64) (Discount, error) {
	if percentOff < 0 || percentOff > 100 {
		return Discount{}, errors.New("percentage discount must be between 0 and 100")
	}
	return Discount{percentOff: &percentOff}, nil
}

func (d Discount) IsPercentage() bool {
	return d.percentOff != nil
}

func (d Discount) AmountOffCents() int {
	if d.amountOffCents != nil {
		return *d.amountOffCents
	}
	return 0
}

func (d Discount) PercentOff() float64 {
	if d.percentOff != nil {
		return *d.percentOff
	}
	return 0
}

type Note struct {
	value string
}

func NewNote(value string) Note {
	return Note{value: value}
}

func (n Note) String() string {
	return n.value
}

func (n Note) IsEmpty() bool {
	return n.value == ""
}
