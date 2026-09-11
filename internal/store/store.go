package store

import (
	"context"
	"errors"

	"ticket-system/internal/models"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrDuplicateEmail    = errors.New("email already registered")
	ErrInvalidTransition = errors.New("invalid status transition")
)

type Store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)

	CreateTicket(ctx context.Context, userID, title string, description *string) (*models.Ticket, error)
	GetTicketsByUserID(ctx context.Context, userID string) ([]models.Ticket, error)
	GetTicketByIDAndUserID(ctx context.Context, id, userID string) (*models.Ticket, error)
	UpdateTicketStatus(ctx context.Context, id, userID, newStatus string) (*models.Ticket, error)
}
