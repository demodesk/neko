package buckets

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/codec"
)

type BucketsManagerCtx struct {
	logger         zerolog.Logger
	codec          codec.RTPCodec
	streams        map[string]types.StreamSinkManager
	streamIDs      []string
	streamAuto     bool
	bitrateHistory *queue
	sync.Mutex
}

func BucketsNew(codec codec.RTPCodec, streams map[string]types.StreamSinkManager, streamIDs []string) *BucketsManagerCtx {
	logger := log.With().
		Str("module", "capture").
		Str("submodule", "buckets").
		Logger()

	return &BucketsManagerCtx{
		logger:         logger,
		codec:          codec,
		streams:        streams,
		streamIDs:      streamIDs,
		bitrateHistory: &queue{},
		streamAuto:     true,
	}
}

func (m *BucketsManagerCtx) Shutdown() {
	m.logger.Info().Msgf("shutdown")
}

func (m *BucketsManagerCtx) DestroyAll() {
	for _, stream := range m.streams {
		if stream.Started() {
			stream.DestroyPipeline()
		}
	}
}

func (m *BucketsManagerCtx) RecreateAll() error {
	for _, stream := range m.streams {
		if stream.Started() {
			err := stream.CreatePipeline()
			if err != nil && !errors.Is(err, types.ErrCapturePipelineAlreadyExists) {
				return err
			}
		}
	}

	return nil
}

func (m *BucketsManagerCtx) IDs() []string {
	return m.streamIDs
}

func (m *BucketsManagerCtx) Codec() codec.RTPCodec {
	return m.codec
}

func (m *BucketsManagerCtx) SetReceiver(receiver types.Receiver) {
	receiver.OnBitrateChange(func(bitrate int) (bool, error) {
		if !m.streamAuto {
			return false, nil
		}

		m.bitrateHistory.push(elem{
			bitrate: bitrate,
			created: time.Now(),
		})

		stream := m.findNearestStream(m.bitrateHistory.avg())
		ok, err := receiver.SetStream(stream)
		return ok, err
	})

	receiver.OnVideoChange(func(videoID string) (bool, error) {
		stream := m.streams[videoID]
		m.logger.Info().Msgf("video change: %s", videoID)
		return receiver.SetStream(stream)
	})
}

func (m *BucketsManagerCtx) findNearestStream(peerBitrate int) types.StreamSinkManager {
	type streamDiff struct {
		id          string
		bitrateDiff int
	}

	sortDiff := func(a, b int) bool {
		switch {
		case a < 0 && b < 0:
			return a > b
		case a >= 0:
			if b >= 0 {
				return a <= b
			}
			return true
		}
		return false
	}

	var diffs []streamDiff

	for _, stream := range m.streams {
		streamBitrate, err := stream.Bitrate()
		if err != nil {
			m.logger.Fatal().Err(err).Str("id", stream.ID()).Msg("failed to get stream bitrate")
		}

		currentDiff := peerBitrate - streamBitrate

		diffs = append(diffs, streamDiff{
			id:          stream.ID(),
			bitrateDiff: currentDiff,
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		return sortDiff(diffs[i].bitrateDiff, diffs[j].bitrateDiff)
	})

	bestDiff := diffs[0]

	return m.streams[bestDiff.id]
}

func (m *BucketsManagerCtx) RemoveReceiver(receiver types.Receiver) error {
	receiver.OnBitrateChange(nil)
	receiver.RemoveStream()
	return nil
}

func (m *BucketsManagerCtx) SetVideoAuto(auto bool) {
	m.Lock()
	defer m.Unlock()
	m.streamAuto = auto
}

func (m *BucketsManagerCtx) VideoAuto() bool {
	m.Lock()
	defer m.Unlock()
	return m.streamAuto
}
