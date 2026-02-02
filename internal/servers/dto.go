package servers

type InitServerRequest struct {
	VACode         string `json:"va_code" validate:"required"`
	VAName         string `json:"va_name" validate:"required"`
	CallsignPrefix string `json:"callsign_prefix"`
	CallsignSuffix string `json:"callsign_suffix"`
}

type InitServerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	VACode  string `json:"va_code"`
	VAID    string `json:"va_id,omitempty"`
}
