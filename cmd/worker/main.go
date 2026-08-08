package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"okf-converter/internal/config"
	"okf-converter/internal/converter"
	"okf-converter/internal/domain"
	"okf-converter/internal/repository"
	"okf-converter/internal/validator"
)

func main() {
	cfg := config.LoadConfig()

	log.Println("[WORKER] Iniciando servicio de procesamiento asíncrono Go...")

	// 1. Conexión a PostgreSQL
	db, err := repository.NewDB(cfg.GetDSN())
	if err != nil {
		log.Fatalf("[WORKER FATAL] Error al conectar con PostgreSQL: %v", err)
	}

	// 2. Conexión a MinIO Storage
	storage, err := repository.NewStorage(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		log.Fatalf("[WORKER FATAL] Error al conectar con MinIO: %v", err)
	}

	// 3. Conexión a RabbitMQ
	queue, err := repository.NewQueue(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		log.Fatalf("[WORKER FATAL] Error al conectar con RabbitMQ: %v", err)
	}
	defer queue.Close()

	// 4. Instanciar Conversor y Validador OKF
	okfConverter := converter.NewOKFConverter()
	okfValidator := validator.NewOKFValidator()

	// 5. Escuchar canal de la cola RabbitMQ
	msgs, err := queue.ConsumeJobs()
	if err != nil {
		log.Fatalf("[WORKER FATAL] Error iniciando consumidor de mensajes: %v", err)
	}

	log.Println("[WORKER] Worker Go esperando mensajes de tareas de conversión...")

	for d := range msgs {
		var payload domain.JobPayload
		if err := json.Unmarshal(d.Body, &payload); err != nil {
			log.Printf("[WORKER ERROR] Mensaje con formato JSON inválido: %v", err)
			d.Ack(false)
			continue
		}

		log.Printf("[WORKER] Procesando tarea de conversión | JobID: %s | Usuario: %s", payload.JobID, payload.UserID)

		// CONTROL DE IDEMPOTENCIA: Verificar si el trabajo ya fue completado previamente
		existingJob, err := db.GetJobByID(payload.JobID)
		if err == nil && existingJob.Status == domain.StatusCompleted {
			log.Printf("[WORKER IDEMPOTENCIA] El trabajo %s ya se encuentra completado. Omitiendo duplicado.", payload.JobID)
			d.Ack(false)
			continue
		}

		// Actualizar estado a PROCESSING
		_ = db.UpdateJobStatus(payload.JobID, domain.StatusProcessing, "", "", 0)
		_ = db.AddBundleLog(&domain.BundleLog{
			JobID:   payload.JobID,
			Step:    "PROCESAMIENTO",
			Status:  "INFO",
			Message: "Inicio de descarga y parsing del documento en segundo plano.",
		})

		// Descargar archivo original desde MinIO
		ctx := context.Background()
		fileStream, err := storage.DownloadFile(ctx, payload.FileKey)
		if err != nil {
			msg := fmt.Sprintf("Error descargando archivo original de MinIO: %v", err)
			log.Printf("[WORKER ERROR] %s", msg)
			_ = db.UpdateJobStatus(payload.JobID, domain.StatusFailed, "", msg, 0)
			d.Ack(false)
			continue
		}

		rawBytes, err := io.ReadAll(fileStream)
		fileStream.Close()
		if err != nil {
			msg := fmt.Sprintf("Error leyendo contenido del archivo: %v", err)
			log.Printf("[WORKER ERROR] %s", msg)
			_ = db.UpdateJobStatus(payload.JobID, domain.StatusFailed, "", msg, 0)
			d.Ack(false)
			continue
		}

		// Generar resumen de logs hasta el momento
		logsList, _ := db.GetBundleLogsByJobID(payload.JobID)
		logEntries := []string{}
		for _, l := range logsList {
			logEntries = append(logEntries, fmt.Sprintf("| %s | %s | %s | %s |",
				l.CreatedAt.Format("15:04:05"), l.Step, l.Status, l.Message))
		}

		// Ejecutar Conversión a Bundle OKF
		zipBytes, unitsCount, err := okfConverter.ConvertToOKFBundle(payload.JobID, payload.OriginalFilename, rawBytes, logEntries)
		if err != nil {
			msg := fmt.Sprintf("Fallo en la conversión a bundle OKF: %v", err)
			log.Printf("[WORKER ERROR] %s", msg)
			_ = db.UpdateJobStatus(payload.JobID, domain.StatusFailed, "", msg, 0)
			d.Ack(false)
			continue
		}

		// Validar Bundle OKF antes de publicar
		validation := okfValidator.ValidateZipBundle(zipBytes)
		if !validation.IsValid {
			errMsg := fmt.Sprintf("Validación de Bundle fallida: %v", validation.Errors)
			log.Printf("[WORKER VALIDACIÓN FALLIDA] Job %s es inválido y no será publicado: %s", payload.JobID, errMsg)
			_ = db.UpdateJobStatus(payload.JobID, domain.StatusInvalid, "", errMsg, unitsCount)
			_ = db.AddBundleLog(&domain.BundleLog{
				JobID:   payload.JobID,
				Step:    "VALIDACION",
				Status:  "ERROR",
				Message: errMsg,
			})
			d.Ack(false)
			continue
		}

		// Guardar paquete ZIP del Bundle OKF en MinIO
		bundleKey := fmt.Sprintf("bundles/%s/%s.zip", payload.UserID.String(), payload.JobID.String())
		zipReader := bytes.NewReader(zipBytes)

		if err := storage.UploadFile(ctx, bundleKey, zipReader, int64(len(zipBytes)), "application/zip"); err != nil {
			msg := fmt.Sprintf("Error guardando paquete de bundle en MinIO: %v", err)
			log.Printf("[WORKER ERROR] %s", msg)
			_ = db.UpdateJobStatus(payload.JobID, domain.StatusFailed, "", msg, 0)
			d.Ack(false)
			continue
		}

		// Actualizar estado final a COMPLETED
		_ = db.UpdateJobStatus(payload.JobID, domain.StatusCompleted, bundleKey, "", unitsCount)
		_ = db.AddBundleLog(&domain.BundleLog{
			JobID:   payload.JobID,
			Step:    "PUBLICACION",
			Status:  "SUCCESS",
			Message: fmt.Sprintf("Bundle OKF validado exitosamente con %d unidades. Publicado en %s", unitsCount, bundleKey),
		})

		log.Printf("[WORKER ÉXITO] Trabajo %s completado y bundle publicado exitosamente", payload.JobID)
		d.Ack(false)
	}
}
