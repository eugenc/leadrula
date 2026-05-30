package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// AvatarStore uploads user profile images to an S3-compatible bucket (e.g. Railway).
type AvatarStore struct {
	client     *s3.Client
	bucket     string
	publicBase string
}

func NewAvatarStore(endpoint, bucket, accessKey, secretKey, publicBase string) *AvatarStore {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	bucket = strings.TrimSpace(bucket)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)
	publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		log.Println("avatar store: S3 not configured, uploads disabled")
		return &AvatarStore{}
	}
	if publicBase == "" {
		publicBase = endpoint + "/" + bucket
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Printf("avatar store: aws config: %v", err)
		return &AvatarStore{}
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &AvatarStore{client: client, bucket: bucket, publicBase: publicBase}
}

func (s *AvatarStore) Enabled() bool {
	return s.client != nil
}

// Put uploads the object and returns its public URL.
func (s *AvatarStore) Put(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("avatar storage not configured")
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return s.publicBase + "/" + key, nil
}
