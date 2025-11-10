package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/apperror"
	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
	"github.com/diagnosis/luxsuv-api-v2/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	UserStore              store.UserStore
	DriverApplicationStore store.DriverApplicationStore
}

func NewAdminHandler(us store.UserStore, das store.DriverApplicationStore) *AdminHandler {
	return &AdminHandler{UserStore: us, DriverApplicationStore: das}
}

func (h *AdminHandler) HandleCreateUserWithRole(w http.ResponseWriter, r *http.Request) {
	claims, err := secure.ClaimsFromContext(r.Context())
	if err != nil || strings.ToLower(claims.Role) != "super_admin" {
		helper.RespondError(w, r, apperror.Forbidden("super_admin required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid body"))
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))
	role := strings.ToLower(strings.TrimSpace(body.Role))
	if len(email) < 4 || len(body.Password) < 8 {
		helper.RespondError(w, r, apperror.BadRequest("email >= 4, password >= 8"))
		return
	}
	switch role {
	case "rider", "driver", "admin", "super_admin":
	default:
		helper.RespondError(w, r, apperror.BadRequest("invalid role"))
		return
	}
	hash, err := secure.HashPassword(body.Password)
	if err != nil {
		helper.RespondError(w, r, apperror.InternalError("hash error", err))
		return
	}

	id, err := h.UserStore.CreateUserWithRole(ctx, email, hash, role)
	if err != nil {
		logger.Error(ctx, "create user with role failed", "error", err)
		helper.RespondError(w, r, apperror.InternalError("create failed", err))
		return
	}
	helper.RespondJSON(w, r, http.StatusCreated, map[string]any{
		"id": id, "email": email, "role": role,
	})
}

func (h *AdminHandler) HandleSetUserRole(w http.ResponseWriter, r *http.Request) {
	claims, err := secure.ClaimsFromContext(r.Context())
	if err != nil || strings.ToLower(claims.Role) != "super_admin" {
		helper.RespondError(w, r, apperror.Forbidden("super_admin required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	uidStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid user id"))
		return
	}

	var body struct {
		Role string `json:"role"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid body"))
		return
	}

	role := strings.ToLower(strings.TrimSpace(body.Role))
	switch role {
	case "rider", "driver", "admin", "super_admin":
	default:
		helper.RespondError(w, r, apperror.BadRequest("invalid role"))
		return
	}

	if err := h.UserStore.(*store.PostgresUserStore).SetUserRole(ctx, userID, role); err != nil {
		helper.RespondError(w, r, apperror.InternalError("set role failed", err))
		return
	}

	helper.RespondMessage(w, r, http.StatusOK, "role updated")
}
func (h *AdminHandler) HandleReviewDriverApplication(w http.ResponseWriter, r *http.Request) {
	// We expect middleware to have enforced admin/super_admin.
	// Keep a light assert (optional).
	if claims, err := secure.ClaimsFromContext(r.Context()); err != nil || claims == nil {
		helper.RespondError(w, r, apperror.InternalError("claims missing; auth middleware not applied", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// application id in path
	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid application id"))
		return
	}

	var body struct {
		Action string `json:"action"`          // "approve" | "reject"
		Notes  string `json:"notes,omitempty"` // optional reviewer notes
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		helper.RespondError(w, r, apperror.BadRequest("invalid body"))
		return
	}

	action := strings.ToLower(strings.TrimSpace(body.Action))
	if action != "approve" && action != "reject" {
		helper.RespondError(w, r, apperror.BadRequest(`action must be "approve" or "reject"`))
		return
	}

	appRec, err := h.DriverApplicationStore.GetByID(ctx, appID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			helper.RespondError(w, r, apperror.NotFound("application not found"))
			return
		}
		helper.RespondError(w, r, apperror.InternalError("failed to fetch application", err))
		return
	}

	if body.Notes != "" {
		if err := h.DriverApplicationStore.UpdateNotes(ctx, appID, body.Notes); err != nil {
			helper.RespondError(w, r, apperror.InternalError("failed to save notes", err))
			return
		}
	}

	switch action {
	case "approve":
		if err := h.DriverApplicationStore.UpdateStatus(ctx, appID, store.DriverApproved); err != nil {
			helper.RespondError(w, r, apperror.InternalError("failed to set approved", err))
			return
		}
		// Activate user and set role=driver (idempotent if already driver)
		if err := h.UserStore.ActivateUser(ctx, appRec.UserID); err != nil {
			helper.RespondError(w, r, apperror.InternalError("failed to activate user", err))
			return
		}
		// Optional: enforce driver role
		if setter, ok := h.UserStore.(interface {
			SetUserRole(context.Context, uuid.UUID, string) error
		}); ok {
			_ = setter.SetUserRole(ctx, appRec.UserID, "driver")
		}

		helper.RespondJSON(w, r, http.StatusOK, map[string]any{
			"application_id": appID,
			"status":         "approved",
		})

	case "reject":
		if err := h.DriverApplicationStore.UpdateStatus(ctx, appID, store.DriverRejected); err != nil {
			helper.RespondError(w, r, apperror.InternalError("failed to set rejected", err))
			return
		}
		// Optional: deactivate user if you don’t want rejected applicants to log in
		// _ = h.UserStore.DeactivateUser(ctx, appRec.UserID)

		helper.RespondJSON(w, r, http.StatusOK, map[string]any{
			"application_id": appID,
			"status":         "rejected",
		})
	}
}
