package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/giovanysievert/ask-anything/internal/httputil"
)

type Handler struct {
	svc      *Service
	validate *validator.Validate
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, validate: validator.New()}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Post("/", h.create)
		r.Get("/", h.list)
		r.Get("/{id}", h.getByID)
		r.Put("/{id}", h.update)
		r.Delete("/{id}", h.delete)
	})
}

type createRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=1,max=255"`
}

type updateRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=1,max=255"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httputil.ReadJSON(w, r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fields := h.validationErrors(req); fields != nil {
		httputil.WriteFieldErrors(w, fields)
		return
	}

	u, err := h.svc.Create(r.Context(), CreateInput{Email: req.Email, Name: req.Name})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, u)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	u, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 0)
	offset := parseIntQuery(r, "offset", 0)

	users, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req updateRequest
	if err := httputil.ReadJSON(w, r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fields := h.validationErrors(req); fields != nil {
		httputil.WriteFieldErrors(w, fields)
		return
	}

	u, err := h.svc.Update(r.Context(), id, UpdateInput{Email: req.Email, Name: req.Name})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeServiceError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httputil.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrEmailTaken):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	default:
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *Handler) validationErrors(v any) map[string]string {
	err := h.validate.Struct(v)
	if err == nil {
		return nil
	}
	fields := make(map[string]string)
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		for _, fe := range validationErrs {
			fields[fe.Field()] = "failed on rule: " + fe.Tag()
		}
	}
	return fields
}

func parseID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func parseIntQuery(r *http.Request, key string, fallback int32) int32 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return int32(n)
}
