package events

const (
	TypeVideoProcessingRequested = "VideoProcessingRequested"
	TypeVideoProcessingStarted   = "VideoProcessingStarted"
	TypeVideoProcessingCompleted = "VideoProcessingCompleted"
	TypeVideoProcessingFailed    = "VideoProcessingFailed"
)

type VideoProcessingRequested struct {
	RequestID string `json:"requestId"`
	VideoID   string `json:"videoId"`
	UserID    string `json:"userId"`
	UserEmail string `json:"userEmail"`
	ObjectKey string `json:"objectKey"`
	Filename  string `json:"filename"`
}

func (VideoProcessingRequested) EventType() string { return TypeVideoProcessingRequested }

type VideoProcessingStarted struct {
	RequestID string `json:"requestId"`
	Attempt   int    `json:"attempt"`
}

func (VideoProcessingStarted) EventType() string { return TypeVideoProcessingStarted }

type VideoProcessingCompleted struct {
	RequestID  string `json:"requestId"`
	ZipKey     string `json:"zipKey"`
	FrameCount int    `json:"frameCount"`
	DurationMs int64  `json:"durationMs"`
}

func (VideoProcessingCompleted) EventType() string { return TypeVideoProcessingCompleted }

type VideoProcessingFailed struct {
	RequestID string `json:"requestId"`
	Reason    string `json:"reason"`
	Retryable bool   `json:"retryable"`
	Attempt   int    `json:"attempt"`
}

func (VideoProcessingFailed) EventType() string { return TypeVideoProcessingFailed }
