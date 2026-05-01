package handlers

import (
	"net/http"
)

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "The requested resource was not found")
}
