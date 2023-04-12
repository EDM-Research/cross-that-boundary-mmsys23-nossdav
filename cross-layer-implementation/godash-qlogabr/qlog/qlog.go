package qlog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/francoispqt/gojay"
)

// Setting of this only works when quic-go is used as a library.
// When building a binary from this repository, the version can be set using the following go build flag:
// -ldflags="-X github.com/lucas-clemente/quic-go/qlog.quicGoVersion=foobar"
var goDashVersion = "(devel)"

func init() {
	if goDashVersion != "(devel)" { // variable set by ldflags
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok { // no build info available. This happens when quic-go is not used as a library.
		return
	}
	for _, d := range info.Deps {
		if d.Path == "github.com/uccmisl/godash" {
			goDashVersion = d.Version
			if d.Replace != nil {
				if len(d.Replace.Version) > 0 {
					goDashVersion = d.Version
				} else {
					goDashVersion += " (replaced)"
				}
			}
			break
		}
	}
}

const eventChanSize = 50

type Tracer struct {
	getLogWriter func(p Perspective, streamID string) io.WriteCloser
}

// NewTracer creates a new qlog tracer.
func NewTracer(getLogWriter func(p Perspective, streamID string) io.WriteCloser) *Tracer {
	return &Tracer{getLogWriter: getLogWriter}
}

func (t *Tracer) TracerForStream(_ context.Context, p Perspective, sid StreamID) *StreamTracer {
	if w := t.getLogWriter(p, sid.String()); w != nil {
		return NewStreamTracer(w, p, sid)
	}
	return nil
}

// A ConnectionTracer records events.
type streamTracer interface {
	UpdatedMetrics(rttStats *RTTStats)
	Close()
	Debug(name, msg string)

	// Playback
	InitialiseStream(autoplay bool)
	PlayerInteraction(state InteractionState, playhead playheadStatus, speed float64)
	Rebuffer(playhead playheadStatus)
	EndStream(playhead playheadStatus)
	PlayheadProgress(playhead playheadStatus)

	// ABR
	Switch(mediaType MediaType, from, to representation)
	ChangeReadyState(state ReadyState)

	// Buffer
	UpdateBufferOccupancy(mediaType MediaType, bufferStats bufferStats)

	// Network
	Request(mediaType MediaType, resourceURL string, byteRange string)
	RequestUpdate(resourceURL string, bytesReceived int64)
	AbortRequest(resourceURL string)
}

type StreamTracer struct {
	mutex sync.Mutex

	w             io.WriteCloser
	sid           StreamID
	perspective   Perspective
	referenceTime time.Time

	events     chan event
	encodeErr  error
	runStopped chan struct{}

	RTT         *RTTStats
	lastMetrics *metrics
}

var _ streamTracer = &StreamTracer{}

func NewStreamTracer(w io.WriteCloser, p Perspective, sid StreamID) *StreamTracer {
	t := &StreamTracer{
		w:             w,
		perspective:   p,
		sid:           sid,
		runStopped:    make(chan struct{}),
		events:        make(chan event, eventChanSize),
		referenceTime: time.Now(),
		RTT:           NewRTTStats(),
	}
	go t.run()
	return t
}

func (t *StreamTracer) run() {
	defer close(t.runStopped)
	buf := &bytes.Buffer{}
	enc := gojay.NewEncoder(buf)
	tl := &topLevel{
		traces: []trace{
			{
				Title:        "MPEG-DASH goDash",
				Description:  "MPEG-DASH goDash [" + time.Now().String() + "]",
				VantagePoint: vantagePoint{Type: t.perspective, Name: "goDash application layer"},
				CommonFields: commonFields{
					ProtocolType:  "QLOG_ABR",
					ReferenceTime: t.referenceTime,
				},
			},
		},
	}
	if err := enc.Encode(tl); err != nil {
		panic(fmt.Sprintf("qlog encoding into a bytes.Buffer failed: %s", err))
	}
	buf.Truncate(buf.Len() - 3)
	if err := buf.WriteByte('\n'); err != nil {
		panic(fmt.Sprintf("qlog encoding into a bytes.Buffer failed: %s", err))
	}
	if _, err := t.w.Write(buf.Bytes()); err != nil {
		t.encodeErr = err
	}

	if _, err := t.w.Write([]byte(",\"events\": [\n")); err != nil {
		panic(fmt.Sprintf("qlog encoding events key failed: %s", err))
	}

	firsteventwritten := false

	enc = gojay.NewEncoder(t.w)
	for ev := range t.events {
		if t.encodeErr != nil { // if encoding failed, just continue draining the event channel
			continue
		}
		if firsteventwritten {
			if _, err := t.w.Write([]byte(",")); err != nil {
				t.encodeErr = err
			}
		} else {
			firsteventwritten = true
		}
		if err := enc.Encode(ev); err != nil {
			t.encodeErr = err
			continue
		}
		if _, err := t.w.Write([]byte("\n")); err != nil {
			t.encodeErr = err
		}
	}
	if _, err := t.w.Write([]byte("]}]}\n")); err != nil {
		panic(fmt.Sprintf("qlog encoding close events key failed: %s", err))
	}
}

func (t *StreamTracer) Close() {
	if err := t.export(); err != nil {
		log.Printf("exporting qlog failed: %s\n", err)
	}
}

// export writes a qlog.
func (t *StreamTracer) export() error {
	close(t.events)
	<-t.runStopped
	if t.encodeErr != nil {
		return t.encodeErr
	}
	return t.w.Close()
}

func (t *StreamTracer) recordEvent(eventTime time.Time, details eventDetails) {
	t.events <- event{
		RelativeTime: eventTime.Sub(t.referenceTime),
		eventDetails: details,
	}
}

func (t *StreamTracer) Debug(name, msg string) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventGeneric{
		name: name,
		msg:  msg,
	})
	t.mutex.Unlock()
}

func (t *StreamTracer) UpdatedMetrics(rttStats *RTTStats) {
	m := &metrics{
		MinRTT:      rttStats.MinRTT(),
		SmoothedRTT: rttStats.SmoothedRTT(),
		LatestRTT:   rttStats.LatestRTT(),
		RTTVariance: rttStats.MeanDeviation(),
	}
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventMetricsUpdated{
		Last:    t.lastMetrics,
		Current: m,
	})
	t.lastMetrics = m
	t.mutex.Unlock()
}

// Playback

func (t *StreamTracer) InitialiseStream(autoplay bool) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventPlaybackStreamInitialised{autoplay: autoplay})
	t.mutex.Unlock()
}

func (t *StreamTracer) PlayerInteraction(state InteractionState, playhead playheadStatus, speed float64) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventPlaybackInteraction{state: state, playhead: playhead, speed: speed})
	t.mutex.Unlock()
}

func (t *StreamTracer) Rebuffer(playhead playheadStatus) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventPlaybackRebuffer{playhead: playhead})
	t.mutex.Unlock()
}

func (t *StreamTracer) EndStream(playhead playheadStatus) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventPlaybackStreamEnd{playhead: playhead})
	t.mutex.Unlock()
}

func (t *StreamTracer) PlayheadProgress(playhead playheadStatus) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventPlaybackPlayheadProgress{playhead: playhead})
	t.mutex.Unlock()
}

// ABR

func (t *StreamTracer) Switch(mediaType MediaType, from, to representation) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventABRSwitch{mediaType: mediaType, from: from, to: to})
	t.mutex.Unlock()
}

func (t *StreamTracer) ChangeReadyState(state ReadyState) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventABRReadyStateChange{state: state})
	t.mutex.Unlock()
}

// Buffer

func (t *StreamTracer) UpdateBufferOccupancy(mediaType MediaType, bufferStats bufferStats) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventBufferOccupancyUpdated{media_type: mediaType, buffer_stats: bufferStats})
	t.mutex.Unlock()
}

// Network

func (t *StreamTracer) Request(mediaType MediaType, resourceURL string, byteRange string) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventNetworkRequest{media_type: mediaType, resource_url: resourceURL, byte_range: byteRange})
	t.mutex.Unlock()
}

func (t *StreamTracer) RequestUpdate(resourceURL string, bytesReceived int64) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventNetworkRequestUpdate{resource_url: resourceURL, bytesReceived: bytesReceived})
	t.mutex.Unlock()
}

func (t *StreamTracer) AbortRequest(resourceURL string) {
	t.mutex.Lock()
	t.recordEvent(time.Now(), &eventNetworkAbort{resource_url: resourceURL})
	t.mutex.Unlock()
}
