package buckets

import (
	"errors"
	"math"
	"sort"
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
	bitrateHistory *queue
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
	}
}

func (manager *BucketsManagerCtx) Shutdown() {
	manager.logger.Info().Msgf("shutdown")

	manager.DestroyAll()
}

func (manager *BucketsManagerCtx) DestroyAll() {
	for _, stream := range manager.streams {
		if stream.Started() {
			stream.DestroyPipeline()
		}
	}
}

func (manager *BucketsManagerCtx) RecreateAll() error {
	for _, stream := range manager.streams {
		if stream.Started() {
			err := stream.CreatePipeline()
			if err != nil && !errors.Is(err, types.ErrCapturePipelineAlreadyExists) {
				return err
			}
		}
	}
	return nil
}

func (manager *BucketsManagerCtx) IDs() []string {
	return manager.streamIDs
}

func (manager *BucketsManagerCtx) Codec() codec.RTPCodec {
	return manager.codec
}

func (manager *BucketsManagerCtx) SetReceiver(receiver types.Receiver) {
	receiver.OnBitrateChange(func(peerBitrate int) (bool, error) {
		bitrate := peerBitrate
		if receiver.VideoAuto() {
			bitrate = manager.normaliseBitrate(bitrate)
		}

		stream := manager.findNearestStream(bitrate)
		streamID := stream.ID()

		// TODO: make this less noisy in logs
		manager.logger.Debug().
			Str("video_id", streamID).
			Int("len", manager.bitrateHistory.len()).
			Int("peer_bitrate", peerBitrate).
			Int("bitrate", bitrate).
			Msg("change video bitrate")

		return receiver.SetStream(stream)
	})

	receiver.OnVideoChange(func(videoID string) (bool, error) {
		stream := manager.streams[videoID]
		manager.logger.Info().
			Str("video_id", videoID).
			Msg("video change")

		return receiver.SetStream(stream)
	})
}

func (manager *BucketsManagerCtx) normaliseBitrate(currentBitrate int) int {
	avgBitrate := float64(manager.bitrateHistory.avg())
	histLen := float64(manager.bitrateHistory.len())

	manager.bitrateHistory.push(elem{
		bitrate: currentBitrate,
		created: time.Now(),
	})

	if avgBitrate == 0 || histLen == 0 || currentBitrate == 0 {
		return currentBitrate
	}

	lastN := int(math.Floor(float64(currentBitrate) / avgBitrate * histLen))
	if lastN > manager.bitrateHistory.len() {
		lastN = manager.bitrateHistory.len()
	}

	if lastN == 0 {
		return currentBitrate
	}

	return manager.bitrateHistory.avgLastN(lastN)
}

func (manager *BucketsManagerCtx) findNearestStream(peerBitrate int) types.StreamSinkManager {
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

	for _, stream := range manager.streams {
		diffs = append(diffs, streamDiff{
			id:          stream.ID(),
			bitrateDiff: peerBitrate - stream.Bitrate(),
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		return sortDiff(diffs[i].bitrateDiff, diffs[j].bitrateDiff)
	})

	bestDiff := diffs[0]

	return manager.streams[bestDiff.id]
}

func (manager *BucketsManagerCtx) RemoveReceiver(receiver types.Receiver) error {
	receiver.OnBitrateChange(nil)
	receiver.OnVideoChange(nil)
	receiver.RemoveStream()
	return nil
}
