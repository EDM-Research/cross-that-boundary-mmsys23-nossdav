package qlog

// category is the qlog event category.
type category uint8

const (
	categoryPlayback category = iota
	categoryABR
	categoryBuffer
	categoryNetwork
	categoryGeneric
)

func (c category) String() string {
	switch c {
	case categoryPlayback:
		return "playback"
	case categoryABR:
		return "abr"
	case categoryBuffer:
		return "buffer"
	case categoryNetwork:
		return "network"
	case categoryGeneric:
		return "generic"
	default:
		return "unknown category"
	}
}

type MediaType uint8

const (
	MediaTypeVideo MediaType = iota
	MediaTypeAudio
	MediaTypeSubtitles
	MediaTypeOther
)

func (c MediaType) String() string {
	switch c {
	case MediaTypeVideo:
		return "video"
	case MediaTypeAudio:
		return "audio"
	case MediaTypeSubtitles:
		return "subtitles"
	case MediaTypeOther:
		return "other"
	default:
		return "unknown media type"
	}
}

type InteractionState uint8

const (
	InteractionStatePlay InteractionState = iota
	InteractionStatePause
	InteractionStateSeek
	InteractionStateSpeed
)

func (c InteractionState) String() string {
	switch c {
	case InteractionStatePlay:
		return "play"
	case InteractionStatePause:
		return "pause"
	case InteractionStateSeek:
		return "seek"
	case InteractionStateSpeed:
		return "speed"
	default:
		return "unknown interaction state"
	}
}

type ReadyState uint8

const (
	ReadyStateHaveNothing ReadyState = iota
	ReadyStateHaveMetadata
	ReadyStateHaveCurrentData
	ReadyStateHaveFutureData
	ReadyStateHaveEnoughData
)

func (c ReadyState) String() string {
	switch c {
	case ReadyStateHaveNothing:
		return "have nothing"
	case ReadyStateHaveMetadata:
		return "have metadata"
	case ReadyStateHaveCurrentData:
		return "have current data"
	case ReadyStateHaveFutureData:
		return "have future data"
	case ReadyStateHaveEnoughData:
		return "have enough data"
	default:
		return "unknown readystate"
	}
}
