package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/term-paper-2026/backend/internal/logging"
	"github.com/term-paper-2026/backend/internal/models"
	"github.com/term-paper-2026/backend/internal/repository"
	"github.com/term-paper-2026/backend/internal/auth"
)

type PatientHandler struct {
	repo *repository.PatientRepo
}

func NewPatientHandler(r *repository.PatientRepo) *PatientHandler {
	return &PatientHandler{repo: r}
}

// Create — POST /api/patients
func (h *PatientHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	log := logging.FromContext(ctx).With(slog.String("handler", "patient.Create"))

	var in models.PatientCreate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if in.FirstName == "" || in.LastName == "" {
		writeError(ctx, w, http.StatusBadRequest, "first_name and last_name are required")
		return
	}

	log.LogAttrs(ctx, slog.LevelInfo, "creating patient",
		slog.String("first_name", in.FirstName),
		slog.String("last_name", in.LastName),
	)

	p, err := h.repo.Create(ctx, in, userID)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}

	log.LogAttrs(ctx, slog.LevelInfo, "patient created", slog.Int("patient_id", p.PatientID))
	writeJSON(ctx, w, http.StatusCreated, p)
}

// Update — PATCH /api/patients/{id}
func (h *PatientHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad id")
		return
	}
	log := logging.FromContext(ctx).With(
		slog.String("handler", "patient.Update"),
		slog.Int("patient_id", id),
	)

	var in models.PatientUpdate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	p, err := h.repo.Update(ctx, id, in, userID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(ctx, w, http.StatusNotFound, "patient not found")
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	log.LogAttrs(ctx, slog.LevelInfo, "patient updated")
	writeJSON(ctx, w, http.StatusOK, p)
}

// Get — GET /api/patients/{id}
func (h *PatientHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad id")
		return
	}
	p, err := h.repo.Get(ctx, id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(ctx, w, http.StatusNotFound, "patient not found")
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(ctx, w, http.StatusOK, p)
}

// List — GET /api/patients?limit=&offset=&q=
func (h *PatientHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	q := r.URL.Query().Get("q")
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	log := logging.FromContext(ctx).With(
		slog.String("handler", "patient.List"),
		slog.Int("limit", limit), slog.Int("offset", offset),
		slog.String("q", q),
	)
	log.LogAttrs(ctx, slog.LevelDebug, "listing patients")

	items, total, err := h.repo.List(ctx, userID, q, limit, offset)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}

	log.LogAttrs(ctx, slog.LevelInfo, "patients listed",
		slog.Int("count", len(items)), slog.Int("total", total))
	writeJSON(ctx, w, http.StatusOK, models.PageResponse{
		Items: items, Total: total, Limit: limit, Offset: offset,
	})
}

// Delete — DELETE /api/patients/{id}
func (h *PatientHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFrom(ctx)
	if userID == 0 {
		writeError(ctx, w, http.StatusUnauthorized, "no user in context")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "bad id")
		return
	}
	if err := h.repo.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(ctx, w, http.StatusNotFound, "patient not found")
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
