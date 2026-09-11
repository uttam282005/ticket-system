package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ticket-system/internal/db"
	"ticket-system/internal/handlers"
	"ticket-system/internal/middleware"
	"ticket-system/internal/store"
	"ticket-system/web"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Connecting to PostgreSQL...")
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer pool.Close()

	log.Println("Running database migrations...")
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migrations applied successfully")

	pgStore := store.NewPostgresStore(pool)

	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler(pgStore, jwtSecret)
	ticketHandler := handlers.NewTicketHandler(pgStore)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Public routes
	r.Get("/health", healthHandler.Health)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	// Protected routes
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.Auth(jwtSecret))
		pr.Post("/tickets", ticketHandler.CreateTicket)
		pr.Get("/tickets", ticketHandler.ListTickets)
		pr.Get("/tickets/{id}", ticketHandler.GetTicket)
		pr.Patch("/tickets/{id}/status", ticketHandler.UpdateStatus)
	})

	// Web UI handler for root and static assets
	r.Handle("/*", web.Handler())

	// Server address is strictly hardcoded to :8080 per specification
	server := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)
	case <-ctx.Done():
		log.Println("Shutdown signal received, shutting down gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
		log.Println("Server stopped")
	}
}
