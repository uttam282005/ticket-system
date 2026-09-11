package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"ticket-system/internal/middleware"
	"ticket-system/internal/models"
	"ticket-system/internal/store"

	"github.com/go-chi/chi/v5"
)

type TicketHandler struct {
	store store.Store
}

func NewTicketHandler(store store.Store) *TicketHandler {
	return &TicketHandler{store: store}
}

func (h *TicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	ticket, err := h.store.CreateTicket(r.Context(), userID, title, req.Description)
	if err != nil {
		log.Printf("error creating ticket: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, ticket)
}

func (h *TicketHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tickets, err := h.store.GetTicketsByUserID(r.Context(), userID)
	if err != nil {
		log.Printf("error listing tickets: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if tickets == nil {
		tickets = make([]models.Ticket, 0)
	}

	writeJSON(w, http.StatusOK, models.TicketListResponse{
		Tickets: tickets,
	})
}

func (h *TicketHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	ticket, err := h.store.GetTicketByIDAndUserID(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "ticket not found")
			return
		}
		log.Printf("error getting ticket: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ticket)
}

func (h *TicketHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var req models.UpdateTicketStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Status = strings.TrimSpace(req.Status)
	if !models.IsValidStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid status value")
		return
	}

	ticket, err := h.store.UpdateTicketStatus(r.Context(), id, userID, req.Status)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "ticket not found")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "cannot update status of a closed ticket")
			return
		}
		log.Printf("error updating ticket status: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ticket)
}
