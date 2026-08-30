package openmerge

import (
	"context"
	"fmt"
	"time"
)

type APIError struct {
	StatusCode int
	RequestID  string
	Retryable  bool
	Details    any
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openmerge: %s (status=%d request_id=%s)", e.Message, e.StatusCode, e.RequestID)
}

type RequestTimeoutError struct {
	WriteID string
	Timeout time.Duration
}

func (e *RequestTimeoutError) Error() string {
	return fmt.Sprintf(
		"openmerge: writeback %s did not reach a terminal state within %s",
		e.WriteID,
		e.Timeout,
	)
}

func (e *RequestTimeoutError) Unwrap() error {
	return context.DeadlineExceeded
}
