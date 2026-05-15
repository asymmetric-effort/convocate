package protocol

// HealthResponse is the response returned by GET /v1/health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}
