package infra

import (
	"errors"
	"fmt"
)

// ErrResourceUnavailable is returned when a configured resource cannot be
// opened, or when a caller asks for a resource that was not configured.
var ErrResourceUnavailable = errors.New("ezboot: resource unavailable")

// ResourceError describes which resource failed and why.
//
// Callers can use errors.Is(err, ErrResourceUnavailable) to distinguish a
// missing/unavailable dependency from a business error.
type ResourceError struct {
	Kind string
	Name string

	Err error
}

func (e *ResourceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ezboot: %s %q unavailable: %v", e.Kind, e.Name, e.Err)
	}
	return fmt.Sprintf("ezboot: %s %q unavailable", e.Kind, e.Name)
}

func (e *ResourceError) Unwrap() error {
	return e.Err
}

func (e *ResourceError) Is(target error) bool {
	return target == ErrResourceUnavailable
}

func resourceError(kind, name string, err error) error {
	return &ResourceError{Kind: kind, Name: name, Err: err}
}
