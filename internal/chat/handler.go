package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/giovanysievert/ask-anything/internal/httputil"
)

type Handler struct {
	svc      *Service
	logger   *slog.Logger
	validate *validator.Validate
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger, validate: validator.New()}
}

func (h *Handler) RegisterJSONRoutes(r chi.Router) {
	r.Post("/conversations", h.createConversation)
	r.Get("/conversations", h.listConversations)
	r.Get("/conversations/{id}/messages", h.listMessages)
}

func (h *Handler) RegisterStreamRoutes(r chi.Router) {
	r.Post("/conversations/{id}/messages", h.postMessage)
}

type CreateConversationRequest struct {
	Title string `json:"title" validate:"omitempty,max=255" example:"React Native study"`
}

type MessageRequest struct {
	Content string `json:"content" validate:"required,min=1" example:"How do I optimize a slow FlatList?"`
}

// createConversation godoc
//
//	@Summary		Create a conversation
//	@Description	Starts a new chat conversation.
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateConversationRequest	false	"Optional title"
//	@Success		201		{object}	Conversation
//	@Failure		400		{object}	httputil.ErrorResponse
//	@Failure		422		{object}	httputil.ErrorResponse
//	@Failure		500		{object}	httputil.ErrorResponse
//	@Router			/conversations [post]
func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	var req CreateConversationRequest
	if r.ContentLength != 0 {
		if err := httputil.ReadJSON(w, r, &req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if fields := h.validationErrors(req); fields != nil {
		httputil.WriteFieldErrors(w, fields)
		return
	}

	conv, err := h.svc.CreateConversation(r.Context(), req.Title)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create conversation")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, conv)
}

// listConversations godoc
//
//	@Summary		List conversations
//	@Description	Returns conversations ordered by most recently updated.
//	@Tags			chat
//	@Produce		json
//	@Success		200	{array}		Conversation
//	@Failure		500	{object}	httputil.ErrorResponse
//	@Router			/conversations [get]
func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := h.svc.ListConversations(r.Context(), 100, 0)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, convs)
}

// listMessages godoc
//
//	@Summary		List messages in a conversation
//	@Description	Returns the full message history of a conversation, oldest first.
//	@Tags			chat
//	@Produce		json
//	@Param			id	path		string	true	"Conversation ID"
//	@Success		200	{array}		Message
//	@Failure		400	{object}	httputil.ErrorResponse
//	@Failure		404	{object}	httputil.ErrorResponse
//	@Failure		500	{object}	httputil.ErrorResponse
//	@Router			/conversations/{id}/messages [get]
func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	messages, err := h.svc.ListMessages(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "conversation not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, messages)
}

// postMessage godoc
//
//	@Summary		Send a message and stream the reply (SSE)
//	@Description	Appends the user message, retrieves grounding chunks (RAG), and streams the assistant reply as Server-Sent Events. Emits `event: delta` frames with a JSON `{"text":"..."}` payload, a terminal `event: done` frame carrying the persisted assistant message, and `event: error` on failure.
//	@Tags			chat
//	@Accept			json
//	@Produce		text/event-stream
//	@Param			id		path	string			true	"Conversation ID"
//	@Param			request	body	MessageRequest	true	"User message"
//	@Success		200		{string}	string	"SSE stream"
//	@Failure		400		{object}	httputil.ErrorResponse
//	@Failure		404		{object}	httputil.ErrorResponse
//	@Failure		422		{object}	httputil.ErrorResponse
//	@Router			/conversations/{id}/messages [post]
func (h *Handler) postMessage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req MessageRequest
	if err := httputil.ReadJSON(w, r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fields := h.validationErrors(req); fields != nil {
		httputil.WriteFieldErrors(w, fields)
		return
	}

	if _, err := h.svc.ListMessages(r.Context(), id); err != nil {
		if errors.Is(err, ErrConversationNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "conversation not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load conversation")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	onDelta := func(text string) error {
		if err := writeSSE(w, "delta", map[string]string{"text": text}); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	assistant, err := h.svc.StreamTurn(r.Context(), id, req.Content, onDelta)
	if err != nil {
		h.logger.Error("chat turn failed", slog.Any("err", err), slog.String("conversation_id", id.String()))
		_ = writeSSE(w, "error", map[string]string{"message": "failed to generate reply"})
		flusher.Flush()
		return
	}

	_ = writeSSE(w, "done", assistant)
	flusher.Flush()
}

func writeSSE(w http.ResponseWriter, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	return err
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
