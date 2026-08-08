package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"okf-converter/internal/domain"
)

func TestJobPayload_Serialization(t *testing.T) {
	originalPayload := domain.JobPayload{
		JobID:            uuid.New(),
		UserID:           uuid.New(),
		OriginalFilename: "documento_cloud.md",
		FileKey:          "originals/user-123/job-456.md",
		Timestamp:        time.Now(),
	}

	// 1. Serializar a JSON
	jsonBytes, err := json.Marshal(originalPayload)
	if err != nil {
		t.Fatalf("error serializando payload de RabbitMQ: %v", err)
	}

	// 2. Deserializar desde JSON
	var deserializedPayload domain.JobPayload
	if err := json.Unmarshal(jsonBytes, &deserializedPayload); err != nil {
		t.Fatalf("error deserializando payload de RabbitMQ: %v", err)
	}

	if deserializedPayload.JobID != originalPayload.JobID {
		t.Errorf("JobID esperado %s, obtenido %s", originalPayload.JobID, deserializedPayload.JobID)
	}

	if deserializedPayload.UserID != originalPayload.UserID {
		t.Errorf("UserID esperado %s, obtenido %s", originalPayload.UserID, deserializedPayload.UserID)
	}

	if deserializedPayload.OriginalFilename != originalPayload.OriginalFilename {
		t.Errorf("Filename esperado %s, obtenido %s", originalPayload.OriginalFilename, deserializedPayload.OriginalFilename)
	}
}
