package coupon

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidCouponCode      = errors.New("invalid coupon code format")
	ErrInvalidDiscountAmount  = errors.New("discount amount cannot be negative")
	ErrInvalidDiscountPercent = errors.New("percentage discount must be between 0 and 100")
)

var couponCodeRegex = regexp.MustCompile(`^[A-Z0-9]{3,20}$`)

type Code string

func NewCouponCode(code string) (Code, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if !couponCodeRegex.MatchString(code) {
		return Code(""), ErrInvalidCouponCode
	}
	return Code(code), nil
}

func (c Code) String() string {
	return string(c)
}

type Discount struct {
	amountOffCents *int
	percentOff     *float64
}

func NewFixedDiscount(amountOffCents int) (Discount, error) {
	if amountOffCents < 0 {
		return Discount{}, ErrInvalidDiscountAmount
	}
	return Discount{amountOffCents: &amountOffCents}, nil
}

func NewPercentageDiscount(percentOff float64) (Discount, error) {
	if percentOff < 0 || percentOff > 100 {
		return Discount{}, ErrInvalidDiscountPercent
	}
	return Discount{percentOff: &percentOff}, nil
}

func (d Discount) IsPercentage() bool {
	return d.percentOff != nil
}

func (d Discount) IsFixed() bool {
	return d.amountOffCents != nil
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

func NewDiscount(amountOffCents *int32, percentOff *float64) (Discount, error) {
	if amountOffCents != nil && percentOff != nil {
		return Discount{}, errors.New("discount can only be either fixed amount or percentage, not both")
	}

	if amountOffCents == nil && percentOff == nil {
		return Discount{}, errors.New("discount must have either fixed amount or percentage")
	}

	if amountOffCents != nil {
		amount := int(*amountOffCents)
		return NewFixedDiscount(amount)
	}

	return NewPercentageDiscount(*percentOff)
}

func (d Discount) Apply(basePriceCents int64) int64 {
	discountAmount := d.CalculateDiscountAmount(int(basePriceCents))
	result := basePriceCents - int64(discountAmount)
	if result < 0 {
		return 0
	}
	return result
}

func (d Discount) CalculateDiscountAmount(priceCents int) int {
	if d.IsPercentage() {
		discountAmount := float64(priceCents) * (d.PercentOff() / 100.0)
		return int(discountAmount)
	}

	// For fixed discount, return the minimum of discount amount and price
	if d.AmountOffCents() > priceCents {
		return priceCents // Cannot discount more than the original price
	}
	return d.AmountOffCents()
}
