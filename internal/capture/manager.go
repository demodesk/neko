package capture

import (
	"fmt"

	"github.com/kataras/go-events"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"demodesk/neko/internal/types"
	"demodesk/neko/internal/config"
	"demodesk/neko/internal/capture/gst"
)

type CaptureManagerCtx struct {
	logger        zerolog.Logger
	video         *gst.Pipeline
	audio         *gst.Pipeline
	broadcast     *gst.Pipeline
	config        *config.Capture
	audio_stop    chan bool
	video_stop    chan bool
	emmiter       events.EventEmmiter
	streaming     bool
	broadcasting  bool
	broadcast_url string
	desktop       types.DesktopManager
}

func New(desktop types.DesktopManager, config *config.Capture) *CaptureManagerCtx {
	return &CaptureManagerCtx{
		logger:        log.With().Str("module", "capture").Logger(),
		audio_stop:     make(chan bool),
		video_stop:     make(chan bool),
		emmiter:       events.New(),
		config:        config,
		streaming:     false,
		broadcasting:  false,
		broadcast_url: "",
		desktop:       desktop,
	}
}

func (manager *CaptureManagerCtx) Start() {
	manager.logger.Info().
		Str("screen_resolution", fmt.Sprintf("%dx%d@%d", manager.config.ScreenWidth, manager.config.ScreenHeight, manager.config.ScreenRate)).
		Msgf("Setting screen resolution...")

	if err := manager.desktop.ChangeScreenSize(manager.config.ScreenWidth, manager.config.ScreenHeight, manager.config.ScreenRate); err != nil {
		manager.logger.Warn().Err(err).Msg("unable to change screen size")
	}

	manager.StartBroadcastPipeline()
}

func (manager *CaptureManagerCtx) Shutdown() error {
	manager.logger.Info().Msgf("capture shutting down")
	manager.audio_stop <- true
	manager.video_stop <- true
	manager.StopBroadcastPipeline()

	return nil
}

func (manager *CaptureManagerCtx) VideoCodec() string {
	return manager.config.VideoCodec
}

func (manager *CaptureManagerCtx) AudioCodec() string {
	return manager.config.AudioCodec
}

func (manager *CaptureManagerCtx) OnVideoFrame(listener func(sample types.Sample)) {
	manager.emmiter.On("video", func(payload ...interface{}) {
		listener(payload[0].(types.Sample))
	})
}

func (manager *CaptureManagerCtx) OnAudioFrame(listener func(sample types.Sample)) {
	manager.emmiter.On("audio", func(payload ...interface{}) {
		listener(payload[0].(types.Sample))
	})
}

func (manager *CaptureManagerCtx) StartStream() {
	manager.logger.Info().Msgf("Pipelines starting...")

	manager.createVideoPipeline()
	manager.createAudioPipeline()
	manager.streaming = true
}

func (manager *CaptureManagerCtx) StopStream() {
	manager.logger.Info().Msgf("Pipelines stopping...")

	manager.audio_stop <- true
	manager.video_stop <- true
	manager.streaming = false
}

func (manager *CaptureManagerCtx) Streaming() bool {
	return manager.streaming
}

func (manager *CaptureManagerCtx) ChangeResolution(width int, height int, rate int) error {
	manager.video_stop <- true
	manager.StopBroadcastPipeline()

	defer func() {
		manager.createVideoPipeline()
		manager.StartBroadcastPipeline()
	}()
	
	return manager.desktop.ChangeScreenSize(width, height, rate)
}

func (manager *CaptureManagerCtx) createVideoPipeline() {
	var err error

	manager.logger.Info().
		Str("video_codec", manager.config.VideoCodec).
		Str("video_display", manager.config.Display).
		Str("video_params", manager.config.VideoParams).
		Msgf("Creating video pipeline...")

	manager.video, err = gst.CreateAppPipeline(
		manager.config.VideoCodec,
		manager.config.Display,
		manager.config.VideoParams,
	)

	if err != nil {
		manager.logger.Panic().Err(err).Msg("unable to create video pipeline")
	}

	manager.logger.Info().
		Str("pipeline", manager.video.Src).
		Msgf("Starting video pipeline...")

	manager.video.Start()

	go func() {
		manager.logger.Debug().Msg("started emitting video data")

		defer func() {
			manager.logger.Debug().Msg("stopped emitting video data")
		}()

		for {
			select {
			case <-manager.video_stop:
				manager.logger.Info().Msgf("Stopping video pipeline...")
				manager.video.Stop()
				return
			case sample := <-manager.video.Sample:
				manager.emmiter.Emit("video", sample)
			}
		}
	}()
}

func (manager *CaptureManagerCtx) createAudioPipeline() {
	var err error

	manager.logger.Info().
		Str("audio_codec", manager.config.AudioCodec).
		Str("audio_display", manager.config.Device).
		Str("audio_params", manager.config.AudioParams).
		Msgf("Creating audio pipeline...")

	manager.audio, err = gst.CreateAppPipeline(
		manager.config.AudioCodec,
		manager.config.Device,
		manager.config.AudioParams,
	)

	if err != nil {
		manager.logger.Panic().Err(err).Msg("unable to create audio pipeline")
	}

	manager.logger.Info().
		Str("pipeline", manager.audio.Src).
		Msgf("Starting audio pipeline...")

	manager.audio.Start()

	go func() {
		manager.logger.Debug().Msg("started emitting audio data")

		defer func() {
			manager.logger.Debug().Msg("stopped emitting audio data")
		}()

		for {
			select {
			case <-manager.audio_stop:
				manager.logger.Info().Msgf("Stopping audio pipeline...")
				manager.audio.Stop()
				return
			case sample := <-manager.audio.Sample:
				manager.emmiter.Emit("audio", sample)
			}
		}
	}()
}