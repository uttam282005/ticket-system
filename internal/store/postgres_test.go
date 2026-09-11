package store

import (
	"context"
	"os"
	"testing"
	"time"

	"ticket-system/internal/db"
	"ticket-system/internal/models"

	"github.com/google/uuid"
)

func TestPostgresStoreIntegration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping PostgreSQL integration test: TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	store := NewPostgresStore(pool)

	uniqueSuffix := uuid.NewString()[:8]
	email1 := "user1_" + uniqueSuffix + "@example.com"
	email2 := "user2_" + uniqueSuffix + "@example.com"

	// 1. Create User 1
	u1, err := store.CreateUser(ctx, email1, "hash1")
	if err != nil {
		t.Fatalf("failed to create user 1: %v", err)
	}
	if u1.ID == "" || u1.Email != email1 {
		t.Fatalf("unexpected user 1 data: %+v", u1)
	}

	// 2. Duplicate email returns ErrDuplicateEmail
	_, err = store.CreateUser(ctx, email1, "hash_other")
	if err != ErrDuplicateEmail {
		t.Fatalf("expected ErrDuplicateEmail, got: %v", err)
	}

	// 3. Create User 2
	u2, err := store.CreateUser(ctx, email2, "hash2")
	if err != nil {
		t.Fatalf("failed to create user 2: %v", err)
	}

	// 4. Create Ticket for User 1
	desc := "My ticket description"
	t1, err := store.CreateTicket(ctx, u1.ID, "Server down", &desc)
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}
	if t1.Status != models.StatusOpen || t1.UserID != u1.ID {
		t.Fatalf("unexpected ticket: %+v", t1)
	}

	// 5. User 1 can fetch ticket
	fetched, err := store.GetTicketByIDAndUserID(ctx, t1.ID, u1.ID)
	if err != nil {
		t.Fatalf("user 1 failed to fetch ticket: %v", err)
	}
	if fetched.ID != t1.ID {
		t.Fatalf("fetched ticket id mismatch: %s != %s", fetched.ID, t1.ID)
	}

	// 6. User 2 CANNOT fetch ticket (ownership enforcement in SQL returns ErrNotFound)
	_, err = store.GetTicketByIDAndUserID(ctx, t1.ID, u2.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-user fetch, got: %v", err)
	}

	// 7. Status transition: open -> closed directly allowed (Fix 2)
	t2, err := store.CreateTicket(ctx, u1.ID, "Direct close ticket", nil)
	if err != nil {
		t.Fatalf("failed to create ticket 2: %v", err)
	}
	closedTicket, err := store.UpdateTicketStatus(ctx, t2.ID, u1.ID, models.StatusClosed)
	if err != nil {
		t.Fatalf("expected open -> closed to succeed per Fix 2, got: %v", err)
	}
	if closedTicket.Status != models.StatusClosed {
		t.Fatalf("expected status closed, got: %s", closedTicket.Status)
	}

	// 8. Modifying closed ticket returns ErrInvalidTransition
	_, err = store.UpdateTicketStatus(ctx, t2.ID, u1.ID, models.StatusOpen)
	if err != ErrInvalidTransition {
		t.Fatalf("expected ErrInvalidTransition on closed ticket, got: %v", err)
	}

	// 9. Non-owner cannot update status
	_, err = store.UpdateTicketStatus(ctx, t1.ID, u2.ID, models.StatusInProgress)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on non-owner status update, got: %v", err)
	}

	// 10. List tickets returns only user's tickets
	list1, err := store.GetTicketsByUserID(ctx, u1.ID)
	if err != nil {
		t.Fatalf("failed to list user 1 tickets: %v", err)
	}
	if len(list1) < 2 {
		t.Fatalf("expected at least 2 tickets for user 1, got %d", len(list1))
	}
	for _, item := range list1 {
		if item.UserID != u1.ID {
			t.Fatalf("user 1 ticket list contained other user's ticket: %+v", item)
		}
	}
}
