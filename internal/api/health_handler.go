package api

import (
	"net/http"

	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	helper.RespondMessage(w, r, 200, "Luxsuv sever seems healthy")
}
