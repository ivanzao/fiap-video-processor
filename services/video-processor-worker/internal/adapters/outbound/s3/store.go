package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type Store struct {
	client *awss3.Client
	bucket string
}

func NewStore(client *awss3.Client, bucket string) *Store {
	return &Store{client: client, bucket: bucket}
}

func (s *Store) Download(ctx context.Context, key, dst string) error {
	out, err := s.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return video.ErrObjectNotFound
		}
		return fmt.Errorf("s3: get object: %w", err)
	}
	defer func() { _ = out.Body.Close() }()
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("s3: create %s: %w", dst, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("s3: download body: %w", err)
	}
	return f.Close()
}

func (s *Store) Upload(ctx context.Context, src, key, contentType string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("s3: open %s: %w", src, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("s3: stat %s: %w", src, err)
	}
	_, err = s.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: f,
		ContentType: aws.String(contentType), ContentLength: aws.Int64(info.Size()),
	})
	if err != nil {
		return fmt.Errorf("s3: put object: %w", err)
	}
	return nil
}
