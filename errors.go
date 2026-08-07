package openmerge

import "fmt"

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
