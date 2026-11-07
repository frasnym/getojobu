package handler

// StatusResponse is the body returned by health/readiness endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}
