package webrtc

import (
	"errors"
	"io"
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/rs/zerolog"

	"github.com/demodesk/neko/pkg/types"
)

type Track struct {
	logger   zerolog.Logger
	track    *webrtc.TrackLocalStaticSample
	paused   bool
	listener func(sample types.Sample)

	stream   types.StreamSinkManager
	streamMu sync.Mutex

	onRtcp   func(rtcp.Packet)
	onRtcpMu sync.RWMutex
}

func NewTrack(stream types.StreamSinkManager, logger zerolog.Logger) (*Track, error) {
	codec := stream.Codec()

	id := codec.Type.String()
	track, err := webrtc.NewTrackLocalStaticSample(codec.Capability, id, "stream")
	if err != nil {
		return nil, err
	}

	logger = logger.With().Str("id", id).Logger()

	t := &Track{
		logger: logger,
		track:  track,
	}

	t.listener = func(sample types.Sample) {
		if t.paused {
			return
		}

		err := track.WriteSample(media.Sample(sample))
		if err != nil && errors.Is(err, io.ErrClosedPipe) {
			logger.Warn().Err(err).Msg("pipeline failed to write")
		}
	}

	err = t.SetStream(stream)
	return t, err
}

func (t *Track) SetStream(stream types.StreamSinkManager) error {
	t.streamMu.Lock()
	defer t.streamMu.Unlock()

	var err error
	if t.stream != nil {
		err = t.stream.MoveListenerTo(&t.listener, stream)
	} else {
		err = stream.AddListener(&t.listener)
	}

	if err == nil {
		t.stream = stream
	}

	return err
}

func (t *Track) RemoveStream() {
	t.streamMu.Lock()
	defer t.streamMu.Unlock()

	if t.stream != nil {
		_ = t.stream.RemoveListener(&t.listener)
		t.stream = nil
	}
}

func (t *Track) AddToConnection(connection *webrtc.PeerConnection) error {
	sender, err := connection.AddTrack(t.track)
	if err != nil {
		return err
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			n, _, err := sender.Read(rtcpBuf)
			if err != nil {
				if err == io.EOF || err == io.ErrClosedPipe {
					return
				}

				t.logger.Err(err).Msg("RTCP read error")
				continue
			}

			packets, err := rtcp.Unmarshal(rtcpBuf[:n])
			if err != nil {
				t.logger.Err(err).Msg("RTCP unmarshal error")
				continue
			}

			t.onRtcpMu.RLock()
			handler := t.onRtcp
			t.onRtcpMu.RUnlock()

			for _, packet := range packets {
				if handler != nil {
					go handler(packet)
				}
			}
		}
	}()

	return nil
}

func (t *Track) SetPaused(paused bool) {
	t.paused = paused
}

func (t *Track) OnRTCP(f func(rtcp.Packet)) {
	t.onRtcpMu.Lock()
	defer t.onRtcpMu.Unlock()

	t.onRtcp = f
}
