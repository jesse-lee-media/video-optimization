package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"video-optimization/internal/environment"
	"video-optimization/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsCredentials "github.com/aws/aws-sdk-go-v2/credentials"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

var config *Config

type Config struct {
	Client   *awsS3.Client
	Bucket   string
	Endpoint string
}

func Init() {
	env := environment.GetEnvironment()

	logger.Logger.Infow("Initializing S3 config")
	cfg, err := awsConfig.LoadDefaultConfig(
		context.Background(),
		awsConfig.WithRegion("auto"),
		awsConfig.WithCredentialsProvider(
			awsCredentials.NewStaticCredentialsProvider(
				env.R2AccessKeyId,
				env.R2SecretAccessKey,
				"",
			),
		),
	)

	if err != nil {
		logger.Logger.Fatalf("Failed to load S3 config: %v", err)
	}

	client := awsS3.NewFromConfig(cfg, func(o *awsS3.Options) {
		o.BaseEndpoint = aws.String(env.R2Endpoint)
		o.UsePathStyle = true
	})
	config = &Config{
		Client:   client,
		Bucket:   env.R2Bucket,
		Endpoint: env.R2Endpoint,
	}

	logger.Logger.Infow("Successfully initialized S3 config", "bucket", env.R2Bucket, "endpoint", env.R2Endpoint)
}

func Download(ctx context.Context, key, filePath string) error {
	out, err := config.Client.GetObject(ctx, &awsS3.GetObjectInput{
		Bucket: aws.String(config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		logger.Logger.Errorw("Failed to get object from S3", "key", key, "error", err)
		return fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer out.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		logger.Logger.Errorw("Error creating file", "filepath", filePath, "error", err)
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, out.Body); err != nil {
		logger.Logger.Errorw("Error saving file", "filepath", filePath, "error", err)
		return fmt.Errorf("error saving file: %w", err)
	}

	logger.Logger.Infow("Successfully downloaded file from S3", "key", key, "filepath", filePath)
	return nil
}

func Upload(ctx context.Context, filePath, key, contentType string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		logger.Logger.Errorw("Failed to open file for upload", "filepath", filePath, "error", err)
		return "", fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer f.Close()

	_, err = config.Client.PutObject(ctx, &awsS3.PutObjectInput{
		Bucket:      aws.String(config.Bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		logger.Logger.Errorw("Failed to upload file to S3", "key", key, "error", err)
		return "", fmt.Errorf("failed to upload file to S3: %w", err)
	}

	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(config.Endpoint, "/"), config.Bucket, key)
	logger.Logger.Infow("Successfully uploaded file to S3", "key", key, "url", url)
	return url, nil
}

func Delete(ctx context.Context, key string) error {
	_, err := config.Client.DeleteObject(ctx, &awsS3.DeleteObjectInput{
		Bucket: aws.String(config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		logger.Logger.Errorw("Failed to delete object from S3", "key", key, "error", err)
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}
	logger.Logger.Infow("Successfully deleted object from S3", "key", key)
	return nil
}
