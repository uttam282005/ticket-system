package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPasswordHashing(t *testing.T) {
	password := "supersecret123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if hash == password {
		t.Fatalf("hash should not equal plain text password")
	}

	if !CheckPassword(hash, password) {
		t.Fatalf("expected password verification to succeed")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Fatalf("expected password verification to fail for wrong password")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "my-jwt-secret-test"
	userID := "user-12345"

	tokenStr, err := GenerateToken(userID, secret)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}

	if claims.UserID() != userID {
		t.Fatalf("expected userID %s, got %s", userID, claims.UserID())
	}

	if claims.Subject != userID {
		t.Fatalf("expected subject %s, got %s", userID, claims.Subject)
	}

	// Validate with wrong secret
	_, err = ValidateToken(tokenStr, "wrong-secret")
	if err == nil {
		t.Fatalf("expected error validating token with wrong secret")
	}

	// Validate malformed token
	_, err = ValidateToken("invalid.token.string", secret)
	if err == nil {
		t.Fatalf("expected error validating malformed token")
	}
}

func TestExpiredToken(t *testing.T) {
	secret := "my-jwt-secret-test"
	userID := "user-12345"

	// Create expired token manually
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = ValidateToken(tokenStr, secret)
	if err == nil {
		t.Fatalf("expected error validating expired token")
	}
}
