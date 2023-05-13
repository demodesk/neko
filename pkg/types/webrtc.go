package types

import (
	"errors"

	"github.com/pion/webrtc/v3"
)

var (
	ErrWebRTCDataChannelNotFound = errors.New("webrtc data channel not found")
	ErrWebRTCConnectionNotFound  = errors.New("webrtc connection not found")
	ErrWebRTCStreamNotFound      = errors.New("webrtc stream not found")
)

type ICEServer struct {
	URLs       []string `mapstructure:"urls"       json:"urls"`
	Username   string   `mapstructure:"username"   json:"username,omitempty"`
	Credential string   `mapstructure:"credential" json:"credential,omitempty"`
}

type VideoTrack struct {
	ID      string `json:"id"`
	Bitrate uint64 `json:"bitrate"`
}

type WebRTCPeer interface {
	CreateOffer(ICERestart bool) (*webrtc.SessionDescription, error)
	CreateAnswer() (*webrtc.SessionDescription, error)
	SetRemoteDescription(webrtc.SessionDescription) error
	SetCandidate(webrtc.ICECandidateInit) error

	SetVideo(StreamSelector) error
	Video() VideoTrack
	SetPaused(isPaused bool) error
	Paused() bool
	SetVideoAuto(auto bool)
	VideoAuto() bool

	SendCursorPosition(x, y int) error
	SendCursorImage(cur *CursorImage, img []byte) error

	Destroy()
}

type WebRTCManager interface {
	Start()
	Shutdown() error

	ICEServers() []ICEServer

	CreatePeer(session Session) (*webrtc.SessionDescription, error)
	SetCursorPosition(x, y int)
}
