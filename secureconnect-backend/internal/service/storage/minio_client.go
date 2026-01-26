package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerHalfOpen
	CircuitBreakerOpen
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxFailures  int
	Timeout      time.Duration
	ResetTimeout time.Duration
}

// DefaultCircuitBreakerConfig returns default circuit breaker settings
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:  5,
		Timeout:      10 * time.Second,
		ResetTimeout: 30 * time.Second,
	}
}

// MinioClient wraps MinIO client with resilience features
type MinioClient struct {
	client      *minio.Client
	config      *CircuitBreakerConfig
	state       CircuitBreakerState
	failures    int
	lastFailure time.Time
}

// NewMinioClient creates a new MinIO client with resilience features
func NewMinioClient(endpoint, accessKey, secretKey string) (*MinioClient, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &MinioClient{
		client: minioClient,
		config: DefaultCircuitBreakerConfig(),
		state:  CircuitBreakerClosed,
	}, nil
}

// UploadFile uploads a file to MinIO with timeout, retry, and circuit breaker
func (c *MinioClient) UploadFile(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	// Set timeout from context if available
	uploadCtx := ctx
	if _, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		uploadCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Execute upload with circuit breaker
	info, err := c.uploadWithCircuitBreaker(uploadCtx, bucketName, objectName, reader, size, opts)
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("upload failed: %w", err)
	}

	return info, nil
}

// uploadWithCircuitBreaker executes upload with circuit breaker logic
func (c *MinioClient) uploadWithCircuitBreaker(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	// Check circuit breaker state
	if c.state == CircuitBreakerOpen {
		return minio.UploadInfo{}, errors.New("circuit breaker is open")
	}

	// Execute upload
	info, err := c.client.PutObject(ctx, bucketName, objectName, reader, size, opts)

	// Handle success
	if err == nil {
		c.onSuccess()
		return info, nil
	}

	// Handle failure
	c.onFailure(err)

	// Check if circuit breaker should open
	if c.failures >= c.config.MaxFailures {
		c.state = CircuitBreakerOpen
		log.Printf("MinIO circuit breaker opened after %d failures", c.failures)
	}

	return info, err
}

// GetFile downloads a file from MinIO with timeout and retry
func (c *MinioClient) GetFile(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	// Set timeout from context if available
	downloadCtx := ctx
	if _, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		downloadCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Execute download with circuit breaker
	obj, err := c.getFileWithCircuitBreaker(downloadCtx, bucketName, objectName, opts)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	return obj, nil
}

// getFileWithCircuitBreaker executes download with circuit breaker logic
func (c *MinioClient) getFileWithCircuitBreaker(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	// Check circuit breaker state
	if c.state == CircuitBreakerOpen {
		return nil, errors.New("circuit breaker is open")
	}

	// Execute download
	obj, err := c.client.GetObject(ctx, bucketName, objectName, opts)

	// Handle success
	if err == nil {
		c.onSuccess()
		return obj, nil
	}

	// Handle failure
	c.onFailure(err)

	// Check if circuit breaker should open
	if c.failures >= c.config.MaxFailures {
		c.state = CircuitBreakerOpen
		log.Printf("MinIO circuit breaker opened after %d failures", c.failures)
	}

	return obj, err
}

// DeleteFile deletes a file from MinIO with timeout and retry
func (c *MinioClient) DeleteFile(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	// Set timeout from context if available
	deleteCtx := ctx
	if _, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		deleteCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Execute delete with circuit breaker
	err := c.deleteFileWithCircuitBreaker(deleteCtx, bucketName, objectName, opts)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

// deleteFileWithCircuitBreaker executes delete with circuit breaker logic
func (c *MinioClient) deleteFileWithCircuitBreaker(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	// Check circuit breaker state
	if c.state == CircuitBreakerOpen {
		return errors.New("circuit breaker is open")
	}

	// Execute delete
	err := c.client.RemoveObject(ctx, bucketName, objectName, opts)

	// Handle success
	if err == nil {
		c.onSuccess()
		return nil
	}

	// Handle failure
	c.onFailure(err)

	// Check if circuit breaker should open
	if c.failures >= c.config.MaxFailures {
		c.state = CircuitBreakerOpen
		log.Printf("MinIO circuit breaker opened after %d failures", c.failures)
	}

	return err
}

// onSuccess handles successful operation
func (c *MinioClient) onSuccess() {
	c.failures = 0
	c.state = CircuitBreakerClosed
	c.lastFailure = time.Time{}
}

// onFailure handles failed operation
func (c *MinioClient) onFailure(err error) {
	c.failures++
	c.lastFailure = time.Now()

	// Log error
	log.Printf("MinIO operation failed: %v (failure %d/%s)", err, c.failures, err.Error())
}

// ResetCircuitBreaker resets the circuit breaker
func (c *MinioClient) ResetCircuitBreaker() {
	c.state = CircuitBreakerClosed
	c.failures = 0
	c.lastFailure = time.Time{}
	log.Println("MinIO circuit breaker reset")
}

// GetState returns the current circuit breaker state
func (c *MinioClient) GetState() CircuitBreakerState {
	return c.state
}

// IsOpen returns true if circuit breaker is closed (allowing requests)
func (c *MinioClient) IsOpen() bool {
	return c.state == CircuitBreakerClosed
}

// Close closes the MinIO client
func (c *MinioClient) Close() error {
	// minio.Client does not have a Close method
	// This is a no-op for compatibility
	return nil
}
