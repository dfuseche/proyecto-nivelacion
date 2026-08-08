package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusPending    JobStatus = "PENDING"
	StatusProcessing JobStatus = "PROCESSING"
	StatusCompleted  JobStatus = "COMPLETED"
	StatusFailed     JobStatus = "FAILED"
	StatusInvalid    JobStatus = "INVALID"
)

// User representa a un usuario registrado en el sistema
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Job representa una solicitud de conversión documental
type Job struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	OriginalFilename string    `json:"original_filename"`
	FileKey          string    `json:"file_key"`
	BundleKey        string    `json:"bundle_key,omitempty"`
	Status           JobStatus `json:"status"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	UnitsCount       int       `json:"units_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// BundleLog representa un registro de auditoría/trazabilidad del bundle OKF
type BundleLog struct {
	ID        uuid.UUID `json:"id"`
	JobID     uuid.UUID `json:"job_id"`
	Step      string    `json:"step"`
	Status    string    `json:"status"` // INFO, SUCCESS, WARNING, ERROR
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// JobPayload es la estructura enviada a través de RabbitMQ
type JobPayload struct {
	JobID            uuid.UUID `json:"job_id"`
	UserID           uuid.UUID `json:"user_id"`
	OriginalFilename string    `json:"original_filename"`
	FileKey          string    `json:"file_key"`
	Timestamp        time.Time `json:"timestamp"`
}

// JWT Claims
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// DTOs de Autenticación
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
