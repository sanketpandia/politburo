package pilots

// RegisterPilotRequest - minimal input for registration
type RegisterPilotRequest struct {
	IfcId      string `json:"ifc_id" validate:"required"`
	LastFlight string `json:"last_flight" validate:"required"`
}

// RegisterPilotResponse - minimal response with VA flag
type RegisterPilotResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	IsVARegistered bool   `json:"is_va_registered"` // Flag indicating if current server is a VA
}
