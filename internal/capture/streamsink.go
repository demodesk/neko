package capture

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

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
	waitForKf  bool // wait for a keyframe before sending samples

	bitrate  uint64 // atomic
	brDeltas map[int]float64

	logger zerolog.Logger
	mu     sync.Mutex
	wg     sync.WaitGroup

	codec      codec.RTPCodec
	pipeline   gst.Pipeline
	pipelineMu sync.Mutex
	pipelineFn func() (string, error)

	listeners   map[uintptr]types.SampleListener
	listenersKf map[uintptr]types.SampleListener // keyframe lobby
	listenersMu sync.Mutex

	// metrics
	currentListeners prometheus.Gauge
	totalBytes       prometheus.Counter
	pipelinesCounter prometheus.Counter
	pipelinesActive  prometheus.Gauge
}

func streamSinkNew(c codec.RTPCodec, pipelineFn func() (string, error), id string, getBitrate func() (int, error)) *StreamSinkManagerCtx {
	logger := log.With().
		Str("module", "capture").
		Str("submodule", "stream-sink").
		Str("id", id).Logger()

	manager := &StreamSinkManagerCtx{
		id:         id,
		getBitrate: getBitrate,
		// only wait for keyframes if the codec is video
		waitForKf: c.IsVideo(),

		brDeltas: map[int]float64{},

		logger:     logger,
		codec:      c,
		pipelineFn: pipelineFn,

		listeners:   map[uintptr]types.SampleListener{},
		listenersKf: map[uintptr]types.SampleListener{},

		// metrics
		currentListeners: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "streamsink_listeners",
			Namespace: "neko",
			Subsystem: "capture",
			Help:      "Current number of listeners for a pipeline.",
			ConstLabels: map[string]string{
				"video_id":   id,
				"codec_name": c.Name,
				"codec_type": c.Type.String(),
			},
		}),
		totalBytes: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "streamsink_bytes",
			Namespace: "neko",
			Subsystem: "capture",
			Help:      "Total number of bytes created by the pipeline.",
			ConstLabels: map[string]string{
				"video_id":   id,
				"codec_name": c.Name,
				"codec_type": c.Type.String(),
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
				"codec_name": c.Name,
				"codec_type": c.Type.String(),
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
				"codec_name": c.Name,
				"codec_type": c.Type.String(),
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
	for key := range manager.listenersKf {
		delete(manager.listenersKf, key)
	}
	manager.listenersMu.Unlock()

	manager.DestroyPipeline()
	manager.wg.Wait()
}

func (manager *StreamSinkManagerCtx) ID() string {
	return manager.id
}

func (manager *StreamSinkManagerCtx) Bitrate() int {
	// TODO: fix bitrate switching calculation
	// return real bitrate if available
	//realBitrate := atomic.LoadUint64(&manager.bitrate)
	//if realBitrate != 0 {
	//	return int(realBitrate)
	//}

	// if we do not have function to estimate bitrate, return 0
	if manager.getBitrate == nil {
		return 0
	}

	// recalculate bitrate every time, take screen resolution (and fps) into account
	// we called this function during startup, so it shouldn't error here
	bitrate, err := manager.getBitrate()
	if err != nil {
		manager.logger.Err(err).Msg("unexpected error while getting bitrate")
	}

	return bitrate
}

func (manager *StreamSinkManagerCtx) Codec() codec.RTPCodec {
	return manager.codec
}

func (manager *StreamSinkManagerCtx) start() error {
	if len(manager.listeners)+len(manager.listenersKf) == 0 {
		err := manager.CreatePipeline()
		if err != nil && !errors.Is(err, types.ErrCapturePipelineAlreadyExists) {
			return err
		}

		manager.logger.Info().Msgf("first listener, starting")
	}

	return nil
}

func (manager *StreamSinkManagerCtx) stop() {
	if len(manager.listeners)+len(manager.listenersKf) == 0 {
		manager.DestroyPipeline()
		manager.logger.Info().Msgf("last listener, stopping")
	}
}

func (manager *StreamSinkManagerCtx) addListener(listener types.SampleListener) {
	ptr := reflect.ValueOf(listener).Pointer()
	emitKeyframe := false

	manager.listenersMu.Lock()
	if manager.waitForKf {
		// if this is the first listener, we need to emit a keyframe
		emitKeyframe = len(manager.listenersKf) == 0
		// if we're waiting for a keyframe, add it to the keyframe lobby
		manager.listenersKf[ptr] = listener
	} else {
		// otherwise, add it as a regular listener
		manager.listeners[ptr] = listener
	}
	manager.listenersMu.Unlock()

	manager.logger.Debug().Interface("ptr", ptr).Msgf("adding listener")
	manager.currentListeners.Set(float64(manager.ListenersCount()))

	// if we will be waiting for a keyframe, emit one now
	if manager.pipeline != nil && emitKeyframe {
		manager.pipeline.EmitVideoKeyframe()
	}
}

func (manager *StreamSinkManagerCtx) removeListener(listener types.SampleListener) {
	ptr := reflect.ValueOf(listener).Pointer()

	manager.listenersMu.Lock()
	delete(manager.listeners, ptr)
	delete(manager.listenersKf, ptr) //	if it's a keyframe listener, remove it too
	manager.listenersMu.Unlock()

	manager.logger.Debug().Interface("ptr", ptr).Msgf("removing listener")
	manager.currentListeners.Set(float64(manager.ListenersCount()))
}

func (manager *StreamSinkManagerCtx) AddListener(listener types.SampleListener) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if listener == nil {
		return errors.New("listener cannot be nil")
	}

	// start if stopped
	if err := manager.start(); err != nil {
		return err
	}

	// add listener
	manager.addListener(listener)

	return nil
}

func (manager *StreamSinkManagerCtx) RemoveListener(listener types.SampleListener) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if listener == nil {
		return errors.New("listener cannot be nil")
	}

	// remove listener
	manager.removeListener(listener)

	// stop if started
	manager.stop()

	return nil
}

// moving listeners between streams ensures, that target pipeline is running
// before listener is added, and stops source pipeline if there are 0 listeners
func (manager *StreamSinkManagerCtx) MoveListenerTo(listener types.SampleListener, stream types.StreamSinkManager) error {
	if listener == nil {
		return errors.New("listener cannot be nil")
	}

	targetStream, ok := stream.(*StreamSinkManagerCtx)
	if !ok {
		return errors.New("target stream manager does not support moving listeners")
	}

	// we need to acquire both mutextes, from source stream and from target stream
	// in order to do that safely (without possibility of deadlock) we need third
	// global mutex, that ensures atomic locking

	// lock global mutex
	moveSinkListenerMu.Lock()

	// lock source stream
	manager.mu.Lock()
	defer manager.mu.Unlock()

	// lock target stream
	targetStream.mu.Lock()
	defer targetStream.mu.Unlock()

	// unlock global mutex
	moveSinkListenerMu.Unlock()

	// start if stopped
	if err := targetStream.start(); err != nil {
		return err
	}

	// swap listeners
	manager.removeListener(listener)
	targetStream.addListener(listener)

	// stop if started
	manager.stop()

	return nil
}

func (manager *StreamSinkManagerCtx) ListenersCount() int {
	manager.listenersMu.Lock()
	defer manager.listenersMu.Unlock()

	return len(manager.listeners) + len(manager.listenersKf)
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

			manager.onSample(sample)
		}
	}()

	manager.pipelinesCounter.Inc()
	manager.pipelinesActive.Set(1)

	return nil
}

func (manager *StreamSinkManagerCtx) saveBitrateDelta(timestamp time.Time, delta float64) {
	// get unix timestamp in seconds
	sec := timestamp.Unix()
	// last bucket is timestamp rounded to 3 seconds - 1 second
	last := int((sec - 1) % 3)
	// current bucket is timestamp rounded to 3 seconds
	curr := int(sec % 3)
	// next bucket is timestamp rounded to 3 seconds + 1 second
	next := int((sec + 1) % 3)

	if manager.brDeltas[next] != 0 {
		// atomic update bitrate
		atomic.StoreUint64(&manager.bitrate, uint64(manager.brDeltas[last]))
		// empty next bucket
		manager.brDeltas[next] = 0
		// log bitrate
		if manager.codec.IsVideo() {
			manager.logger.Debug().
				Uint64("bitrate", manager.bitrate).
				Msg("bitrate")
		}
	}

	// add rate to current bucket
	manager.brDeltas[curr] += delta
}

func (manager *StreamSinkManagerCtx) onSample(sample types.Sample) {
	manager.listenersMu.Lock()
	defer manager.listenersMu.Unlock()

	// save to metrics
	length := float64(sample.Length) / 8
	manager.totalBytes.Add(length)

	// save bitrate delta
	delta := length / (float64(sample.Duration.Microseconds()) / 1e6)
	manager.saveBitrateDelta(sample.Timestamp, delta)

	// if is not delta unit -> it can be decoded independently -> it is a keyframe
	if manager.waitForKf && !sample.DeltaUnit && len(manager.listenersKf) > 0 {
		// if current sample is a keyframe, move listeners from
		// keyframe lobby to actual listeners map and clear lobby
		for k, v := range manager.listenersKf {
			manager.listeners[k] = v
		}
		manager.listenersKf = make(map[uintptr]types.SampleListener)
	}

	for _, emit := range manager.listeners {
		emit.Sample() <- sample
	}
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

	manager.brDeltas = make(map[int]float64)
	atomic.StoreUint64(&manager.bitrate, 0)
}
