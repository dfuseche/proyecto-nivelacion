package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"okf-converter/internal/domain"
)

func TestAuthService_GenerateAndValidateToken(t *testing.T) {
	jwtSecret := "test-secret-key-123"
	authService := NewAuthService(nil, jwtSecret)

	testUser := &domain.User{
		ID:    uuid.New(),
		Email: "test@universidad.edu.co",
	}

	// 1. Generar Token JWT
	token, err := authService.generateToken(testUser)
	if err != nil {
		t.Fatalf("error generando token: %v", err)
	}

	if token == "" {
		t.Fatal("el token generado está vacío")
	}

	// 2. Validar Token JWT
	claims, err := authService.ValidateToken(token)
	if err != nil {
		t.Fatalf("error validando token legítimo: %v", err)
	}

	if claims.UserID != testUser.ID {
		t.Errorf("UserID esperado %s, obtenido %s", testUser.ID, claims.UserID)
	}

	if claims.Email != testUser.Email {
		t.Errorf("Email esperado %s, obtenido %s", testUser.Email, claims.Email)
	}
}

func TestAuthService_InvalidToken(t *testing.T) {
	jwtSecret := "test-secret-key-123"
	authService := NewAuthService(nil, jwtSecret)

	// Token malformado
	_, err := authService.ValidateToken("invalid.token.string")
	if err == nil {
		t.Error("se esperaba error al validar un token malformado")
	}

	// Token firmado con otra clave secreta
	wrongSecretService := NewAuthService(nil, "other-secret")
	fakeUser := &domain.User{ID: uuid.New(), Email: "hacker@test.com"}
	fakeToken, _ := wrongSecretService.generateToken(fakeUser)

	_, err = authService.ValidateToken(fakeToken)
	if err == nil {
		t.Error("se esperaba error al validar un token firmado con clave secreta distinta")
	}
}
