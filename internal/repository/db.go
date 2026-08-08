package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"okf-converter/internal/domain"
)

type DB struct {
	conn *sql.DB
}

func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir la conexión con postgres: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("error al verificar ping con postgres: %w", err)
	}

	log.Println("[DB] Conexión a PostgreSQL establecida exitosamente")
	return &DB{conn: conn}, nil
}

func (db *DB) CreateUser(user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING created_at`

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	err := db.conn.QueryRow(query, user.ID, user.Email, user.PasswordHash).Scan(&user.CreatedAt)
	if err != nil {
		return fmt.Errorf("error creando usuario: %w", err)
	}
	return nil
}

func (db *DB) GetUserByEmail(email string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, created_at FROM users WHERE email = $1`
	user := &domain.User{}

	err := db.conn.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("usuario no encontrado")
		}
		return nil, fmt.Errorf("error obteniendo usuario por email: %w", err)
	}

	return user, nil
}

func (db *DB) GetUserByID(id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, email, password_hash, created_at FROM users WHERE id = $1`
	user := &domain.User{}

	err := db.conn.QueryRow(query, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("usuario no encontrado")
		}
		return nil, fmt.Errorf("error obteniendo usuario por ID: %w", err)
	}

	return user, nil
}

func (db *DB) CreateJob(job *domain.Job) error {
	query := `
		INSERT INTO jobs (id, user_id, original_filename, file_key, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`

	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.Status == "" {
		job.Status = domain.StatusPending
	}

	err := db.conn.QueryRow(query, job.ID, job.UserID, job.OriginalFilename, job.FileKey, job.Status).
		Scan(&job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return fmt.Errorf("error creando trabajo: %w", err)
	}

	return nil
}

func (db *DB) UpdateJobStatus(jobID uuid.UUID, status domain.JobStatus, bundleKey string, errorMsg string, unitsCount int) error {
	query := `
		UPDATE jobs
		SET status = $1, bundle_key = $2, error_message = $3, units_count = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5`

	_, err := db.conn.Exec(query, status, bundleKey, errorMsg, unitsCount, jobID)
	if err != nil {
		return fmt.Errorf("error actualizando estado del trabajo: %w", err)
	}
	return nil
}

func (db *DB) GetJobByID(jobID uuid.UUID) (*domain.Job, error) {
	query := `
		SELECT id, user_id, original_filename, file_key, COALESCE(bundle_key, ''), status, COALESCE(error_message, ''), units_count, created_at, updated_at
		FROM jobs
		WHERE id = $1`

	job := &domain.Job{}
	err := db.conn.QueryRow(query, jobID).Scan(
		&job.ID, &job.UserID, &job.OriginalFilename, &job.FileKey, &job.BundleKey,
		&job.Status, &job.ErrorMessage, &job.UnitsCount, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("trabajo no encontrado")
		}
		return nil, fmt.Errorf("error obteniendo trabajo por ID: %w", err)
	}

	return job, nil
}

func (db *DB) GetJobsByUserID(userID uuid.UUID) ([]domain.Job, error) {
	query := `
		SELECT id, user_id, original_filename, file_key, COALESCE(bundle_key, ''), status, COALESCE(error_message, ''), units_count, created_at, updated_at
		FROM jobs
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := db.conn.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("error consultando trabajos del usuario: %w", err)
	}
	defer rows.Close()

	jobs := []domain.Job{}
	for rows.Next() {
		var j domain.Job
		if err := rows.Scan(
			&j.ID, &j.UserID, &j.OriginalFilename, &j.FileKey, &j.BundleKey,
			&j.Status, &j.ErrorMessage, &j.UnitsCount, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando fila de trabajo: %w", err)
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}

func (db *DB) AddBundleLog(bLog *domain.BundleLog) error {
	query := `
		INSERT INTO bundle_logs (id, job_id, step, status, message)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`

	if bLog.ID == uuid.Nil {
		bLog.ID = uuid.New()
	}

	err := db.conn.QueryRow(query, bLog.ID, bLog.JobID, bLog.Step, bLog.Status, bLog.Message).Scan(&bLog.CreatedAt)
	if err != nil {
		return fmt.Errorf("error agregando log de bundle: %w", err)
	}
	return nil
}

func (db *DB) GetBundleLogsByJobID(jobID uuid.UUID) ([]domain.BundleLog, error) {
	query := `
		SELECT id, job_id, step, status, message, created_at
		FROM bundle_logs
		WHERE job_id = $1
		ORDER BY created_at ASC`

	rows, err := db.conn.Query(query, jobID)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs de bundle: %w", err)
	}
	defer rows.Close()

	logsList := []domain.BundleLog{}
	for rows.Next() {
		var l domain.BundleLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.Step, &l.Status, &l.Message, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logsList = append(logsList, l)
	}

	return logsList, nil
}
