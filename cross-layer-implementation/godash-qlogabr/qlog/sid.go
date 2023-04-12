package qlog

import (
	"crypto/rand"
)

type StreamID string

// GenerateConnectionID generates a connection ID using cryptographic random
func GenerateStreamID(len int) (StreamID, error) {
	b := make([]byte, len)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return StreamID(b), nil
}

func (c StreamID) String() string {
	if len(c) == 0 {
		return "(empty)"
	}
	return string(c)
}
