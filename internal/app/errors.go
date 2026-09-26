package app

import (
	"errors"
	"log"
	"net/http"
)

// HTTPError is the transport-facing error used by HTTP adapters.
type HTTPError struct {
	Status  int
	Message string
}

func (e HTTPError) Error() string {
	return e.Message
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		if IsAPI(r) {
			JSON(w, httpErr.Status, map[string]string{"error": httpErr.Message})
			return
		}
		http.Error(w, httpErr.Message, httpErr.Status)
		return
	}
	log.Println(err)
	if IsAPI(r) {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}
