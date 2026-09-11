package store

import (
	"context"
	"errors"

	"ticket-system/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	query := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at
	`
	var user models.User
	err := s.pool.QueryRow(ctx, query, email, passwordHash).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`
	var user models.User
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`
	var user models.User
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) CreateTicket(ctx context.Context, userID, title string, description *string) (*models.Ticket, error) {
	query := `
		INSERT INTO tickets (user_id, title, description, status)
		VALUES ($1, $2, $3, 'open')
		RETURNING id, user_id, title, description, status, created_at, updated_at
	`
	var ticket models.Ticket
	err := s.pool.QueryRow(ctx, query, userID, title, description).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (s *PostgresStore) GetTicketsByUserID(ctx context.Context, userID string) ([]models.Ticket, error) {
	query := `
		SELECT id, user_id, title, description, status, created_at, updated_at
		FROM tickets
		WHERE user_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]models.Ticket, 0)
	for rows.Next() {
		var ticket models.Ticket
		err := rows.Scan(
			&ticket.ID,
			&ticket.UserID,
			&ticket.Title,
			&ticket.Description,
			&ticket.Status,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (s *PostgresStore) GetTicketByIDAndUserID(ctx context.Context, id, userID string) (*models.Ticket, error) {
	query := `
		SELECT id, user_id, title, description, status, created_at, updated_at
		FROM tickets
		WHERE id = $1 AND user_id = $2
	`
	var ticket models.Ticket
	err := s.pool.QueryRow(ctx, query, id, userID).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

func (s *PostgresStore) UpdateTicketStatus(ctx context.Context, id, userID, newStatus string) (*models.Ticket, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Fetch current status with row-level lock enforcing ownership in SQL
	var currentStatus string
	querySelect := `
		SELECT status
		FROM tickets
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, querySelect, id, userID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Rule: Once a ticket is closed, no further status changes are allowed.
	if currentStatus == models.StatusClosed {
		return nil, ErrInvalidTransition
	}

	queryUpdate := `
		UPDATE tickets
		SET status = $1, updated_at = now()
		WHERE id = $2 AND user_id = $3
		RETURNING id, user_id, title, description, status, created_at, updated_at
	`
	var ticket models.Ticket
	err = tx.QueryRow(ctx, queryUpdate, newStatus, id, userID).Scan(
		&ticket.ID,
		&ticket.UserID,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &ticket, nil
}
