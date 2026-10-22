package minio

import (
	"backend/pkg/errs"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	BucketName string
	BaseUrl    string
}

type MinIORepository interface {
	FileUploadSvc(file multipart.File, directoryPath, filename string, fileSize int64) (string, error)
	DeleteFileSvc(bucketName string) error
	UploadSingleFile(file *multipart.FileHeader, filepath string) (string, error)
	UploadMultipleFiles(files []*multipart.FileHeader, filepath string) ([]string, error)
	DeleteFile(objectName string) error
	GetFileURL(objectName string) string
}

type minIORepository struct {
	minioClient *minio.Client
	config      Config
}

// GetFileURL implements MinioRepository.
func (m *minIORepository) GetFileURL(objectName string) string {
	if objectName == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", m.config.BaseUrl, m.config.BucketName, objectName)
}

// UploadMultipleFiles implements MinioRepository.
func (m *minIORepository) UploadMultipleFiles(files []*multipart.FileHeader, filepath string) ([]string, error) {
	var filenames []string
	IMAGES := files
	if IMAGES == nil {
		errorMsg := fmt.Errorf("ERROR_FILE_KEY_IS_REQUIRED: %v", len(IMAGES))
		return nil, errs.NewBusinessError(errorMsg)
	}

	for _, image := range IMAGES {
		uniqueId := uuid.New()
		FileName := strings.Replace(uniqueId.String(), "-", "", -1)
		FileExt := strings.Split(image.Filename, ".")[1]

		if image.Size > 5000000 { // 5MB
			errorMsg := fmt.Errorf("ERROR_IMAGE_TOO_LARGE: %v", image.Size)
			return nil, errs.NewBusinessError(errorMsg)
		}
		newFilename := fmt.Sprintf("%s.%s", FileName, FileExt)
		file, err := image.Open()
		if err != nil {
			errorMsg := fmt.Errorf("ERROR_FILE_OPEN: %v", err)
			return nil, errs.NewBusinessError(errorMsg)
		}
		fileSize := image.Size
		presignedURL, err := m.FileUploadSvc(file, filepath, newFilename, fileSize)
		if err != nil {
			errorMsg := fmt.Errorf("ERROR_IMAGE_UPLOAD: %v", err)

			return nil, errs.NewBusinessError(errorMsg)
		}

		filenames = append(filenames, presignedURL)
	}

	return filenames, nil
}

func (m *minIORepository) DeleteFile(objectName string) error {
	return m.DeleteFileSvc(objectName)
}

func (m *minIORepository) UploadSingleFile(file *multipart.FileHeader, filepath string) (string, error) {
	if file == nil {
		errorMsg := fmt.Errorf("ERROR_FILE_KEY_IS_REQUIRED: %v", file)
		return "", errs.NewBusinessError(errorMsg)
	} else {
		uniqueId1 := uuid.New()
		FileName := strings.Replace(uniqueId1.String(), "-", "", -1)
		File := strings.Split(file.Filename, ".")[1]
		//Validate Image
		// if IMAGE[0].Size > 15728640  { //15MB
		if file.Size > 5000000 { //5MB
			errorMsg := fmt.Errorf("ERROR_IMAGE_TOO_LARGE: %v", file.Size)

			return "", errs.NewBusinessError(errorMsg)
		}
		newFilename := fmt.Sprintf("%s.%s", FileName, File)
		newFile, err := file.Open()
		if err != nil {
			errorMsg := fmt.Errorf("ERROR_FILE_OPEN: %v", err)
			return "", errs.NewBusinessError(errorMsg)
		}
		fileSize := file.Size
		filename, err := m.FileUploadSvc(newFile, filepath, newFilename, fileSize)
		if err != nil {
			errorMsg := fmt.Errorf("ERROR_IMAGE: %v", err)
			return "", errs.NewBusinessError(errorMsg)
		}
		return filename, nil
	}
}

func (m *minIORepository) DeleteFileSvc(objectName string) error {
	err := m.minioClient.RemoveObject(context.Background(), m.config.BucketName, objectName, minio.RemoveObjectOptions{})
	return err
}

func (m *minIORepository) FileUploadSvc(file multipart.File, directoryPath, filename string, fileSize int64) (string, error) {
	if directoryPath == "" {
		directoryPath = "images"
	}
	objectName := fmt.Sprintf("%v/%v", directoryPath, filename)

	contentType, err := getContentType(file)
	if err != nil {
		return "", fmt.Errorf("ERROR_GETTING_CONTENT_TYPE: %v", err)
	}

	_, err = m.minioClient.PutObject(context.Background(), m.config.BucketName, objectName, file, fileSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("ERROR_UPLOADING_FILE: %v", err)
	}
	return objectName, nil
}

func getContentType(file multipart.File) (string, error) {
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		return "", fmt.Errorf("ERROR_READING_FILE: %v", err)
	}
	contentType := http.DetectContentType(buffer)
	if _, err := file.Seek(0, 0); err != nil {
		return "", fmt.Errorf("ERROR_SEEK_FILE: %v", err)
	}
	return contentType, nil
}

func NewMinIORepository(config Config) (MinIORepository, error) {
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("MINIO_SETUP_ERROR: %v", err)
	}
	log.Printf("🌟 Connected to MinIO at %s 🚀", config.Endpoint)
	return &minIORepository{
		minioClient: minioClient,
		config:      config,
	}, nil
}

// var _ output.MinIORepository = (*MinIORepository)(nil)
