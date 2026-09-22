package s3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

const (
	metaUserID  = "user-id"
	metaVideoID = "video-id"
)

type Store struct {
	client  *awss3.Client
	presign *awss3.PresignClient
	bucket  string
}

func NewStore(client, presignClient *awss3.Client, bucket string) *Store {
	return &Store{client: client, presign: awss3.NewPresignClient(presignClient), bucket: bucket}
}

func (s *Store) PresignUpload(ctx context.Context, key, contentType string, sizeBytes int64, owner video.ObjectOwner, ttl time.Duration) (video.PresignedUpload, error) {
	metadata := map[string]string{metaUserID: owner.UserID, metaVideoID: owner.VideoID}
	req, err := s.presign.PresignPutObject(ctx, &awss3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(sizeBytes),
		Metadata:      metadata,
	}, awss3.WithPresignExpires(ttl))
	if err != nil {
		return video.PresignedUpload{}, fmt.Errorf("s3: presign upload: %w", err)
	}
	headers := map[string]string{"Content-Type": contentType}
	for name, value := range metadata {
		headers["x-amz-meta-"+name] = value
	}
	return video.PresignedUpload{URL: req.URL, Headers: headers}, nil
}

func (s *Store) Head(ctx context.Context, key string) (video.ObjectInfo, error) {
	out, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
			return video.ObjectInfo{}, video.ErrUploadNotFound
		}
		return video.ObjectInfo{}, fmt.Errorf("s3: head object: %w", err)
	}
	return video.ObjectInfo{Owner: video.ObjectOwner{UserID: out.Metadata[metaUserID], VideoID: out.Metadata[metaVideoID]}}, nil
}

func (s *Store) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, awss3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3: presign download: %w", err)
	}
	return req.URL, nil
}
