package handler

import (
	"demodesk/neko/internal/types"
	"demodesk/neko/internal/types/event"
	"demodesk/neko/internal/types/message"
)

func (h *MessageHandlerCtx) screenSet(session types.Session, payload *message.ScreenResolution) error {
	if !session.Admin() {
		h.logger.Debug().Msg("user not admin")
		return nil
	}

	if err := h.capture.ChangeResolution(payload.Width, payload.Height, payload.Rate); err != nil {
		h.logger.Warn().Err(err).Msgf("unable to change screen size")
		return err
	}

	if err := h.sessions.Broadcast(message.ScreenResolution{
		Event:  event.SCREEN_RESOLUTION,
		ID:     session.ID(),
		Width:  payload.Width,
		Height: payload.Height,
		Rate:   payload.Rate,
	}, nil); err != nil {
		h.logger.Warn().Err(err).Msgf("sending event %s has failed", event.SCREEN_RESOLUTION)
		return err
	}

	return nil
}

func (h *MessageHandlerCtx) screenResolution(session types.Session) error {
	size := h.desktop.GetScreenSize()
	if size == nil {
		h.logger.Debug().Msg("could not get screen size")
		return nil
	}

	if err := session.Send(message.ScreenResolution{
		Event:  event.SCREEN_RESOLUTION,
		Width:  size.Width,
		Height: size.Height,
		Rate:   int(size.Rate),
	}); err != nil {
		h.logger.Warn().Err(err).Msgf("sending event %s has failed", event.SCREEN_RESOLUTION)
		return err
	}

	return nil
}

func (h *MessageHandlerCtx) screenConfigurations(session types.Session) error {
	if !session.Admin() {
		h.logger.Debug().Msg("user not admin")
		return nil
	}

	if err := session.Send(message.ScreenConfigurations{
		Event:          event.SCREEN_CONFIGURATIONS,
		Configurations: h.desktop.ScreenConfigurations(),
	}); err != nil {
		h.logger.Warn().Err(err).Msgf("sending event %s has failed", event.SCREEN_CONFIGURATIONS)
		return err
	}

	return nil
}