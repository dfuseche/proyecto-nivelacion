package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"okf-converter/internal/domain"
	"okf-converter/internal/repository"
)

type JobService struct {
	db      *repository.DB
	storage *repository.Storage
	queue   *repository.Queue
}

func NewJobService(db *repository.DB, storage *repository.Storage, queue *repository.Queue) *JobService {
	return &JobService{
		db:      db,
		storage: storage,
		queue:   queue,
	}
}

func isAllowedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".txt", ".html", ".markdown", ".docx", ".pdf":
		return true
	default:
		return false
	}
}

func (s *JobService) CreateJob(ctx context.Context, userID uuid.UUID, filename string, reader io.Reader, size int64) (*domain.Job, error) {
	if !isAllowedExtension(filename) {
		return nil, fmt.Errorf("formato '%s' no permitido. Formatos soportados: .md, .txt, .html, .markdown, .docx, .pdf", filepath.Ext(filename))
	}

	jobID := uuid.New()
	ext := filepath.Ext(filename)
	fileKey := fmt.Sprintf("originals/%s/%s%s", userID.String(), jobID.String(), ext)

	// 1. Guardar archivo original en MinIO fuera del disco local efímero
	if err := s.storage.UploadFile(ctx, fileKey, reader, size, "application/octet-stream"); err != nil {
		return nil, fmt.Errorf("error guardando archivo original en storage: %w", err)
	}

	// 2. Registrar el trabajo en PostgreSQL con estado PENDING
	job := &domain.Job{
		ID:               jobID,
		UserID:           userID,
		OriginalFilename: filename,
		FileKey:          fileKey,
		Status:           domain.StatusPending,
	}

	if err := s.db.CreateJob(job); err != nil {
		return nil, fmt.Errorf("error registrando trabajo en base de datos: %w", err)
	}

	// Registrar log inicial
	_ = s.db.AddBundleLog(&domain.BundleLog{
		JobID:   jobID,
		Step:    "RECEPCION",
		Status:  "INFO",
		Message: fmt.Sprintf("Documento '%s' recibido correctamente. Trabajo registrado en estado PENDING.", filename),
	})

	// 3. Encolar el mensaje en RabbitMQ
	payload := domain.JobPayload{
		JobID:            jobID,
		UserID:           userID,
		OriginalFilename: filename,
		FileKey:          fileKey,
		Timestamp:        time.Now(),
	}

	if err := s.queue.PublishJob(payload); err != nil {
		_ = s.db.UpdateJobStatus(jobID, domain.StatusFailed, "", "Error al publicar trabajo en la cola de mensajes", 0)
		return nil, fmt.Errorf("error publicando en la cola: %w", err)
	}

	return job, nil
}

func (s *JobService) GetJobByID(jobID, userID uuid.UUID) (*domain.Job, []domain.BundleLog, error) {
	job, err := s.db.GetJobByID(jobID)
	if err != nil {
		return nil, nil, errors.New("trabajo no encontrado")
	}

	// Regla de Aislamiento Multiusuario: Verificar propiedad
	if job.UserID != userID {
		return nil, nil, errors.New("acceso denegado: no es propietario de este recurso")
	}

	logs, err := s.db.GetBundleLogsByJobID(jobID)
	if err != nil {
		logs = []domain.BundleLog{}
	}

	return job, logs, nil
}

func (s *JobService) GetJobsByUserID(userID uuid.UUID) ([]domain.Job, error) {
	return s.db.GetJobsByUserID(userID)
}

func (s *JobService) DeleteJob(ctx context.Context, jobID, userID uuid.UUID) error {
	job, err := s.db.GetJobByID(jobID)
	if err != nil {
		return errors.New("trabajo no encontrado")
	}

	// Regla de Aislamiento Multiusuario: Verificar propiedad
	if job.UserID != userID {
		return errors.New("acceso denegado: no es propietario de este recurso")
	}

	if job.FileKey != "" {
		_ = s.storage.DeleteFile(ctx, job.FileKey)
	}
	if job.BundleKey != "" {
		_ = s.storage.DeleteFile(ctx, job.BundleKey)
	}

	return s.db.DeleteJob(jobID, userID)
}

func (s *JobService) GetDownloadStream(ctx context.Context, jobID, userID uuid.UUID) (io.ReadCloser, string, error) {
	job, err := s.db.GetJobByID(jobID)
	if err != nil {
		return nil, "", errors.New("trabajo no encontrado")
	}

	if job.UserID != userID {
		return nil, "", errors.New("acceso denegado: no es propietario de este recurso")
	}

	if job.Status != domain.StatusCompleted {
		return nil, "", fmt.Errorf("el bundle no está disponible para descarga (estado: %s)", job.Status)
	}

	if job.BundleKey == "" {
		return nil, "", errors.New("referencia de bundle no encontrada")
	}

	stream, err := s.storage.DownloadFile(ctx, job.BundleKey)
	if err != nil {
		return nil, "", fmt.Errorf("error obteniendo bundle desde el almacenamiento: %w", err)
	}

	downloadName := fmt.Sprintf("bundle_%s.zip", job.ID.String()[:8])
	return stream, downloadName, nil
}
