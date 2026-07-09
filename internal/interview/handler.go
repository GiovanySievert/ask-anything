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

// QuestionRequest is the body for POST /questions.
type QuestionRequest struct {
	Topic string `json:"topic" validate:"required,min=1,max=255" example:"react native flatlist"`
	Level string `json:"level" validate:"required,min=1,max=50" example:"senior"`
}

// AnswerRequest is the body for POST /answers.
type AnswerRequest struct {
	Question string `json:"question" validate:"required,min=1" example:"How would you optimize a slow FlatList?"`
	Answer   string `json:"answer" validate:"required,min=1" example:"Use getItemLayout, memoize renderItem, switch to FlashList."`
}

// generateQuestion godoc
//
//	@Summary		Generate an interview question (RAG)
//	@Description	Embeds the topic, finds the most similar ingested chunks, and asks Claude for one question grounded in them.
//	@Tags			interview
//	@Accept			json
//	@Produce		json
//	@Param			request	body		QuestionRequest	true	"Topic and level"
//	@Success		200		{object}	Question
//	@Failure		400		{object}	httputil.ErrorResponse
//	@Failure		422		{object}	httputil.ErrorResponse
//	@Failure		500		{object}	httputil.ErrorResponse
//	@Router			/questions [post]
func (h *Handler) generateQuestion(w http.ResponseWriter, r *http.Request) {
	var req QuestionRequest
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

// evaluateAnswer godoc
//
//	@Summary		Evaluate a candidate's answer
//	@Description	Asks Claude to score the answer and return structured feedback plus a follow-up question.
//	@Tags			interview
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AnswerRequest	true	"Question and the candidate's answer"
//	@Success		200		{object}	Evaluation
//	@Failure		400		{object}	httputil.ErrorResponse
//	@Failure		422		{object}	httputil.ErrorResponse
//	@Failure		500		{object}	httputil.ErrorResponse
//	@Router			/answers [post]
func (h *Handler) evaluateAnswer(w http.ResponseWriter, r *http.Request) {
	var req AnswerRequest
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
