package models

import "time"

const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusClosed     = "closed"
)

func IsValidStatus(status string) bool {
	return status == StatusOpen || status == StatusInProgress || status == StatusClosed
}

type Ticket struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateTicketRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
}

type TicketListResponse struct {
	Tickets []Ticket `json:"tickets"`
}

type UpdateTicketStatusRequest struct {
	Status string `json:"status"`
}
