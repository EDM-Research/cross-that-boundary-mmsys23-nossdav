package qlog

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/utils"
	"github.com/lucas-clemente/quic-go/logging"

	"github.com/francoispqt/gojay"
)

func milliseconds(dur time.Duration) float64 { return float64(dur.Nanoseconds()) / 1e6 }

type eventDetails interface {
	Category() category
	Name() string
	EventType() string
	gojay.MarshalerJSONObject
}

type Event struct {
	RelativeTime time.Duration
	eventDetails
}

func (e Event) GetEventDetails() eventDetails {
	return e.eventDetails
}

var _ gojay.MarshalerJSONObject = Event{}

func (e Event) IsNil() bool { return false }
func (e Event) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Float64Key("time", milliseconds(e.RelativeTime))
	enc.StringKey("name", e.Category().String()+":"+e.Name())
	enc.ObjectKey("data", e.eventDetails)
}

type versions []versionNumber

func (v versions) IsNil() bool { return false }
func (v versions) MarshalJSONArray(enc *gojay.Encoder) {
	for _, e := range v {
		enc.AddString(e.String())
	}
}

type rawInfo struct {
	Length        logging.ByteCount // full packet length, including header and AEAD authentication tag
	PayloadLength logging.ByteCount // length of the packet payload, excluding AEAD tag
}

func (i rawInfo) IsNil() bool { return false }
func (i rawInfo) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Uint64Key("length", uint64(i.Length))
	enc.Uint64KeyOmitEmpty("payload_length", uint64(i.PayloadLength))
}

type EventConnectionStarted struct {
	SrcAddr  *net.UDPAddr
	DestAddr *net.UDPAddr

	SrcConnectionID  protocol.ConnectionID
	DestConnectionID protocol.ConnectionID
}

var _ eventDetails = &EventConnectionStarted{}

func (e EventConnectionStarted) Category() category { return categoryTransport }
func (e EventConnectionStarted) Name() string       { return "connection_started" }
func (e EventConnectionStarted) EventType() string  { return "EventConnectionStarted" }
func (e EventConnectionStarted) IsNil() bool        { return false }

func (e EventConnectionStarted) MarshalJSONObject(enc *gojay.Encoder) {
	if utils.IsIPv4(e.SrcAddr.IP) {
		enc.StringKey("ip_version", "ipv4")
	} else {
		enc.StringKey("ip_version", "ipv6")
	}
	enc.StringKey("src_ip", e.SrcAddr.IP.String())
	enc.IntKey("src_port", e.SrcAddr.Port)
	enc.StringKey("dst_ip", e.DestAddr.IP.String())
	enc.IntKey("dst_port", e.DestAddr.Port)
	enc.StringKey("src_cid", e.SrcConnectionID.String())
	enc.StringKey("dst_cid", e.DestConnectionID.String())
}

type EventVersionNegotiated struct {
	clientVersions, serverVersions []versionNumber
	chosenVersion                  versionNumber
}

func (e EventVersionNegotiated) Category() category { return categoryTransport }
func (e EventVersionNegotiated) Name() string       { return "version_information" }
func (e EventVersionNegotiated) EventType() string  { return "EventVersionNegotiated" }
func (e EventVersionNegotiated) IsNil() bool        { return false }

func (e EventVersionNegotiated) MarshalJSONObject(enc *gojay.Encoder) {
	if len(e.clientVersions) > 0 {
		enc.ArrayKey("client_versions", versions(e.clientVersions))
	}
	if len(e.serverVersions) > 0 {
		enc.ArrayKey("server_versions", versions(e.serverVersions))
	}
	enc.StringKey("chosen_version", e.chosenVersion.String())
}

type EventConnectionClosed struct {
	e error
}

func (e EventConnectionClosed) Category() category { return categoryTransport }
func (e EventConnectionClosed) Name() string       { return "connection_closed" }
func (e EventConnectionClosed) EventType() string  { return "EventConnectionClosed" }
func (e EventConnectionClosed) IsNil() bool        { return false }

func (e EventConnectionClosed) MarshalJSONObject(enc *gojay.Encoder) {
	var (
		statelessResetErr     *quic.StatelessResetError
		handshakeTimeoutErr   *quic.HandshakeTimeoutError
		idleTimeoutErr        *quic.IdleTimeoutError
		applicationErr        *quic.ApplicationError
		transportErr          *quic.TransportError
		versionNegotiationErr *quic.VersionNegotiationError
	)
	switch {
	case errors.As(e.e, &statelessResetErr):
		enc.StringKey("owner", ownerRemote.String())
		enc.StringKey("trigger", "stateless_reset")
		enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", statelessResetErr.Token))
	case errors.As(e.e, &handshakeTimeoutErr):
		enc.StringKey("owner", ownerLocal.String())
		enc.StringKey("trigger", "handshake_timeout")
	case errors.As(e.e, &idleTimeoutErr):
		enc.StringKey("owner", ownerLocal.String())
		enc.StringKey("trigger", "idle_timeout")
	case errors.As(e.e, &applicationErr):
		owner := ownerLocal
		if applicationErr.Remote {
			owner = ownerRemote
		}
		enc.StringKey("owner", owner.String())
		enc.Uint64Key("application_code", uint64(applicationErr.ErrorCode))
		enc.StringKey("reason", applicationErr.ErrorMessage)
	case errors.As(e.e, &transportErr):
		owner := ownerLocal
		if transportErr.Remote {
			owner = ownerRemote
		}
		enc.StringKey("owner", owner.String())
		enc.StringKey("connection_code", transportError(transportErr.ErrorCode).String())
		enc.StringKey("reason", transportErr.ErrorMessage)
	case errors.As(e.e, &versionNegotiationErr):
		enc.StringKey("owner", ownerRemote.String())
		enc.StringKey("trigger", "version_negotiation")
	}
}

type EventPacketSent struct {
	Header        packetHeader
	Length        logging.ByteCount
	PayloadLength logging.ByteCount
	Frames        frames
	IsCoalesced   bool
	Trigger       string
}

var _ eventDetails = EventPacketSent{}

func (e EventPacketSent) Category() category { return categoryTransport }
func (e EventPacketSent) Name() string       { return "packet_sent" }
func (e EventPacketSent) EventType() string  { return "EventPacketSent" }
func (e EventPacketSent) IsNil() bool        { return false }

func (e EventPacketSent) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
	enc.ObjectKey("raw", rawInfo{Length: e.Length, PayloadLength: e.PayloadLength})
	enc.ArrayKeyOmitEmpty("frames", e.Frames)
	enc.BoolKeyOmitEmpty("is_coalesced", e.IsCoalesced)
	enc.StringKeyOmitEmpty("trigger", e.Trigger)
}

type EventPacketReceived struct {
	Header        gojay.MarshalerJSONObject // either a shortHeader or a packetHeader
	Length        logging.ByteCount
	PayloadLength logging.ByteCount
	Frames        frames
	IsCoalesced   bool
	Trigger       string
}

var _ eventDetails = EventPacketReceived{}

func (e EventPacketReceived) Category() category { return categoryTransport }
func (e EventPacketReceived) Name() string       { return "packet_received" }
func (e EventPacketReceived) EventType() string  { return "EventPacketReceived" }
func (e EventPacketReceived) IsNil() bool        { return false }

func (e EventPacketReceived) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
	enc.ObjectKey("raw", rawInfo{Length: e.Length, PayloadLength: e.PayloadLength})
	enc.ArrayKeyOmitEmpty("frames", e.Frames)
	enc.BoolKeyOmitEmpty("is_coalesced", e.IsCoalesced)
	enc.StringKeyOmitEmpty("trigger", e.Trigger)
}

func (e EventPacketReceived) GetPayloadLength() uint64 {
	return uint64(e.PayloadLength)
}

type EventRetryReceived struct {
	Header packetHeader
}

func (e EventRetryReceived) Category() category { return categoryTransport }
func (e EventRetryReceived) Name() string       { return "packet_received" }
func (e EventRetryReceived) EventType() string  { return "EventRetryReceived" }
func (e EventRetryReceived) IsNil() bool        { return false }

func (e EventRetryReceived) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
}

type EventVersionNegotiationReceived struct {
	Header            packetHeaderVersionNegotiation
	SupportedVersions []versionNumber
}

func (e EventVersionNegotiationReceived) Category() category { return categoryTransport }
func (e EventVersionNegotiationReceived) Name() string       { return "packet_received" }
func (e EventVersionNegotiationReceived) EventType() string  { return "EventVersionNegotiationReceived" }
func (e EventVersionNegotiationReceived) IsNil() bool        { return false }

func (e EventVersionNegotiationReceived) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", e.Header)
	enc.ArrayKey("supported_versions", versions(e.SupportedVersions))
}

type EventPacketBuffered struct {
	PacketType logging.PacketType
	PacketSize protocol.ByteCount
}

func (e EventPacketBuffered) Category() category { return categoryTransport }
func (e EventPacketBuffered) Name() string       { return "packet_buffered" }
func (e EventPacketBuffered) EventType() string  { return "EventPacketBuffered" }
func (e EventPacketBuffered) IsNil() bool        { return false }

func (e EventPacketBuffered) MarshalJSONObject(enc *gojay.Encoder) {
	//nolint:gosimple
	enc.ObjectKey("header", packetHeaderWithType{PacketType: e.PacketType})
	enc.ObjectKey("raw", rawInfo{Length: e.PacketSize})
	enc.StringKey("trigger", "keys_unavailable")
}

type EventPacketDropped struct {
	PacketType logging.PacketType
	PacketSize protocol.ByteCount
	Trigger    packetDropReason
}

func (e EventPacketDropped) Category() category { return categoryTransport }
func (e EventPacketDropped) Name() string       { return "packet_dropped" }
func (e EventPacketDropped) EventType() string  { return "EventPacketDropped" }
func (e EventPacketDropped) IsNil() bool        { return false }

func (e EventPacketDropped) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", packetHeaderWithType{PacketType: e.PacketType})
	enc.ObjectKey("raw", rawInfo{Length: e.PacketSize})
	enc.StringKey("trigger", e.Trigger.String())
}

type metrics struct {
	MinRTT      time.Duration
	SmoothedRTT time.Duration
	LatestRTT   time.Duration
	RTTVariance time.Duration

	CongestionWindow protocol.ByteCount
	BytesInFlight    protocol.ByteCount
	PacketsInFlight  int
}

type EventMetricsUpdated struct {
	Last    *metrics
	Current *metrics
}

func (e EventMetricsUpdated) Category() category { return categoryRecovery }
func (e EventMetricsUpdated) Name() string       { return "metrics_updated" }
func (e EventMetricsUpdated) EventType() string  { return "EventMetricsUpdated" }
func (e EventMetricsUpdated) IsNil() bool        { return false }

func (e EventMetricsUpdated) MarshalJSONObject(enc *gojay.Encoder) {
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

	if e.Last == nil || e.Last.CongestionWindow != e.Current.CongestionWindow {
		enc.Uint64Key("congestion_window", uint64(e.Current.CongestionWindow))
	}
	if e.Last == nil || e.Last.BytesInFlight != e.Current.BytesInFlight {
		enc.Uint64Key("bytes_in_flight", uint64(e.Current.BytesInFlight))
	}
	if e.Last == nil || e.Last.PacketsInFlight != e.Current.PacketsInFlight {
		enc.Uint64KeyOmitEmpty("packets_in_flight", uint64(e.Current.PacketsInFlight))
	}
}

type EventUpdatedPTO struct {
	Value uint32
}

func (e EventUpdatedPTO) Category() category { return categoryRecovery }
func (e EventUpdatedPTO) Name() string       { return "metrics_updated" }
func (e EventUpdatedPTO) EventType() string  { return "EventUpdatedPTO" }
func (e EventUpdatedPTO) IsNil() bool        { return false }

func (e EventUpdatedPTO) MarshalJSONObject(enc *gojay.Encoder) {
	enc.Uint32Key("pto_count", e.Value)
}

type EventPacketLost struct {
	PacketType   logging.PacketType
	PacketNumber protocol.PacketNumber
	Trigger      packetLossReason
}

func (e EventPacketLost) Category() category { return categoryRecovery }
func (e EventPacketLost) Name() string       { return "packet_lost" }
func (e EventPacketLost) EventType() string  { return "EventPacketLost" }
func (e EventPacketLost) IsNil() bool        { return false }

func (e EventPacketLost) MarshalJSONObject(enc *gojay.Encoder) {
	enc.ObjectKey("header", packetHeaderWithTypeAndPacketNumber{
		PacketType:   e.PacketType,
		PacketNumber: e.PacketNumber,
	})
	enc.StringKey("trigger", e.Trigger.String())
}

type EventKeyUpdated struct {
	Trigger    keyUpdateTrigger
	KeyType    keyType
	Generation protocol.KeyPhase
	// we don't log the keys here, so we don't need `old` and `new`.
}

func (e EventKeyUpdated) Category() category { return categorySecurity }
func (e EventKeyUpdated) Name() string       { return "key_updated" }
func (e EventKeyUpdated) EventType() string  { return "EventKeyUpdated" }
func (e EventKeyUpdated) IsNil() bool        { return false }

func (e EventKeyUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("trigger", e.Trigger.String())
	enc.StringKey("key_type", e.KeyType.String())
	if e.KeyType == keyTypeClient1RTT || e.KeyType == keyTypeServer1RTT {
		enc.Uint64Key("generation", uint64(e.Generation))
	}
}

var _ eventDetails = EventKeyDiscarded{}

type EventKeyDiscarded struct {
	KeyType    keyType
	Generation protocol.KeyPhase
}

func (e EventKeyDiscarded) Category() category { return categorySecurity }
func (e EventKeyDiscarded) Name() string       { return "key_discarded" }
func (e EventKeyDiscarded) EventType() string  { return "EventKeyDiscarded" }
func (e EventKeyDiscarded) IsNil() bool        { return false }

func (e EventKeyDiscarded) MarshalJSONObject(enc *gojay.Encoder) {
	if e.KeyType != keyTypeClient1RTT && e.KeyType != keyTypeServer1RTT {
		enc.StringKey("trigger", "tls")
	}
	enc.StringKey("key_type", e.KeyType.String())
	if e.KeyType == keyTypeClient1RTT || e.KeyType == keyTypeServer1RTT {
		enc.Uint64Key("generation", uint64(e.Generation))
	}
}

type EventTransportParameters struct {
	Restore bool
	Owner   owner
	SentBy  protocol.Perspective

	OriginalDestinationConnectionID protocol.ConnectionID
	InitialSourceConnectionID       protocol.ConnectionID
	RetrySourceConnectionID         *protocol.ConnectionID

	StatelessResetToken     *protocol.StatelessResetToken
	DisableActiveMigration  bool
	MaxIdleTimeout          time.Duration
	MaxUDPPayloadSize       protocol.ByteCount
	AckDelayExponent        uint8
	MaxAckDelay             time.Duration
	ActiveConnectionIDLimit uint64

	InitialMaxData                 protocol.ByteCount
	InitialMaxStreamDataBidiLocal  protocol.ByteCount
	InitialMaxStreamDataBidiRemote protocol.ByteCount
	InitialMaxStreamDataUni        protocol.ByteCount
	InitialMaxStreamsBidi          int64
	InitialMaxStreamsUni           int64

	PreferredAddress *preferredAddress

	MaxDatagramFrameSize protocol.ByteCount
}

func (e EventTransportParameters) Category() category { return categoryTransport }
func (e EventTransportParameters) Name() string {
	if e.Restore {
		return "parameters_restored"
	}
	return "parameters_set"
}
func (e EventTransportParameters) EventType() string { return "EventTransportParameters" }
func (e EventTransportParameters) IsNil() bool       { return false }

func (e EventTransportParameters) MarshalJSONObject(enc *gojay.Encoder) {
	if !e.Restore {
		enc.StringKey("owner", e.Owner.String())
		if e.SentBy == protocol.PerspectiveServer {
			enc.StringKey("original_destination_connection_id", e.OriginalDestinationConnectionID.String())
			if e.StatelessResetToken != nil {
				enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", e.StatelessResetToken[:]))
			}
			if e.RetrySourceConnectionID != nil {
				enc.StringKey("retry_source_connection_id", (*e.RetrySourceConnectionID).String())
			}
		}
		enc.StringKey("initial_source_connection_id", e.InitialSourceConnectionID.String())
	}
	enc.BoolKey("disable_active_migration", e.DisableActiveMigration)
	enc.FloatKeyOmitEmpty("max_idle_timeout", milliseconds(e.MaxIdleTimeout))
	enc.Int64KeyNullEmpty("max_udp_payload_size", int64(e.MaxUDPPayloadSize))
	enc.Uint8KeyOmitEmpty("ack_delay_exponent", e.AckDelayExponent)
	enc.FloatKeyOmitEmpty("max_ack_delay", milliseconds(e.MaxAckDelay))
	enc.Uint64KeyOmitEmpty("active_connection_id_limit", e.ActiveConnectionIDLimit)

	enc.Int64KeyOmitEmpty("initial_max_data", int64(e.InitialMaxData))
	enc.Int64KeyOmitEmpty("initial_max_stream_data_bidi_local", int64(e.InitialMaxStreamDataBidiLocal))
	enc.Int64KeyOmitEmpty("initial_max_stream_data_bidi_remote", int64(e.InitialMaxStreamDataBidiRemote))
	enc.Int64KeyOmitEmpty("initial_max_stream_data_uni", int64(e.InitialMaxStreamDataUni))
	enc.Int64KeyOmitEmpty("initial_max_streams_bidi", e.InitialMaxStreamsBidi)
	enc.Int64KeyOmitEmpty("initial_max_streams_uni", e.InitialMaxStreamsUni)

	if e.PreferredAddress != nil {
		enc.ObjectKey("preferred_address", e.PreferredAddress)
	}
	if e.MaxDatagramFrameSize != protocol.InvalidByteCount {
		enc.Int64Key("max_datagram_frame_size", int64(e.MaxDatagramFrameSize))
	}
}

type preferredAddress struct {
	IPv4, IPv6          net.IP
	PortV4, PortV6      uint16
	ConnectionID        protocol.ConnectionID
	StatelessResetToken protocol.StatelessResetToken
}

var _ gojay.MarshalerJSONObject = &preferredAddress{}

func (a preferredAddress) IsNil() bool { return false }
func (a preferredAddress) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("ip_v4", a.IPv4.String())
	enc.Uint16Key("port_v4", a.PortV4)
	enc.StringKey("ip_v6", a.IPv6.String())
	enc.Uint16Key("port_v6", a.PortV6)
	enc.StringKey("connection_id", a.ConnectionID.String())
	enc.StringKey("stateless_reset_token", fmt.Sprintf("%x", a.StatelessResetToken))
}

type EventLossTimerSet struct {
	TimerType timerType
	EncLevel  protocol.EncryptionLevel
	Delta     time.Duration
}

func (e EventLossTimerSet) Category() category { return categoryRecovery }
func (e EventLossTimerSet) Name() string       { return "loss_timer_updated" }
func (e EventLossTimerSet) EventType() string  { return "EventLossTimerSet" }
func (e EventLossTimerSet) IsNil() bool        { return false }

func (e EventLossTimerSet) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("event_type", "set")
	enc.StringKey("timer_type", e.TimerType.String())
	enc.StringKey("packet_number_space", encLevelToPacketNumberSpace(e.EncLevel))
	enc.Float64Key("delta", milliseconds(e.Delta))
}

type EventLossTimerExpired struct {
	TimerType timerType
	EncLevel  protocol.EncryptionLevel
}

func (e EventLossTimerExpired) Category() category { return categoryRecovery }
func (e EventLossTimerExpired) Name() string       { return "loss_timer_updated" }
func (e EventLossTimerExpired) EventType() string  { return "EventLossTimerExpired" }
func (e EventLossTimerExpired) IsNil() bool        { return false }

func (e EventLossTimerExpired) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("event_type", "expired")
	enc.StringKey("timer_type", e.TimerType.String())
	enc.StringKey("packet_number_space", encLevelToPacketNumberSpace(e.EncLevel))
}

type EventLossTimerCanceled struct{}

func (e EventLossTimerCanceled) Category() category { return categoryRecovery }
func (e EventLossTimerCanceled) Name() string       { return "loss_timer_updated" }
func (e EventLossTimerCanceled) EventType() string  { return "EventLossTimerCanceled" }
func (e EventLossTimerCanceled) IsNil() bool        { return false }

func (e EventLossTimerCanceled) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("event_type", "cancelled")
}

type EventCongestionStateUpdated struct {
	state congestionState
}

func (e EventCongestionStateUpdated) Category() category { return categoryRecovery }
func (e EventCongestionStateUpdated) Name() string       { return "congestion_state_updated" }
func (e EventCongestionStateUpdated) EventType() string  { return "EventCongestionStateUpdated" }
func (e EventCongestionStateUpdated) IsNil() bool        { return false }

func (e EventCongestionStateUpdated) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("new", e.state.String())
}

type EventGeneric struct {
	name string
	msg  string
}

func (e EventGeneric) Category() category { return categoryTransport }
func (e EventGeneric) Name() string       { return e.name }
func (e EventGeneric) EventType() string  { return "EventGeneric" }
func (e EventGeneric) IsNil() bool        { return false }

func (e EventGeneric) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("details", e.msg)
}
