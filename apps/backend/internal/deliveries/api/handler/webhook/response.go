package webhook

// StatusResponse is the body returned by webhook endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}
