package utils

import (
	"encoding/json"
	"net/http"
)

// Response represents a standardized API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSONResponse sends a JSON response with the given status code and data
func JSONResponse(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

// SuccessResponse sends a successful JSON response
func SuccessResponse(w http.ResponseWriter, data interface{}) error {
	response := Response{
		Success: true,
		Data:    data,
	}
	return JSONResponse(w, http.StatusOK, response)
}

// SuccessWithMessage sends a successful JSON response with a message
func SuccessWithMessage(w http.ResponseWriter, message string, data interface{}) error {
	response := Response{
		Success: true,
		Message: message,
		Data:    data,
	}
	return JSONResponse(w, http.StatusOK, response)
}

// ErrorResponse sends an error JSON response
func ErrorResponse(w http.ResponseWriter, statusCode int, message string) error {
	response := Response{
		Success: false,
		Error:   message,
	}
	return JSONResponse(w, statusCode, response)
}

// BadRequest sends a 400 Bad Request response
func BadRequest(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Bad request"
	}
	return ErrorResponse(w, http.StatusBadRequest, message)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Internal server error"
	}
	return ErrorResponse(w, http.StatusInternalServerError, message)
}

// NotFound sends a 404 Not Found response
func NotFound(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Resource not found"
	}
	return ErrorResponse(w, http.StatusNotFound, message)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Unauthorized"
	}
	return ErrorResponse(w, http.StatusUnauthorized, message)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(w http.ResponseWriter, message string) error {
	if message == "" {
		message = "Forbidden"
	}
	return ErrorResponse(w, http.StatusForbidden, message)
}