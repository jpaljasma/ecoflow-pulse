package ecoflow

import (
	"errors"
	"fmt"
)

// HTTPError reports a non-2xx HTTP response from EcoFlow.
type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("ecoflow api returned status %d", e.StatusCode)
}

// RetryExhaustedError is returned when retry attempts are consumed.
type RetryExhaustedError struct {
	Attempts int
	LastErr  error
}

func (e *RetryExhaustedError) Error() string {
	return fmt.Sprintf("ecoflow request failed after %d attempts: %v", e.Attempts, e.LastErr)
}

func (e *RetryExhaustedError) Unwrap() error {
	return e.LastErr
}

// BusinessError reports an EcoFlow business-level response failure for a
// successful HTTP status where code != "0".
type BusinessError struct {
	Code    string
	Message string
}

func (e *BusinessError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("ecoflow api business error code=%s", e.Code)
	}
	return fmt.Sprintf("ecoflow api business error code=%s message=%s", e.Code, e.Message)
}

// IsBusinessErrorCode reports whether err contains a BusinessError with the
// provided code value.
func IsBusinessErrorCode(err error, code string) bool {
	var businessErr *BusinessError
	if !errors.As(err, &businessErr) {
		return false
	}
	return businessErr.Code == code
}
