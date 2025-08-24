package reservation

type Status string

const (
	StatusConfirmed Status = "confirmed"
	StatusCanceled  Status = "canceled"
)

func (s Status) String() string {
	return string(s)
}

func (s Status) IsValid() bool {
	switch s {
	case StatusConfirmed, StatusCanceled:
		return true
	default:
		return false
	}
}
