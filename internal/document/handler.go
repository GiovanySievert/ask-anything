package document

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

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
	r.Route("/documents", func(r chi.Router) {
		r.Post("/", h.create)
		r.Get("/", h.list)
	})
}

type createRequest struct {
	Title   string `json:"title" validate:"required,min=1,max=255"`
	Content string `json:"content" validate:"required,min=1"`
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

	doc, err := h.svc.Create(r.Context(), CreateInput{Title: req.Title, Content: req.Content})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to ingest document")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, doc)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 0)
	offset := parseIntQuery(r, "offset", 0)

	docs, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, docs)
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
