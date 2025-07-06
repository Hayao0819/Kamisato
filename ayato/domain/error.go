package domain

type APIError struct {
	Message string `json:"message,omitempty"`
	Reason  error  `json:"reason,omitempty"`
	// Code    int    `json:"code"`
}

func (e *APIError) Error() string {
	return e.Message
}
