package coupon

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrCouponExpired     = errors.New("coupon has expired")
	ErrCouponNotYetValid = errors.New("coupon is not yet valid")
)

type Coupon struct {
	id        uuid.UUID
	code      Code
	discount  Discount
	validFrom *time.Time
	validTo   *time.Time
	createdAt time.Time
	updatedAt time.Time
}

func NewCoupon(
	id uuid.UUID,
	code string,
	amountOffCents *int32,
	percentOff *float64,
	validFrom, validTo *time.Time,
) (*Coupon, error) {
	couponCode := Code(code)

	discount, err := NewDiscount(amountOffCents, percentOff)
	if err != nil {
		return nil, err
	}

	return &Coupon{
		id:        id,
		code:      couponCode,
		discount:  discount,
		validFrom: validFrom,
		validTo:   validTo,
	}, nil
}

func (c *Coupon) IsValidAt(t time.Time) bool {
	if c.validFrom != nil && t.Before(*c.validFrom) {
		return false
	}
	if c.validTo != nil && t.After(*c.validTo) {
		return false
	}
	return true
}

func (c *Coupon) ValidateUsage(t time.Time) error {
	if !c.IsValidAt(t) {
		if c.validFrom != nil && t.Before(*c.validFrom) {
			return ErrCouponNotYetValid
		}
		return ErrCouponExpired
	}
	return nil
}

func (c *Coupon) ApplyDiscount(basePriceCents int64) int64 {
	return c.discount.Apply(basePriceCents)
}

func (c *Coupon) ID() uuid.UUID         { return c.id }
func (c *Coupon) Code() Code            { return c.code }
func (c *Coupon) Discount() Discount    { return c.discount }
func (c *Coupon) ValidFrom() *time.Time { return c.validFrom }
func (c *Coupon) ValidTo() *time.Time   { return c.validTo }
func (c *Coupon) CreatedAt() time.Time  { return c.createdAt }
func (c *Coupon) UpdatedAt() time.Time  { return c.updatedAt }
