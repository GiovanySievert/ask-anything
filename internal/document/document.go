package document

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

var ErrNotFound = errors.New("document not found")
