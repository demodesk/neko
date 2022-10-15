package capture

import (
	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/codec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type BucketsManagerCtx struct {
	logger    zerolog.Logger
	codec     codec.RTPCodec
	streams   map[string]*StreamSinkManagerCtx
	streamIDs []string
}

func bucketsNew(codec codec.RTPCodec, streams map[string]*StreamSinkManagerCtx, streamIDs []string) *BucketsManagerCtx {
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

func (m *BucketsManagerCtx) shutdown() {
	m.logger.Info().Msgf("shutdown")
}

func (m *BucketsManagerCtx) destroyAll() {
	// destroy all pipelines
}

func (m *BucketsManagerCtx) recreateAll() error {
	// start all pipelines
	return nil
}

func (m *BucketsManagerCtx) IDs() []string {
	return m.streamIDs
}

func (m *BucketsManagerCtx) Codec() codec.RTPCodec {
	return m.codec
}

func (m *BucketsManagerCtx) SetReceiver(receiver types.Track) error {
	// TODO: Save receiver.
	return receiver.SetStream(m.streams[m.streamIDs[0]])
}

func (m *BucketsManagerCtx) RemoveReceiver(receiver types.Track) error {
	// TODO: Remove receiver.
	receiver.RemoveStream()
	return nil
}
