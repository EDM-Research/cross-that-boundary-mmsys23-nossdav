package qlog

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

var generalTracer *Tracer = nil
var MainTracer *StreamTracer = nil

func init() {
	generalTracer = NewTracer(func(p Perspective, streamID string) io.WriteCloser {
		filename := fmt.Sprintf("logs/"+p.String()+"_abr_%s.qlog", streamID)
		//filename := "logs/client.qlog"
		f, err := os.Create(filename)
		//f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Creating ABR qlog file %s.\n", filename)
		return NewBufferedWriteCloser(bufio.NewWriter(f), f)
	})
	//TODO find a stream id for this tracer
	MainTracer = generalTracer.TracerForStream(context.Background(), PerspectiveClient, "")
}

type bufferedWriteCloser struct {
	*bufio.Writer
	io.Closer
}

// NewBufferedWriteCloser creates an io.WriteCloser from a bufio.Writer and an io.Closer
func NewBufferedWriteCloser(writer *bufio.Writer, closer io.Closer) io.WriteCloser {
	return &bufferedWriteCloser{
		Writer: writer,
		Closer: closer,
	}
}

func (h bufferedWriteCloser) Close() error {
	if err := h.Writer.Flush(); err != nil {
		return err
	}
	return h.Closer.Close()
}
