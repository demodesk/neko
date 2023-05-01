package handler

import (
	"errors"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/event"
	"github.com/demodesk/neko/pkg/types/message"
	"github.com/pion/webrtc/v3"
)

func (h *MessageHandlerCtx) signalRequest(session types.Session, payload *message.SignalVideo) error {
	if !session.Profile().CanWatch {
		return errors.New("not allowed to watch")
	}

	// use default first video, if no video or bitrate is specified
	if payload.Video == "" && payload.Bitrate == 0 {
		videos := h.capture.Video().IDs()
		payload.Video = videos[0]
	}

	offer, err := h.webrtc.CreatePeer(session)
	if err != nil {
		return err
	}

	if peer := session.GetWebRTCPeer(); peer != nil {
		// set webrtc as paused if session has private mode enabled
		if session.PrivateModeEnabled() {
			peer.SetPaused(true)
		}

		// TODO: Set this when connecion is created?
		peer.SetVideoAuto(payload.VideoAuto)

		// TODO: Refactor
		err := peer.SetVideo(types.StreamSelector{
			ID:             payload.Video,
			Bitrate:        uint64(payload.Bitrate),
			BitrateNearest: true,
		})
		if err != nil {
			return err
		}

		// TODO: Refactor
		payload.Video = peer.Video().ID
		payload.VideoAuto = peer.VideoAuto()
	}

	session.Send(
		event.SIGNAL_PROVIDE,
		message.SignalProvide{
			SDP:        offer.SDP,
			ICEServers: h.webrtc.ICEServers(),
			// TODO: Refactor
			Video:     payload.Video,
			Bitrate:   payload.Bitrate,
			VideoAuto: payload.VideoAuto,
		})

	return nil
}

func (h *MessageHandlerCtx) signalRestart(session types.Session) error {
	peer := session.GetWebRTCPeer()
	if peer == nil {
		return errors.New("webRTC peer does not exist")
	}

	offer, err := peer.CreateOffer(true)
	if err != nil {
		return err
	}

	// TODO: Use offer event instead.
	session.Send(
		event.SIGNAL_RESTART,
		message.SignalDescription{
			SDP: offer.SDP,
		})

	return nil
}

func (h *MessageHandlerCtx) signalOffer(session types.Session, payload *message.SignalDescription) error {
	peer := session.GetWebRTCPeer()
	if peer == nil {
		return errors.New("webRTC peer does not exist")
	}

	err := peer.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  payload.SDP,
		Type: webrtc.SDPTypeOffer,
	})
	if err != nil {
		return err
	}

	answer, err := peer.CreateAnswer()
	if err != nil {
		return err
	}

	session.Send(
		event.SIGNAL_ANSWER,
		message.SignalDescription{
			SDP: answer.SDP,
		})

	return nil
}

func (h *MessageHandlerCtx) signalAnswer(session types.Session, payload *message.SignalDescription) error {
	peer := session.GetWebRTCPeer()
	if peer == nil {
		return errors.New("webRTC peer does not exist")
	}

	return peer.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  payload.SDP,
		Type: webrtc.SDPTypeAnswer,
	})
}

func (h *MessageHandlerCtx) signalCandidate(session types.Session, payload *message.SignalCandidate) error {
	peer := session.GetWebRTCPeer()
	if peer == nil {
		return errors.New("webRTC peer does not exist")
	}

	return peer.SetCandidate(payload.ICECandidateInit)
}

func (h *MessageHandlerCtx) signalVideo(session types.Session, payload *message.SignalVideo) error {
	peer := session.GetWebRTCPeer()
	if peer == nil {
		return errors.New("webRTC peer does not exist")
	}

	peer.SetVideoAuto(payload.VideoAuto)

	// TODO: Refactor
	return peer.SetVideo(types.StreamSelector{
		ID:             payload.Video,
		Bitrate:        uint64(payload.Bitrate),
		BitrateNearest: true,
	})
}
