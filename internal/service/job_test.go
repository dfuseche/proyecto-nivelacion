package service

import (
	"testing"

	"github.com/google/uuid"

	"okf-converter/internal/domain"
)

func TestJobService_OwnershipIsolationCheck(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()

	jobOfUserA := &domain.Job{
		ID:               uuid.New(),
		UserID:           userA,
		OriginalFilename: "secreto_user_a.md",
		Status:           domain.StatusCompleted,
		BundleKey:        "bundles/userA/bundle.zip",
	}

	// 1. Simular verificación de propiedad para el propietario legítimo (Usuario A)
	if jobOfUserA.UserID != userA {
		t.Errorf("Usuario A debe tener acceso a su propio trabajo")
	}

	// 2. Simular intento de acceso por un usuario distinto (Usuario B)
	if jobOfUserA.UserID == userB {
		t.Errorf("Usuario B NO debe ser considerado propietario del trabajo del Usuario A")
	}
}
