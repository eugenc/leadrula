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

// DisputeAttachmentStore stores dispute message attachments in an S3-compatible
// bucket. Unlike avatars these are served back through the backend (GetObject)
// so access control is enforced before bytes leave the server.
type DisputeAttachmentStore struct {
	client *s3.Client
	bucket string
}

func NewDisputeAttachmentStore(endpoint, bucket, accessKey, secretKey string) *DisputeAttachmentStore {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	bucket = strings.TrimSpace(bucket)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		log.Println("dispute attachment store: S3 not configured, uploads disabled")
		return &DisputeAttachmentStore{}
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Printf("dispute attachment store: aws config: %v", err)
		return &DisputeAttachmentStore{}
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return &DisputeAttachmentStore{client: client, bucket: bucket}
}

func (s *DisputeAttachmentStore) Enabled() bool {
	return s.client != nil
}

// Put uploads the object under key and returns nil on success.
func (s *DisputeAttachmentStore) Put(ctx context.Context, key, contentType string, body io.Reader) error {
	if !s.Enabled() {
		return fmt.Errorf("dispute attachment storage not configured")
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

// Delete removes an object (used by messaging retention purge).
func (s *DisputeAttachmentStore) Delete(ctx context.Context, key string) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// Get streams the object back for an access-controlled download.
func (s *DisputeAttachmentStore) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if !s.Enabled() {
		return nil, "", fmt.Errorf("dispute attachment storage not configured")
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}
	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return out.Body, contentType, nil
}
