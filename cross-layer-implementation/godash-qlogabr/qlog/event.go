package qlog

import (
	"time"

	"github.com/francoispqt/gojay"
)

func milliseconds(dur time.Duration) float64 { return float64(dur.Nanoseconds()) / 1e6 }

type eventDetails interface {
	Category() category
	Name() string
	gojay.MarshalerJSONObject
}

type event struct {
	RelativeTime time.Duration
	eventDetails
}

var _ gojay.MarshalerJSONObject = event{}

func (e event) IsNil() bool { return false }
func (e event) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Float64Key("time", milliseconds(e.RelativeTime))
	enc.StringKey("name", e.Category().String()+":"+e.Name())
	enc.ObjectKey("data", e.eventDetails)
}

type eventGeneric struct {
	name string
	msg  string
}

func (e eventGeneric) Category() category { return categoryGeneric }
func (e eventGeneric) Name() string       { return e.name }
func (e eventGeneric) IsNil() bool        { return false }

func (e eventGeneric) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("details", e.msg)
}

type metrics struct {
	MinRTT      time.Duration
	SmoothedRTT time.Duration
	LatestRTT   time.Duration
	RTTVariance time.Duration
}

type eventMetricsUpdated struct {
	Last    *metrics
	Current *metrics
}

func (e eventMetricsUpdated) Category() category { return categoryGeneric }
func (e eventMetricsUpdated) Name() string       { return "metrics_updated" }
func (e eventMetricsUpdated) IsNil() bool        { return false }

func (e eventMetricsUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	if e.Last == nil || e.Last.MinRTT != e.Current.MinRTT {
		enc.FloatKey("min_rtt", milliseconds(e.Current.MinRTT))
	}
	if e.Last == nil || e.Last.SmoothedRTT != e.Current.SmoothedRTT {
		enc.FloatKey("smoothed_rtt", milliseconds(e.Current.SmoothedRTT))
	}
	if e.Last == nil || e.Last.LatestRTT != e.Current.LatestRTT {
		enc.FloatKey("latest_rtt", milliseconds(e.Current.LatestRTT))
	}
	if e.Last == nil || e.Last.RTTVariance != e.Current.RTTVariance {
		enc.FloatKey("rtt_variance", milliseconds(e.Current.RTTVariance))
	}
}

// Playback

type eventPlaybackStreamInitialised struct {
	autoplay bool
}

func (e eventPlaybackStreamInitialised) Category() category { return categoryPlayback }
func (e eventPlaybackStreamInitialised) Name() string       { return "stream_initialised" }
func (e eventPlaybackStreamInitialised) IsNil() bool        { return false }

func (e eventPlaybackStreamInitialised) MarshalJSONObject(enc *gojay.Encoder) {
	enc.BoolKey("autoplay", e.autoplay)
}

type eventPlaybackInteraction struct {
	state    InteractionState
	playhead playheadStatus
	speed    float64
}

func (e eventPlaybackInteraction) Category() category { return categoryPlayback }
func (e eventPlaybackInteraction) Name() string       { return "player_interaction" }
func (e eventPlaybackInteraction) IsNil() bool        { return false }

func (e eventPlaybackInteraction) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("state", e.state.String())
	enc.Int64Key("playhead_ms", e.playhead.PlayheadTime.Milliseconds())
	if e.playhead.PlayheadFrame >= 0 {
		enc.Int64Key("playhead_frame", int64(e.playhead.PlayheadFrame))
	}
	enc.Float64KeyOmitEmpty("speed", e.speed)
}

type eventPlaybackRebuffer struct {
	playhead playheadStatus
}

func (e eventPlaybackRebuffer) Category() category { return categoryPlayback }
func (e eventPlaybackRebuffer) Name() string       { return "rebuffer" }
func (e eventPlaybackRebuffer) IsNil() bool        { return false }

func (e eventPlaybackRebuffer) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Int64Key("playhead_ms", e.playhead.PlayheadTime.Milliseconds())
	if e.playhead.PlayheadFrame >= 0 {
		enc.Int64Key("playhead_frame", int64(e.playhead.PlayheadFrame))
	}
}

type eventPlaybackStreamEnd struct {
	playhead playheadStatus
}

func (e eventPlaybackStreamEnd) Category() category { return categoryPlayback }
func (e eventPlaybackStreamEnd) Name() string       { return "stream_end" }
func (e eventPlaybackStreamEnd) IsNil() bool        { return false }

func (e eventPlaybackStreamEnd) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Int64Key("playhead_ms", e.playhead.PlayheadTime.Milliseconds())
	if e.playhead.PlayheadFrame >= 0 {
		enc.Int64Key("playhead_frame", int64(e.playhead.PlayheadFrame))
	}
}

type eventPlaybackPlayheadProgress struct {
	playhead playheadStatus
}

func (e eventPlaybackPlayheadProgress) Category() category { return categoryPlayback }
func (e eventPlaybackPlayheadProgress) Name() string       { return "playhead_progress" }
func (e eventPlaybackPlayheadProgress) IsNil() bool        { return false }

func (e eventPlaybackPlayheadProgress) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Int64Key("playhead_ms", e.playhead.PlayheadTime.Milliseconds())
	if e.playhead.PlayheadFrame >= 0 {
		enc.Int64Key("playhead_frame", int64(e.playhead.PlayheadFrame))
	}
}

// ABR

type eventABRSwitch struct {
	from      representation
	to        representation
	mediaType MediaType
}

func (e eventABRSwitch) Category() category { return categoryABR }
func (e eventABRSwitch) Name() string       { return "switch" }
func (e eventABRSwitch) IsNil() bool        { return false }

func (e eventABRSwitch) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("media_type", e.mediaType.String())
	enc.StringKeyOmitEmpty("from_id", e.from.ID)
	if e.from.Bitrate >= 0 {
		enc.Int64Key("from_bitrate", e.from.Bitrate)
	}
	enc.StringKey("to_id", e.to.ID)
	if e.to.Bitrate >= 0 {
		enc.Int64Key("to_bitrate", e.to.Bitrate)
	}
}

type eventABRReadyStateChange struct {
	state ReadyState
}

func (e eventABRReadyStateChange) Category() category { return categoryABR }
func (e eventABRReadyStateChange) Name() string       { return "readystate_change" }
func (e eventABRReadyStateChange) IsNil() bool        { return false }

func (e eventABRReadyStateChange) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("state", e.state.String())
}

// Buffer

type eventBufferOccupancyUpdated struct {
	media_type   MediaType
	buffer_stats bufferStats
}

func (e eventBufferOccupancyUpdated) Category() category { return categoryBuffer }
func (e eventBufferOccupancyUpdated) Name() string       { return "occupancy_update" }
func (e eventBufferOccupancyUpdated) IsNil() bool        { return false }

func (e eventBufferOccupancyUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("media_type", e.media_type.String())
	enc.Int64Key("playout_ms", e.buffer_stats.PlayoutTime.Milliseconds())
	if e.buffer_stats.PlayoutBytes >= 0 {
		enc.Int64Key("playout_bytes", e.buffer_stats.PlayoutBytes)
	}
	if e.buffer_stats.PlayoutFrames >= 0 {
		enc.Int64Key("playout_frames", int64(e.buffer_stats.PlayoutFrames))
	}

	enc.Int64Key("max_ms", e.buffer_stats.MaxTime.Milliseconds())
	if e.buffer_stats.MaxBytes >= 0 {
		enc.Int64Key("max_bytes", e.buffer_stats.MaxBytes)
	}
	if e.buffer_stats.MaxFrames >= 0 {
		enc.Int64Key("max_frames", int64(e.buffer_stats.MaxFrames))
	}
}

// Network

type eventNetworkRequest struct {
	media_type   MediaType
	resource_url string
	byte_range   string
}

func (e eventNetworkRequest) Category() category { return categoryNetwork }
func (e eventNetworkRequest) Name() string       { return "request" }
func (e eventNetworkRequest) IsNil() bool        { return false }

func (e eventNetworkRequest) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("media_type", e.media_type.String())
	enc.StringKey("resource_url", e.resource_url)
	enc.StringKeyOmitEmpty("range", e.byte_range)
}

type eventNetworkRequestUpdate struct {
	resource_url  string
	bytesReceived int64
}

func (e eventNetworkRequestUpdate) Category() category { return categoryNetwork }
func (e eventNetworkRequestUpdate) Name() string       { return "request_update" }
func (e eventNetworkRequestUpdate) IsNil() bool        { return false }

func (e eventNetworkRequestUpdate) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("resource_url", e.resource_url)
	enc.Int64Key("bytes_received", e.bytesReceived)
}

type eventNetworkAbort struct {
	resource_url string
}

func (e eventNetworkAbort) Category() category { return categoryNetwork }
func (e eventNetworkAbort) Name() string       { return "abort" }
func (e eventNetworkAbort) IsNil() bool        { return false }

func (e eventNetworkAbort) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("resource_url", e.resource_url)
}
