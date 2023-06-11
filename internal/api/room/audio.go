package room

import (
	"encoding/binary"
	"net/http"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/utils"
)

type audioListener struct {
	sample chan types.Sample
}

func (l *audioListener) WriteSample(sample types.Sample) {
	l.sample <- sample
}

func (h *RoomHandler) audioStream(w http.ResponseWriter, r *http.Request) error {
	audio := h.capture.Audiocast()

	sample := make(chan types.Sample)
	audioListener := &audioListener{sample: sample}
	audio.AddListener(audioListener)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return utils.HttpUnprocessableEntity("streaming is not supported by the client")
	}

	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	for {
		select {
		case sample := <-sample:
			binary.Write(w, binary.BigEndian, &sample.Data)
			flusher.Flush() // Trigger "chunked" encoding
		case <-r.Context().Done():
			audio.RemoveListener(audioListener)
			return nil
		}
	}
}
