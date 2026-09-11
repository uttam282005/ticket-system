package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ticket-system/internal/auth"
	"ticket-system/internal/middleware"
	"ticket-system/internal/models"
	"ticket-system/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type memoryStore struct {
	mu        sync.RWMutex
	users     map[string]*models.User
	usersByID map[string]*models.User
	tickets   map[string]*models.Ticket
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:     make(map[string]*models.User),
		usersByID: make(map[string]*models.User),
		tickets:   make(map[string]*models.Ticket),
	}
}

func (m *memoryStore) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[email]; exists {
		return nil, store.ErrDuplicateEmail
	}

	user := &models.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	m.users[email] = user
	m.usersByID[user.ID] = user
	return user, nil
}

func (m *memoryStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[email]
	if !exists {
		return nil, store.ErrNotFound
	}
	return user, nil
}

func (m *memoryStore) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.usersByID[id]
	if !exists {
		return nil, store.ErrNotFound
	}
	return user, nil
}

func (m *memoryStore) CreateTicket(ctx context.Context, userID, title string, description *string) (*models.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ticket := &models.Ticket{
		ID:          uuid.NewString(),
		UserID:      userID,
		Title:       title,
		Description: description,
		Status:      models.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.tickets[ticket.ID] = ticket
	return ticket, nil
}

func (m *memoryStore) GetTicketsByUserID(ctx context.Context, userID string) ([]models.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]models.Ticket, 0)
	for _, t := range m.tickets {
		if t.UserID == userID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *memoryStore) GetTicketByIDAndUserID(ctx context.Context, id, userID string) (*models.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.tickets[id]
	if !exists || t.UserID != userID {
		return nil, store.ErrNotFound
	}
	return t, nil
}

func (m *memoryStore) UpdateTicketStatus(ctx context.Context, id, userID, newStatus string) (*models.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.tickets[id]
	if !exists || t.UserID != userID {
		return nil, store.ErrNotFound
	}

	// Rule: Once a ticket is closed, no further status changes are allowed
	if t.Status == models.StatusClosed {
		return nil, store.ErrInvalidTransition
	}

	t.Status = newStatus
	t.UpdatedAt = time.Now()
	return t, nil
}

func setupTestRouter(s store.Store, secret string) http.Handler {
	r := chi.NewRouter()

	healthHandler := NewHealthHandler()
	authHandler := NewAuthHandler(s, secret)
	ticketHandler := NewTicketHandler(s)

	r.Get("/health", healthHandler.Health)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	r.Group(func(pr chi.Router) {
		pr.Use(middleware.Auth(secret))
		pr.Post("/tickets", ticketHandler.CreateTicket)
		pr.Get("/tickets", ticketHandler.ListTickets)
		pr.Get("/tickets/{id}", ticketHandler.GetTicket)
		pr.Patch("/tickets/{id}/status", ticketHandler.UpdateStatus)
	})

	return r
}

func TestHealthEndpoint(t *testing.T) {
	r := setupTestRouter(newMemoryStore(), "test-secret")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status: ok, got: %s", resp["status"])
	}
}

func TestAuthFlow(t *testing.T) {
	s := newMemoryStore()
	secret := "test-secret"
	r := setupTestRouter(s, secret)

	t.Run("successful register and login", func(t *testing.T) {
		// Register
		body, _ := json.Marshal(models.RegisterRequest{
			Email:    "test@example.com",
			Password: "mypassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", rec.Code, rec.Body.String())
		}
		var regResp models.RegisterResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &regResp)
		if regResp.ID == "" || regResp.Email != "test@example.com" {
			t.Fatalf("invalid reg response: %+v", regResp)
		}

		// Login
		loginBody, _ := json.Marshal(models.LoginRequest{
			Email:    "test@example.com",
			Password: "mypassword123",
		})
		loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
		loginRec := httptest.NewRecorder()
		r.ServeHTTP(loginRec, loginReq)

		if loginRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", loginRec.Code, loginRec.Body.String())
		}
		var loginResp models.LoginResponse
		_ = json.Unmarshal(loginRec.Body.Bytes(), &loginResp)
		if loginResp.Token == "" {
			t.Fatalf("expected non-empty token")
		}

		// Validate token contains correct subject
		claims, err := auth.ValidateToken(loginResp.Token, secret)
		if err != nil || claims.UserID() != regResp.ID {
			t.Fatalf("token invalid or user id mismatch")
		}
	})

	t.Run("duplicate register returns 409", func(t *testing.T) {
		body, _ := json.Marshal(models.RegisterRequest{
			Email:    "test@example.com",
			Password: "different-password",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 conflict, got %d", rec.Code)
		}
	})

	t.Run("register with invalid email returns 400", func(t *testing.T) {
		body, _ := json.Marshal(models.RegisterRequest{
			Email:    "invalid-email",
			Password: "password",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 bad request, got %d", rec.Code)
		}
	})

	t.Run("register with empty password returns 400", func(t *testing.T) {
		body, _ := json.Marshal(models.RegisterRequest{
			Email:    "valid@example.com",
			Password: "",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 bad request, got %d", rec.Code)
		}
	})

	t.Run("login with wrong password returns 401", func(t *testing.T) {
		body, _ := json.Marshal(models.LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 unauthorized, got %d", rec.Code)
		}
	})

	t.Run("login with non-existent email returns 401", func(t *testing.T) {
		body, _ := json.Marshal(models.LoginRequest{
			Email:    "nobody@example.com",
			Password: "password",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 unauthorized, got %d", rec.Code)
		}
	})
}

func TestTicketsFlow(t *testing.T) {
	s := newMemoryStore()
	secret := "test-secret"
	r := setupTestRouter(s, secret)

	// Create 2 users and tokens
	hash1, _ := auth.HashPassword("pass1")
	u1, _ := s.CreateUser(context.Background(), "user1@example.com", hash1)
	token1, _ := auth.GenerateToken(u1.ID, secret)

	hash2, _ := auth.HashPassword("pass2")
	u2, _ := s.CreateUser(context.Background(), "user2@example.com", hash2)
	token2, _ := auth.GenerateToken(u2.ID, secret)

	t.Run("empty ticket list returns empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var listResp models.TicketListResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
		if listResp.Tickets == nil || len(listResp.Tickets) != 0 {
			t.Fatalf("expected non-nil empty slice, got %+v", listResp.Tickets)
		}
		// Also verify raw json has "[]" not "null"
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"tickets":[]`)) {
			t.Fatalf("expected tickets to be empty array, got %s", rec.Body.String())
		}
	})

	t.Run("create ticket validation", func(t *testing.T) {
		// Empty title returns 400
		body, _ := json.Marshal(models.CreateTicketRequest{
			Title: "   ",
		})
		req := httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	var ticketID string
	t.Run("create ticket success", func(t *testing.T) {
		desc := "some description"
		body, _ := json.Marshal(models.CreateTicketRequest{
			Title:       "Bug in payment",
			Description: &desc,
		})
		req := httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", rec.Code, rec.Body.String())
		}
		var ticket models.Ticket
		_ = json.Unmarshal(rec.Body.Bytes(), &ticket)
		if ticket.ID == "" || ticket.UserID != u1.ID || ticket.Title != "Bug in payment" || ticket.Status != models.StatusOpen {
			t.Fatalf("invalid ticket created: %+v", ticket)
		}
		ticketID = ticket.ID
	})

	t.Run("get ticket by id success for owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tickets/"+ticketID, nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var ticket models.Ticket
		_ = json.Unmarshal(rec.Body.Bytes(), &ticket)
		if ticket.ID != ticketID {
			t.Fatalf("expected ticket id %s, got %s", ticketID, ticket.ID)
		}
	})

	t.Run("get ticket by id returns 404 for non-owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tickets/"+ticketID, nil)
		req.Header.Set("Authorization", "Bearer "+token2)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 not found for non-owner, got %d", rec.Code)
		}
	})

	t.Run("status update validation - unrecognized status returns 400", func(t *testing.T) {
		body, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: "pending"})
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticketID+"/status", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid status, got %d", rec.Code)
		}
	})

	t.Run("status update non-owner returns 404", func(t *testing.T) {
		body, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: "in_progress"})
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticketID+"/status", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token2)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for non-owner status update, got %d", rec.Code)
		}
	})

	t.Run("status update - direct skip open to closed returns 200 (Fix 2)", func(t *testing.T) {
		// Create a separate ticket for this test
		tkt, _ := s.CreateTicket(context.Background(), u1.ID, "Direct close ticket", nil)

		body, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: models.StatusClosed})
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+tkt.ID+"/status", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for open -> closed directly, got %d, body: %s", rec.Code, rec.Body.String())
		}
		var updated models.Ticket
		_ = json.Unmarshal(rec.Body.Bytes(), &updated)
		if updated.Status != models.StatusClosed {
			t.Fatalf("expected status closed, got %s", updated.Status)
		}

		// Updating already closed ticket must return 409
		bodyReopen, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: models.StatusOpen})
		reqReopen := httptest.NewRequest(http.MethodPatch, "/tickets/"+tkt.ID+"/status", bytes.NewReader(bodyReopen))
		reqReopen.Header.Set("Authorization", "Bearer "+token1)
		recReopen := httptest.NewRecorder()
		r.ServeHTTP(recReopen, reqReopen)

		if recReopen.Code != http.StatusConflict {
			t.Fatalf("expected 409 when updating closed ticket, got %d", recReopen.Code)
		}

		// Attempting closed -> in_progress must also return 409
		bodyProgress, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: models.StatusInProgress})
		reqProgress := httptest.NewRequest(http.MethodPatch, "/tickets/"+tkt.ID+"/status", bytes.NewReader(bodyProgress))
		reqProgress.Header.Set("Authorization", "Bearer "+token1)
		recProgress := httptest.NewRecorder()
		r.ServeHTTP(recProgress, reqProgress)

		if recProgress.Code != http.StatusConflict {
			t.Fatalf("expected 409 when updating closed ticket to in_progress, got %d", recProgress.Code)
		}
	})

	t.Run("status update sequential: open -> in_progress -> closed", func(t *testing.T) {
		// open -> in_progress
		body1, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: models.StatusInProgress})
		req1 := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticketID+"/status", bytes.NewReader(body1))
		req1.Header.Set("Authorization", "Bearer "+token1)
		rec1 := httptest.NewRecorder()
		r.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rec1.Code, rec1.Body.String())
		}

		// in_progress -> closed
		body2, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: models.StatusClosed})
		req2 := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticketID+"/status", bytes.NewReader(body2))
		req2.Header.Set("Authorization", "Bearer "+token1)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rec2.Code, rec2.Body.String())
		}

		// closed -> in_progress returns 409
		body3, _ := json.Marshal(models.UpdateTicketStatusRequest{Status: models.StatusInProgress})
		req3 := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticketID+"/status", bytes.NewReader(body3))
		req3.Header.Set("Authorization", "Bearer "+token1)
		rec3 := httptest.NewRecorder()
		r.ServeHTTP(rec3, req3)

		if rec3.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec3.Code)
		}
	})
}
