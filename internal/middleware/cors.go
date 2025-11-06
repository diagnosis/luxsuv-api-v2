package middleware

import (
	"fmt"
	"net/http"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
		},
		AllowCredentials: true,
		MaxAge:           300,
	}
}

func CORS(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				allowed := false
				for _, o := range config.AllowedOrigins {
					if o == "*" || o == origin {
						allowed = true
						break
					}
				}

				if allowed {
					if len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*" {
						w.Header().Set("Access-Control-Allow-Origin", "*")
					} else {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}

					if config.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}

					if len(config.ExposedHeaders) > 0 {
						w.Header().Set("Access-Control-Expose-Headers", joinHeaders(config.ExposedHeaders))
					}
				}
			}

			if r.Method == http.MethodOptions {
				if len(config.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", joinHeaders(config.AllowedMethods))
				}
				if len(config.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", joinHeaders(config.AllowedHeaders))
				}
				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func joinHeaders(headers []string) string {
	result := ""
	for i, h := range headers {
		if i > 0 {
			result += ", "
		}
		result += h
	}
	return result
}
