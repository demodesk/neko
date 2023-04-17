package session

import (
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/event"
)

// client is expected to reconnect within 5 second
// if some unexpected websocket disconnect happens
const WS_DELAYED_DURATION = 5 * time.Second

type SessionCtx struct {
	id      string
	token   string
	logger  zerolog.Logger
	manager *SessionManagerCtx
	profile types.MemberProfile
	state   types.SessionState

	websocketPeer types.WebSocketPeer
	websocketMu   sync.Mutex

	// websocket delayed set connected events
	wsDelayedMu    sync.Mutex
	wsDelayedTimer *time.Timer

	webrtcPeer types.WebRTCPeer
	webrtcMu   sync.Mutex
}

func (session *SessionCtx) ID() string {
	return session.id
}

func (session *SessionCtx) Profile() types.MemberProfile {
	return session.profile
}

func (session *SessionCtx) profileChanged() {
	if !session.profile.CanHost && session.IsHost() {
		session.manager.ClearHost()
	}

	if (!session.profile.CanConnect || !session.profile.CanLogin || !session.profile.CanWatch) && session.state.IsWatching {
		session.GetWebRTCPeer().Destroy()
	}

	if (!session.profile.CanConnect || !session.profile.CanLogin) && session.state.IsConnected {
		session.GetWebSocketPeer().Destroy("profile changed")
	}

	// update webrtc paused state
	if webrtcPeer := session.GetWebRTCPeer(); webrtcPeer != nil {
		webrtcPeer.SetPaused(session.PrivateModeEnabled())
	}
}

func (session *SessionCtx) State() types.SessionState {
	return session.state
}

func (session *SessionCtx) IsHost() bool {
	return session.manager.isHost(session)
}

func (session *SessionCtx) PrivateModeEnabled() bool {
	return session.manager.Settings().PrivateMode && !session.profile.IsAdmin
}

func (session *SessionCtx) SetCursor(cursor types.Cursor) {
	if session.manager.Settings().InactiveCursors && session.profile.SendsInactiveCursor {
		session.manager.SetCursor(cursor, session)
	}
}

// ---
// websocket
// ---

//
// Connect WebSocket peer sets current peer and emits connected event. It also destroys the
// previous peer, if there was one. If the peer is already set, it will be ignored.
//
func (session *SessionCtx) ConnectWebSocketPeer(websocketPeer types.WebSocketPeer) {
	session.websocketMu.Lock()
	isCurrentPeer := websocketPeer == session.websocketPeer
	session.websocketPeer, websocketPeer = websocketPeer, session.websocketPeer
	session.websocketMu.Unlock()

	// ignore if already set
	if isCurrentPeer {
		return
	}

	session.logger.Info().Msg("set websocket connected")
	session.state.IsConnected = true
	session.manager.emmiter.Emit("connected", session)

	// if there is a previous peer, destroy it
	if websocketPeer != nil {
		websocketPeer.Destroy("connection replaced")
	}
}

//
// Disconnect WebSocket peer sets current peer to nil and emits disconnected event. It also
// allows for a delayed disconnect. That means, the peer will not be disconnected immediately,
// but after a delay. If the peer is connected again before the delay, the disconnect will be
// cancelled.
//
// If the peer is not the current peer or the peer is nil, it will be ignored.
//
func (session *SessionCtx) DisconnectWebSocketPeer(websocketPeer types.WebSocketPeer, delayed bool) {
	session.websocketMu.Lock()
	isCurrentPeer := websocketPeer == session.websocketPeer && websocketPeer != nil
	session.websocketMu.Unlock()

	// ignore if not current peer
	if !isCurrentPeer {
		return
	}

	//
	// ws delayed
	//

	var wsDelayedTimer *time.Timer

	if delayed {
		wsDelayedTimer = time.AfterFunc(WS_DELAYED_DURATION, func() {
			session.DisconnectWebSocketPeer(websocketPeer, false)
		})
	}

	session.wsDelayedMu.Lock()
	if session.wsDelayedTimer != nil {
		session.wsDelayedTimer.Stop()
	}
	session.wsDelayedTimer = wsDelayedTimer
	session.wsDelayedMu.Unlock()

	if delayed {
		session.logger.Info().Msg("delayed websocket disconnected")
		return
	}

	//
	// not delayed
	//

	session.logger.Info().Msg("set websocket disconnected")
	session.state.IsConnected = false
	session.manager.emmiter.Emit("disconnected", session)

	session.websocketMu.Lock()
	if websocketPeer == session.websocketPeer {
		session.websocketPeer = nil
	}
	session.websocketMu.Unlock()
}

//
// Get current WebRTC peer. Nil if not connected.
//
func (session *SessionCtx) GetWebSocketPeer() types.WebSocketPeer {
	session.websocketMu.Lock()
	defer session.websocketMu.Unlock()

	return session.websocketPeer
}

//
// Send event to websocket peer.
//
func (session *SessionCtx) Send(event string, payload any) {
	peer := session.GetWebSocketPeer()
	if peer != nil {
		peer.Send(event, payload)
	}
}

// ---
// webrtc
// ---

//
// Set webrtc peer and destroy the old one, if there is old one.
//
func (session *SessionCtx) SetWebRTCPeer(webrtcPeer types.WebRTCPeer) {
	session.webrtcMu.Lock()
	session.webrtcPeer, webrtcPeer = webrtcPeer, session.webrtcPeer
	session.webrtcMu.Unlock()

	if webrtcPeer != nil && webrtcPeer != session.webrtcPeer {
		webrtcPeer.Destroy()
	}
}

//
// Set if current webrtc peer is connected or not. Since there might be lefover calls from
// webrtc peer, that are not used anymore, we need to check if the webrtc peer is still the
// same as the one we are setting the connected state for.
//
// If webrtc peer is disconnected, we don't expect it to be reconnected, so we set it to nil
// and send a signal close to the client. New connection is expected to use a new webrtc peer.
//
func (session *SessionCtx) SetWebRTCConnected(webrtcPeer types.WebRTCPeer, connected bool) {
	session.webrtcMu.Lock()
	isCurrentPeer := webrtcPeer == session.webrtcPeer
	session.webrtcMu.Unlock()

	if !isCurrentPeer {
		return
	}

	session.logger.Info().
		Bool("connected", connected).
		Msg("set webrtc connected")

	session.state.IsWatching = connected
	session.manager.emmiter.Emit("state_changed", session)

	if connected {
		return
	}

	session.webrtcMu.Lock()
	isCurrentPeer = webrtcPeer == session.webrtcPeer
	if isCurrentPeer {
		session.webrtcPeer = nil
	}
	session.webrtcMu.Unlock()

	if isCurrentPeer {
		session.Send(event.SIGNAL_CLOSE, nil)
	}
}

//
// Get current WebRTC peer. Nil if not connected.
//
func (session *SessionCtx) GetWebRTCPeer() types.WebRTCPeer {
	session.webrtcMu.Lock()
	defer session.webrtcMu.Unlock()

	return session.webrtcPeer
}
