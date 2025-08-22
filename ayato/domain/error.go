package domain

// APIError represents API error information.
type APIError struct {
	Message string `json:"message,omitempty"`
	Reason  error  `json:"reason,omitempty"`
	// Code    int    `json:"code"`
}

// Error returns the error message.
func (e *APIError) Error() string {
	return e.Message
}
