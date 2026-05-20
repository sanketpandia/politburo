package servers

type InitServerRequest struct {
	VACode string `json:"va_code" validate:"required"`
}

type InitServerResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	VACode        string `json:"va_code"`
	SetupRequired bool   `json:"setup_required"`
	DashboardURL  string `json:"dashboard_url,omitempty"`
	SetupURL      string `json:"setup_url,omitempty"`
}
