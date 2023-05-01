package webrtc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog"

	"github.com/demodesk/neko/internal/webrtc/payload"
	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/event"
	"github.com/demodesk/neko/pkg/types/message"
)

const (
	// how often to read and process bandwidth estimation reports
	estimatorReadInterval = 250 * time.Millisecond
)

type WebRTCPeerCtx struct {
	mu         sync.Mutex
	logger     zerolog.Logger
	session    types.Session
	metrics    *metrics
	connection *webrtc.PeerConnection
	// bandwidth estimator
	estimator      cc.BandwidthEstimator
	bitrateHistory queue
	// stream selectors
	videoSelector types.StreamSelectorManager
	// tracks & channels
	audioTrack  *Track
	videoTrack  *Track
	dataChannel *webrtc.DataChannel
	rtcpChannel chan []rtcp.Packet
	// config
	iceTrickle       bool
	estimatorPassive bool
	videoAuto        bool
}

//
// connection
//

func (peer *WebRTCPeerCtx) CreateOffer(ICERestart bool) (*webrtc.SessionDescription, error) {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	offer, err := peer.connection.CreateOffer(&webrtc.OfferOptions{
		ICERestart: ICERestart,
	})
	if err != nil {
		return nil, err
	}

	return peer.setLocalDescription(offer)
}

func (peer *WebRTCPeerCtx) CreateAnswer() (*webrtc.SessionDescription, error) {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	answer, err := peer.connection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	return peer.setLocalDescription(answer)
}

func (peer *WebRTCPeerCtx) setLocalDescription(description webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if !peer.iceTrickle {
		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete := webrtc.GatheringCompletePromise(peer.connection)

		if err := peer.connection.SetLocalDescription(description); err != nil {
			return nil, err
		}

		<-gatherComplete
	} else {
		if err := peer.connection.SetLocalDescription(description); err != nil {
			return nil, err
		}
	}

	return peer.connection.LocalDescription(), nil
}

func (peer *WebRTCPeerCtx) SetRemoteDescription(desc webrtc.SessionDescription) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	return peer.connection.SetRemoteDescription(desc)
}

func (peer *WebRTCPeerCtx) SetCandidate(candidate webrtc.ICECandidateInit) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	return peer.connection.AddICECandidate(candidate)
}

// TODO: Add shutdown function?
func (peer *WebRTCPeerCtx) Destroy() {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	err := peer.connection.Close()
	peer.logger.Err(err).Msg("peer connection destroyed")
}

func (peer *WebRTCPeerCtx) estimatorReader() {
	// if estimator is disabled, do nothing
	if peer.estimator == nil {
		return
	}

	// use a ticker to get current client target bitrate
	ticker := time.NewTicker(estimatorReadInterval)
	defer ticker.Stop()

	for range ticker.C {
		targetBitrate := peer.estimator.GetTargetBitrate()
		peer.metrics.SetReceiverEstimatedTargetBitrate(float64(targetBitrate))

		if peer.connection.ConnectionState() == webrtc.PeerConnectionStateClosed {
			break
		}

		if !peer.videoAuto || peer.estimatorPassive {
			continue
		}

		bitrate := peer.bitrateHistory.normaliseBitrate(targetBitrate)

		peer.logger.Debug().
			Int("target_bitrate", targetBitrate).
			Int("normalized_bitrate", bitrate).
			Msg("got bitrate from estimator")

		err := peer.SetVideo(types.StreamSelector{
			Bitrate:        uint64(bitrate),
			BitrateNearest: true,
		})
		if err != nil {
			peer.logger.Warn().Err(err).Msg("failed to set video bitrate")
		}
	}
}

//
// video
//

func (peer *WebRTCPeerCtx) SetVideo(selector types.StreamSelector) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	// get requested video stream from selector
	stream, ok := peer.videoSelector.GetStream(selector)
	if !ok {
		return fmt.Errorf("stream not found")
	}

	// set video stream to track
	changed, err := peer.videoTrack.SetStream(stream)
	if err != nil {
		return err
	}

	// if video stream was already set, do nothing
	if !changed {
		return nil
	}

	videoID := peer.videoTrack.stream.ID()
	bitrate := peer.videoTrack.stream.Bitrate()

	peer.metrics.SetVideoID(videoID)
	peer.logger.Debug().
		Uint64("video_bitrate", bitrate).
		Str("video_id", videoID).
		Msg("triggered video stream change")

	go peer.session.Send(
		event.SIGNAL_VIDEO,
		// TODO: Refactor.
		message.SignalVideo{
			Video:     videoID,        // TODO: Refactor.
			Bitrate:   int(bitrate),   // TODO: Refactor.
			VideoAuto: peer.videoAuto, // TODO: Refactor.
		})

	return nil
}

func (peer *WebRTCPeerCtx) Video() types.VideoTrack {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	// TODO: Refactor.
	stream, ok := peer.videoTrack.Stream()
	if !ok {
		// TODO: Refactor.
		return types.VideoTrack{}
	}

	// TODO: Refactor.
	return types.VideoTrack{
		ID:      stream.ID(),
		Bitrate: stream.Bitrate(),
	}
}

func (peer *WebRTCPeerCtx) SetPaused(isPaused bool) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	peer.logger.Info().Bool("is_paused", isPaused).Msg("set paused")
	peer.videoTrack.SetPaused(isPaused)
	peer.audioTrack.SetPaused(isPaused)
	return nil
}

func (peer *WebRTCPeerCtx) Paused() bool {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	return peer.videoTrack.Paused() || peer.audioTrack.Paused()
}

func (peer *WebRTCPeerCtx) SetVideoAuto(videoAuto bool) {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	// if estimator is enabled and is not passive, enable video auto bitrate
	if peer.estimator != nil && !peer.estimatorPassive {
		peer.logger.Info().Bool("video_auto", videoAuto).Msg("set video auto")
		peer.videoAuto = videoAuto
	} else {
		peer.logger.Warn().Msg("estimator is disabled or in passive mode, cannot change video auto")
		peer.videoAuto = false // ensure video auto is disabled
	}
}

func (peer *WebRTCPeerCtx) VideoAuto() bool {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	return peer.videoAuto
}

//
// data channel
//

func (peer *WebRTCPeerCtx) SendCursorPosition(x, y int) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	// do not send cursor position to host
	if peer.session.IsHost() {
		return nil
	}

	header := payload.Header{
		Event:  payload.OP_CURSOR_POSITION,
		Length: 7,
	}

	data := payload.CursorPosition{
		X: uint16(x),
		Y: uint16(y),
	}

	buffer := &bytes.Buffer{}

	if err := binary.Write(buffer, binary.BigEndian, header); err != nil {
		return err
	}

	if err := binary.Write(buffer, binary.BigEndian, data); err != nil {
		return err
	}

	return peer.dataChannel.Send(buffer.Bytes())
}

func (peer *WebRTCPeerCtx) SendCursorImage(cur *types.CursorImage, img []byte) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()

	header := payload.Header{
		Event:  payload.OP_CURSOR_IMAGE,
		Length: uint16(11 + len(img)),
	}

	data := payload.CursorImage{
		Width:  cur.Width,
		Height: cur.Height,
		Xhot:   cur.Xhot,
		Yhot:   cur.Yhot,
	}

	buffer := &bytes.Buffer{}

	if err := binary.Write(buffer, binary.BigEndian, header); err != nil {
		return err
	}

	if err := binary.Write(buffer, binary.BigEndian, data); err != nil {
		return err
	}

	if err := binary.Write(buffer, binary.BigEndian, img); err != nil {
		return err
	}

	return peer.dataChannel.Send(buffer.Bytes())
}
