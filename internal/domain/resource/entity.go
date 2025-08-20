package resource

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEmptyResourceName   = errors.New("resource name cannot be empty")
	ErrNegativeLeadTime    = errors.New("lead time cannot be negative")
	ErrResourceNameTooLong = errors.New("resource name is too long (max 255 characters)")
)

const (
	MaxResourceNameLength = 255
)

type Resource struct {
	id          uuid.UUID
	name        string
	leadTimeMin int
	createdAt   time.Time
	updatedAt   time.Time
}

func NewResource(id uuid.UUID, name string, leadTimeMin int) (*Resource, error) {
	if err := validateResourceName(name); err != nil {
		return nil, err
	}

	if err := validateLeadTime(leadTimeMin); err != nil {
		return nil, err
	}

	return &Resource{
		id:          id,
		name:        strings.TrimSpace(name),
		leadTimeMin: leadTimeMin,
	}, nil
}

func (r *Resource) IsBookableAt(bookingTime time.Time) bool {
	requiredTime := time.Now().Add(time.Duration(r.leadTimeMin) * time.Minute)
	return bookingTime.After(requiredTime)
}

func validateResourceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyResourceName
	}
	if len(name) > MaxResourceNameLength {
		return ErrResourceNameTooLong
	}
	return nil
}

func validateLeadTime(leadTimeMin int) error {
	if leadTimeMin < 0 {
		return ErrNegativeLeadTime
	}
	return nil
}

func (r *Resource) ID() uuid.UUID        { return r.id }
func (r *Resource) Name() string         { return r.name }
func (r *Resource) LeadTimeMin() int     { return r.leadTimeMin }
func (r *Resource) CreatedAt() time.Time { return r.createdAt }
func (r *Resource) UpdatedAt() time.Time { return r.updatedAt }
