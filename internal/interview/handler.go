package interview

import (
	"errors"
	"net/http"

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
	r.Post("/questions", h.generateQuestion)
	r.Post("/answers", h.evaluateAnswer)
}

type questionRequest struct {
	Topic string `json:"topic" validate:"required,min=1,max=255"`
	Level string `json:"level" validate:"required,min=1,max=50"`
}

type answerRequest struct {
	Question string `json:"question" validate:"required,min=1"`
	Answer   string `json:"answer" validate:"required,min=1"`
}

func (h *Handler) generateQuestion(w http.ResponseWriter, r *http.Request) {
	var req questionRequest
	if err := httputil.ReadJSON(w, r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fields := h.validationErrors(req); fields != nil {
		httputil.WriteFieldErrors(w, fields)
		return
	}

	question, err := h.svc.GenerateQuestion(r.Context(), req.Topic, req.Level)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate question")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, question)
}

func (h *Handler) evaluateAnswer(w http.ResponseWriter, r *http.Request) {
	var req answerRequest
	if err := httputil.ReadJSON(w, r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fields := h.validationErrors(req); fields != nil {
		httputil.WriteFieldErrors(w, fields)
		return
	}

	eval, err := h.svc.EvaluateAnswer(r.Context(), req.Question, req.Answer)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to evaluate answer")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, eval)
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
