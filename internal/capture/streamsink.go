package capture

import (
	"errors"
	"reflect"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/demodesk/neko/pkg/gst"
	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/codec"
)

var moveSinkListenerMu = sync.Mutex{}

type StreamSinkManagerCtx struct {
	id         string
	getBitrate func() (int, error)

	logger zerolog.Logger
	mu     sync.Mutex
	wg     sync.WaitGroup

	codec      codec.RTPCodec
	pipeline   gst.Pipeline
	pipelineMu sync.Mutex
	pipelineFn func() (string, error)

	listeners   map[uintptr]*func(sample types.Sample)
	listenersMu sync.Mutex

	// metrics
	currentListeners prometheus.Gauge
	pipelinesCounter prometheus.Counter
	pipelinesActive  prometheus.Gauge
}

func streamSinkNew(codec codec.RTPCodec, pipelineFn func() (string, error), id string, getBitrate func() (int, error)) *StreamSinkManagerCtx {
	logger := log.With().
		Str("module", "capture").
		Str("submodule", "stream-sink").
		Str("id", id).Logger()

	manager := &StreamSinkManagerCtx{
		id:         id,
		getBitrate: getBitrate,

		logger:     logger,
		codec:      codec,
		pipelineFn: pipelineFn,
		listeners:  map[uintptr]*func(sample types.Sample){},

		// metrics
		currentListeners: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "streamsink_listeners",
			Namespace: "neko",
			Subsystem: "capture",
			Help:      "Current number of listeners for a pipeline.",
			ConstLabels: map[string]string{
				"video_id":   id,
				"codec_name": codec.Name,
				"codec_type": codec.Type.String(),
			},
		}),
		pipelinesCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "pipelines_total",
			Namespace: "neko",
			Subsystem: "capture",
			Help:      "Total number of created pipelines.",
			ConstLabels: map[string]string{
				"submodule":  "streamsink",
				"video_id":   id,
				"codec_name": codec.Name,
				"codec_type": codec.Type.String(),
			},
		}),
		pipelinesActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "pipelines_active",
			Namespace: "neko",
			Subsystem: "capture",
			Help:      "Total number of active pipelines.",
			ConstLabels: map[string]string{
				"submodule":  "streamsink",
				"video_id":   id,
				"codec_name": codec.Name,
				"codec_type": codec.Type.String(),
			},
		}),
	}

	return manager
}

func (manager *StreamSinkManagerCtx) shutdown() {
	manager.logger.Info().Msgf("shutdown")

	manager.listenersMu.Lock()
	for key := range manager.listeners {
		delete(manager.listeners, key)
	}
	manager.listenersMu.Unlock()

	manager.DestroyPipeline()
	manager.wg.Wait()
}

func (manager *StreamSinkManagerCtx) ID() string {
	return manager.id
}

func (manager *StreamSinkManagerCtx) Bitrate() (int, error) {
	if manager.getBitrate == nil {
		return 0, nil
	}
	// recalculate bitrate every time, take screen resolution (and fps) into account
	return manager.getBitrate()
}

func (manager *StreamSinkManagerCtx) Codec() codec.RTPCodec {
	return manager.codec
}

func (manager *StreamSinkManagerCtx) Start() error {
	if len(manager.listeners) == 0 {
		err := manager.CreatePipeline()
		if err != nil && !errors.Is(err, types.ErrCapturePipelineAlreadyExists) {
			return err
		}

		manager.logger.Info().Msgf("first listener, starting")
	}

	return nil
}

func (manager *StreamSinkManagerCtx) stop() {
	if len(manager.listeners) == 0 {
		manager.DestroyPipeline()
		manager.logger.Info().Msgf("last listener, stopping")
	}
}

func (manager *StreamSinkManagerCtx) SetListener(listener *func(sample types.Sample)) {
	manager.setListener(listener)
}

func (manager *StreamSinkManagerCtx) setListener(listener *func(sample types.Sample)) {
	ptr := reflect.ValueOf(listener).Pointer()

	manager.listenersMu.Lock()
	manager.listeners[ptr] = listener
	manager.listenersMu.Unlock()

	manager.logger.Debug().Interface("ptr", ptr).Msgf("adding listener")
	manager.currentListeners.Set(float64(manager.ListenersCount()))
}

func (manager *StreamSinkManagerCtx) UnsetListener(listener *func(sample types.Sample)) {
	manager.unsetListener(listener)
}

func (manager *StreamSinkManagerCtx) unsetListener(listener *func(sample types.Sample)) {
	ptr := reflect.ValueOf(listener).Pointer()

	manager.listenersMu.Lock()
	delete(manager.listeners, ptr)
	manager.listenersMu.Unlock()

	manager.logger.Debug().Interface("ptr", ptr).Msgf("removing listener")
	manager.currentListeners.Set(float64(manager.ListenersCount()))
}

func (manager *StreamSinkManagerCtx) AddListener(listener *func(sample types.Sample)) error {
	manager.Lock()
	defer manager.Unlock()

	if listener == nil {
		return errors.New("listener cannot be nil")
	}

	// start if stopped
	if err := manager.Start(); err != nil {
		return err
	}

	// add listener
	manager.setListener(listener)

	return nil
}

func (manager *StreamSinkManagerCtx) RemoveListener(listener *func(sample types.Sample)) error {
	manager.Lock()
	defer manager.Unlock()

	if listener == nil {
		return errors.New("listener cannot be nil")
	}

	// remove listener
	manager.unsetListener(listener)

	// stop if started
	manager.stop()

	return nil
}

// moving listeners between streams ensures, that target pipeline is running
// before listener is added, and stops source pipeline if there are 0 listeners
func (manager *StreamSinkManagerCtx) MoveListenerTo(listener *func(sample types.Sample), targetStream types.StreamSinkManager) error {
	if listener == nil {
		return errors.New("listener cannot be nil")
	}

	// we need to acquire both mutextes, from source stream and from target stream
	// in order to do that safely (without possibility of deadlock) we need third
	// global mutex, that ensures atomic locking

	// lock global mutex
	moveSinkListenerMu.Lock()
	// lock source stream
	manager.Lock()
	// lock target stream
	targetStream.Lock()

	defer func() {
		// unlock target stream
		targetStream.Unlock()
		// unlock source stream
		manager.Unlock()
		// unlock global mutex
		moveSinkListenerMu.Unlock()
	}()

	// start if stopped
	if err := targetStream.Start(); err != nil {
		return err
	}

	// swap listeners
	manager.unsetListener(listener)
	targetStream.SetListener(listener)

	// stop if started
	manager.stop()

	return nil
}

func (manager *StreamSinkManagerCtx) ListenersCount() int {
	manager.listenersMu.Lock()
	defer manager.listenersMu.Unlock()

	return len(manager.listeners)
}

func (manager *StreamSinkManagerCtx) Started() bool {
	return manager.ListenersCount() > 0
}

func (manager *StreamSinkManagerCtx) CreatePipeline() error {
	manager.pipelineMu.Lock()
	defer manager.pipelineMu.Unlock()

	if manager.pipeline != nil {
		return types.ErrCapturePipelineAlreadyExists
	}

	pipelineStr, err := manager.pipelineFn()
	if err != nil {
		return err
	}

	manager.logger.Info().
		Str("codec", manager.codec.Name).
		Str("src", pipelineStr).
		Msgf("creating pipeline")

	manager.pipeline, err = gst.CreatePipeline(pipelineStr)
	if err != nil {
		return err
	}

	manager.pipeline.AttachAppsink("appsink")
	manager.pipeline.Play()

	manager.wg.Add(1)
	pipeline := manager.pipeline

	go func() {
		manager.logger.Debug().Msg("started emitting samples")
		defer manager.wg.Done()

		for {
			sample, ok := <-pipeline.Sample()
			if !ok {
				manager.logger.Debug().Msg("stopped emitting samples")
				return
			}

			manager.listenersMu.Lock()
			for _, emit := range manager.listeners {
				(*emit)(sample)
			}
			manager.listenersMu.Unlock()
		}
	}()

	manager.pipelinesCounter.Inc()
	manager.pipelinesActive.Set(1)

	return nil
}

func (manager *StreamSinkManagerCtx) DestroyPipeline() {
	manager.pipelineMu.Lock()
	defer manager.pipelineMu.Unlock()

	if manager.pipeline == nil {
		return
	}

	manager.pipeline.Destroy()
	manager.logger.Info().Msgf("destroying pipeline")
	manager.pipeline = nil

	manager.pipelinesActive.Set(0)
}

func (manager *StreamSinkManagerCtx) Lock() {
	manager.mu.Lock()
}

func (manager *StreamSinkManagerCtx) Unlock() {
	manager.mu.Unlock()
}
