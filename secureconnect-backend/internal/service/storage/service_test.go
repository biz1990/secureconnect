package storage

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"secureconnect-backend/internal/domain"
)

// Mocks
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) Create(ctx context.Context, file *domain.File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockFileRepository) GetByID(ctx context.Context, fileID uuid.UUID) (*domain.File, error) {
	args := m.Called(ctx, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.File), args.Error(1)
}

func (m *MockFileRepository) UpdateStatus(ctx context.Context, fileID uuid.UUID, status string) error {
	args := m.Called(ctx, fileID, status)
	return args.Error(0)
}

func (m *MockFileRepository) GetUserStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockFileRepository) CheckFileAccess(ctx context.Context, fileID uuid.UUID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, fileID, userID)
	return args.Bool(0), args.Error(1)
}

type MockObjectStorage struct {
	mock.Mock
}

func (m *MockObjectStorage) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	args := m.Called(ctx, bucketName)
	return args.Bool(0), args.Error(1)
}

func (m *MockObjectStorage) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	args := m.Called(ctx, bucketName, opts)
	return args.Error(0)
}

func (m *MockObjectStorage) PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expires)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*url.URL), args.Error(1)
}

func (m *MockObjectStorage) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expires, reqParams)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*url.URL), args.Error(1)
}

func (m *MockObjectStorage) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}

func TestNewService_BucketExists(t *testing.T) {
	mockStorage := new(MockObjectStorage)
	mockRepo := new(MockFileRepository)

	ctx := context.Background()
	_ = ctx // Used in service

	// Expectations - NewService calls BucketExists
	// Note: NewService creates its own background context, so we use mock.Anything for context
	mockStorage.On("BucketExists", mock.Anything, "test-bucket").Return(true, nil)

	service, err := NewService(mockStorage, "test-bucket", mockRepo)

	assert.NoError(t, err)
	assert.NotNil(t, service)

	mockStorage.AssertExpectations(t)
}

func TestGenerateUploadURL(t *testing.T) {
	mockStorage := new(MockObjectStorage)
	mockRepo := new(MockFileRepository)

	// Setup service (simulate success bucket check)
	mockStorage.On("BucketExists", mock.Anything, "test-bucket").Return(true, nil)
	service, _ := NewService(mockStorage, "test-bucket", mockRepo)

	// Reset mocks to clear NewService calls
	mockStorage.Calls = nil
	mockStorage.ExpectedCalls = nil

	userID := uuid.New()
	input := &GenerateUploadURLInput{
		FileName:    "test.jpg",
		FileSize:    1024,
		ContentType: "image/jpeg",
	}

	ctx := context.Background()

	dummyURL, _ := url.Parse("http://minio/test.jpg")

	// Expectations
	mockStorage.On("PresignedPutObject", ctx, "test-bucket", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(dummyURL, nil)
	mockRepo.On("GetUserStorageUsage", ctx, userID).Return(int64(0), nil)
	mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.File")).Return(nil)

	// Execute
	output, err := service.GenerateUploadURL(ctx, userID, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, dummyURL.String(), output.UploadURL)

	mockStorage.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}
