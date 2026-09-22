package video

import (
	"errors"
	"path"
	"strings"
	"time"
)

var (
	ErrNotFound          = errors.New("video: not found")
	ErrForbidden         = errors.New("video: identity does not own this resource")
	ErrUnsupportedFormat = errors.New("video: unsupported video format")
	ErrVideoTooLarge     = errors.New("video: video exceeds the size limit")
	ErrUploadNotFound    = errors.New("video: uploaded object not found in storage")
	ErrUploadMismatch    = errors.New("video: uploaded object does not carry the owner recorded for this video")
	ErrNotCompleted      = errors.New("video: process request is not completed")
	ErrInvalidTransition = errors.New("video: invalid status transition")
	ErrRequestExists     = errors.New("video: process request already exists for this video")
)

type Identity struct {
	UserID string
	Email  string
}

type ObjectOwner struct {
	UserID  string
	VideoID string
}

type PresignedUpload struct {
	URL     string
	Headers map[string]string
}

type ObjectInfo struct {
	Owner ObjectOwner
}

type Video struct {
	ID          string
	UserID      string
	Filename    string
	ContentType string
	SizeBytes   int64
	ObjectKey   string
	CreatedAt   time.Time
}

func (v Video) owner() ObjectOwner { return ObjectOwner{UserID: v.UserID, VideoID: v.ID} }

var supportedExtensions = map[string]struct{}{
	".mp4": {}, ".avi": {}, ".mov": {}, ".mkv": {}, ".wmv": {}, ".flv": {}, ".webm": {},
}

func extensionOf(filename string) (string, bool) {
	ext := strings.ToLower(path.Ext(filename))
	_, ok := supportedExtensions[ext]
	return ext, ok
}
