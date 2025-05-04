package httputil

import (
	api_models "buildmychat-backend/internal/models"
	"encoding/json"
	"log"
	"net/http"
)

// RespondJSON writes a JSON response with the given status code and payload.
func RespondJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		// Can't write header again here, just log the error
	}
}

// RespondError writes a JSON error response with the given status code and message.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	resp := api_models.ErrorResponse{Error: message}
	RespondJSON(w, statusCode, resp)
}
