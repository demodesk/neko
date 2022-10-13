package webrtc

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog"

	"github.com/demodesk/neko/internal/webrtc/payload"
	"github.com/demodesk/neko/pkg/types"
)

type Peer struct {
	mu          sync.Mutex
	logger      zerolog.Logger
	connection  *webrtc.PeerConnection
	dataChannel *webrtc.DataChannel
	changeVideo func(videoID string) error
	setPaused   func(isPaused bool)
	iceTrickle  bool
}

func (p *Peer) CreateOffer(ICERestart bool) (*webrtc.SessionDescription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return nil, types.ErrWebRTCConnectionNotFound
	}

	offer, err := p.connection.CreateOffer(&webrtc.OfferOptions{
		ICERestart: ICERestart,
	})
	if err != nil {
		return nil, err
	}

	return p.setLocalDescription(offer)
}

func (p *Peer) CreateAnswer() (*webrtc.SessionDescription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return nil, types.ErrWebRTCConnectionNotFound
	}

	answer, err := p.connection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	return p.setLocalDescription(answer)
}

func (p *Peer) setLocalDescription(description webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if !p.iceTrickle {
		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete := webrtc.GatheringCompletePromise(p.connection)

		if err := p.connection.SetLocalDescription(description); err != nil {
			return nil, err
		}

		<-gatherComplete
	} else {
		if err := p.connection.SetLocalDescription(description); err != nil {
			return nil, err
		}
	}

	return p.connection.LocalDescription(), nil
}

func (p *Peer) SetOffer(sdp string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return types.ErrWebRTCConnectionNotFound
	}

	return p.connection.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  sdp,
		Type: webrtc.SDPTypeOffer,
	})
}

func (p *Peer) SetAnswer(sdp string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return types.ErrWebRTCConnectionNotFound
	}

	return p.connection.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  sdp,
		Type: webrtc.SDPTypeAnswer,
	})
}

func (p *Peer) SetCandidate(candidate webrtc.ICECandidateInit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return types.ErrWebRTCConnectionNotFound
	}

	return p.connection.AddICECandidate(candidate)
}

func (p *Peer) SetVideoID(videoID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return types.ErrWebRTCConnectionNotFound
	}

	p.logger.Info().Str("video_id", videoID).Msg("change video id")
	return p.changeVideo(videoID)
}

func (p *Peer) SetPaused(isPaused bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection == nil {
		return types.ErrWebRTCConnectionNotFound
	}

	p.logger.Info().Bool("is_paused", isPaused).Msg("set paused")
	p.setPaused(isPaused)
	return nil
}

func (p *Peer) SendCursorPosition(x, y int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dataChannel == nil {
		return types.ErrWebRTCDataChannelNotFound
	}

	data := payload.CursorPosition{
		Header: payload.Header{
			Event:  payload.OP_CURSOR_POSITION,
			Length: 7,
		},
		X: uint16(x),
		Y: uint16(y),
	}

	buffer := &bytes.Buffer{}
	if err := binary.Write(buffer, binary.BigEndian, data); err != nil {
		return err
	}

	return p.dataChannel.Send(buffer.Bytes())
}

func (p *Peer) SendCursorImage(cur *types.CursorImage, img []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dataChannel == nil {
		return types.ErrWebRTCDataChannelNotFound
	}

	data := payload.CursorImage{
		Header: payload.Header{
			Event:  payload.OP_CURSOR_IMAGE,
			Length: uint16(11 + len(img)),
		},
		Width:  cur.Width,
		Height: cur.Height,
		Xhot:   cur.Xhot,
		Yhot:   cur.Yhot,
	}

	buffer := &bytes.Buffer{}

	if err := binary.Write(buffer, binary.BigEndian, data); err != nil {
		return err
	}

	if err := binary.Write(buffer, binary.BigEndian, img); err != nil {
		return err
	}

	return p.dataChannel.Send(buffer.Bytes())
}

func (p *Peer) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connection != nil {
		err := p.connection.Close()
		p.logger.Err(err).Msg("peer connection destroyed")
		p.connection = nil
	}
}
