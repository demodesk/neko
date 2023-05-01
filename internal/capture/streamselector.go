package capture

import (
	"errors"
	"sort"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/codec"
)

type StreamSelectorManagerCtx struct {
	logger    zerolog.Logger
	codec     codec.RTPCodec
	streams   map[string]types.StreamSinkManager
	streamIDs []string
}

func streamSelectorNew(codec codec.RTPCodec, streams map[string]types.StreamSinkManager, streamIDs []string) *StreamSelectorManagerCtx {
	logger := log.With().
		Str("module", "capture").
		Str("submodule", "stream-selector").
		Logger()

	return &StreamSelectorManagerCtx{
		logger:    logger,
		codec:     codec,
		streams:   streams,
		streamIDs: streamIDs,
	}
}

func (manager *StreamSelectorManagerCtx) shutdown() {
	manager.logger.Info().Msgf("shutdown")

	manager.destroyPipelines()
}

func (manager *StreamSelectorManagerCtx) destroyPipelines() {
	for _, stream := range manager.streams {
		if stream.Started() {
			stream.DestroyPipeline()
		}
	}
}

func (manager *StreamSelectorManagerCtx) recreatePipelines() error {
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

func (manager *StreamSelectorManagerCtx) IDs() []string {
	return manager.streamIDs
}

func (manager *StreamSelectorManagerCtx) Codec() codec.RTPCodec {
	return manager.codec
}

func (manager *StreamSelectorManagerCtx) GetStream(selector types.StreamSelector) (types.StreamSinkManager, bool) {
	// select stream by ID
	if selector.ID != "" {
		stream, ok := manager.streams[selector.ID]
		return stream, ok
	}

	// select stream by bitrate
	if selector.Bitrate != 0 {
		// select stream by nearest bitrate
		if selector.BitrateNearest {
			return manager.nearestBitrate(selector.Bitrate), true
		}

		// select stream by exact bitrate
		for _, stream := range manager.streams {
			if stream.Bitrate() == selector.Bitrate {
				return stream, true
			}
		}
	}

	// we couldn't find a stream
	return nil, false
}

func (manager *StreamSelectorManagerCtx) nearestBitrate(bitrate uint64) types.StreamSinkManager {
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
		// TODO: If we don't have a bitrate yet, we can't compare it.
		bitrate := stream.Bitrate()
		if bitrate == 0 {
			continue
		}
		diffs = append(diffs, streamDiff{
			id:          stream.ID(),
			bitrateDiff: int(bitrate) - int(stream.Bitrate()),
		})
	}

	// no streams available
	if len(diffs) == 0 {
		// return first stream
		return manager.streams[manager.streamIDs[0]]
	}

	sort.Slice(diffs, func(i, j int) bool {
		return sortDiff(diffs[i].bitrateDiff, diffs[j].bitrateDiff)
	})

	bestDiff := diffs[0]
	return manager.streams[bestDiff.id]
}
