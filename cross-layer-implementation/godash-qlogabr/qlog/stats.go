package qlog

import "time"

type bufferStats struct {
	PlayoutTime   time.Duration
	PlayoutBytes  int64
	PlayoutFrames int32

	MaxTime   time.Duration
	MaxBytes  int64
	MaxFrames int32
}

func NewBufferStats() bufferStats {
	return bufferStats{
		PlayoutTime:   0,
		PlayoutBytes:  -1,
		PlayoutFrames: -1,

		MaxTime:   0,
		MaxBytes:  -1,
		MaxFrames: -1,
	}
}

type playheadStatus struct {
	PlayheadTime  time.Duration
	PlayheadFrame int32
}

func NewPlayheadStatus() playheadStatus {
	return playheadStatus{
		PlayheadTime:  0,
		PlayheadFrame: -1,
	}
}

type representation struct {
	ID      string
	Bitrate int64
}

func NewRepresentation() representation {
	return representation{
		ID:      "",
		Bitrate: -1,
	}
}
