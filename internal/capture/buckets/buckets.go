package buckets

import (
	"errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/codec"
)

type BucketsManagerCtx struct {
	logger    zerolog.Logger
	codec     codec.RTPCodec
	streams   map[string]types.StreamSinkManager
	streamIDs []string
}

func BucketsNew(codec codec.RTPCodec, streams map[string]types.StreamSinkManager, streamIDs []string) *BucketsManagerCtx {
	logger := log.With().
		Str("module", "capture").
		Str("submodule", "buckets").
		Logger()

	return &BucketsManagerCtx{
		logger:    logger,
		codec:     codec,
		streams:   streams,
		streamIDs: streamIDs,
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
		stream := m.FindNearestStream(bitrate)
		ok, err := receiver.SetStream(stream)
		return ok, err
	})
}

func (m *BucketsManagerCtx) FindNearestStream(peerBitrate int) types.StreamSinkManager {
	type streamDiff struct {
		id          string
		bitrateDiff int
	}

	sortDiff := func(a, b int) bool {
		switch {
		case a < 0 && b < 0:
			return a > b
		case a > 0 && b > 0:
			return a < b
		default:
			return a > b
		}
	}

	var lowestDiff *streamDiff

	for _, stream := range m.streams {
		streamBitrate, err := stream.Bitrate()
		if err != nil {
			m.logger.Fatal().Err(err).Str("id", stream.ID()).Msg("failed to get stream bitrate")
		}

		currentDiff := peerBitrate - streamBitrate

		if lowestDiff == nil {
			lowestDiff = &streamDiff{stream.ID(), currentDiff}
			continue
		}

		if sortDiff(currentDiff, lowestDiff.bitrateDiff) {
			lowestDiff = &streamDiff{stream.ID(), currentDiff}
		}
	}

	return m.streams[lowestDiff.id]
}

func (m *BucketsManagerCtx) RemoveReceiver(receiver types.Receiver) error {
	receiver.OnBitrateChange(nil)
	receiver.RemoveStream()
	return nil
}
