package repository

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *minio.Client
	bucket string
}

func NewStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Storage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("error inicializando cliente MinIO: %w", err)
	}

	s := &Storage{
		client: client,
		bucket: bucket,
	}

	if err := s.EnsureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("error asegurando bucket MinIO: %w", err)
	}

	log.Printf("[STORAGE] Cliente MinIO inicializado en bucket '%s'", bucket)
	return s, nil
}

func (s *Storage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("error al verificar existencia de bucket: %w", err)
	}

	if !exists {
		err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("error creando bucket '%s': %w", s.bucket, err)
		}
		log.Printf("[STORAGE] Bucket '%s' creado exitosamente", s.bucket)
	}
	return nil
}

func (s *Storage) UploadFile(ctx context.Context, objectKey string, reader io.Reader, objectSize int64, contentType string) error {
	opts := minio.PutObjectOptions{ContentType: contentType}
	_, err := s.client.PutObject(ctx, s.bucket, objectKey, reader, objectSize, opts)
	if err != nil {
		return fmt.Errorf("error subiendo archivo a MinIO (%s): %w", objectKey, err)
	}
	return nil
}

func (s *Storage) DownloadFile(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error obteniendo objeto de MinIO (%s): %w", objectKey, err)
	}
	return object, nil
}

func (s *Storage) DeleteFile(ctx context.Context, objectKey string) error {
	if objectKey == "" {
		return nil
	}
	err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("error al eliminar objeto de MinIO (%s): %w", objectKey, err)
	}
	return nil
}
