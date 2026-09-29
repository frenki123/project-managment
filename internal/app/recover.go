package app

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recover prevents a handler panic from terminating the request server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				log.Printf("panic: %v\n%s", value, debug.Stack())
				WriteError(w, r, HTTPError{Status: http.StatusInternalServerError, Message: "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
