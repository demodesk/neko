package buckets

import (
	"reflect"
	"testing"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/types/codec"
	"github.com/rs/zerolog"
)

func TestBucketsManagerCtx_FindNearestStream(t *testing.T) {
	type fields struct {
		logger  zerolog.Logger
		codec   codec.RTPCodec
		streams map[string]types.StreamSinkManager
	}
	type args struct {
		peerBitrate int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   types.StreamSinkManager
	}{
		{
			name: "findNearestStream",
			fields: fields{
				streams: map[string]types.StreamSinkManager{
					"1": mockStreamSink{
						id:      "1",
						bitrate: 500,
					},
					"2": mockStreamSink{
						id:      "2",
						bitrate: 750,
					},
					"3": mockStreamSink{
						id:      "3",
						bitrate: 1000,
					},
					"4": mockStreamSink{
						id:      "4",
						bitrate: 1250,
					},
					"5": mockStreamSink{
						id:      "5",
						bitrate: 1700,
					},
				},
			},
			args: args{
				peerBitrate: 950,
			},
			want: mockStreamSink{
				id:      "2",
				bitrate: 750,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := BucketsNew(tt.fields.codec, tt.fields.streams, []string{})

			if got := m.findNearestStream(tt.args.peerBitrate); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findNearestStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockStreamSink struct {
	id      string
	bitrate int
	types.StreamSinkManager
}

func (m mockStreamSink) ID() string {
	return m.id
}

func (m mockStreamSink) Bitrate() int {
	return m.bitrate
}

func TestBucketsManagerCtx_normaliseBitrate(t *testing.T) {
	type fields struct {
		bitrateHistory *queue
	}
	type args struct {
		currentBitrate int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []int
	}{
		{
			name: "normaliseBitrate: big drop",
			fields: fields{
				bitrateHistory: &queue{
					q: []elem{
						{bitrate: 900},
						{bitrate: 750},
						{bitrate: 780},
						{bitrate: 1100},
						{bitrate: 950},
						{bitrate: 700},
						{bitrate: 800},
						{bitrate: 900},
						{bitrate: 1000},
						{bitrate: 1100},
						// avg = 898
					},
				},
			},
			args: args{
				currentBitrate: 350,
			},
			want: []int{816, 700, 537, 350, 350},
		}, {
			name: "normaliseBitrate: small drop",
			fields: fields{
				bitrateHistory: &queue{
					q: []elem{
						{bitrate: 900},
						{bitrate: 750},
						{bitrate: 780},
						{bitrate: 1100},
						{bitrate: 950},
						{bitrate: 700},
						{bitrate: 800},
						{bitrate: 900},
						{bitrate: 1000},
						{bitrate: 1100},
						// avg = 898
					},
				},
			},
			args: args{
				currentBitrate: 700,
			},
			want: []int{878, 842, 825, 825, 812, 787, 750, 700},
		}, {
			name: "normaliseBitrate",
			fields: fields{
				bitrateHistory: &queue{
					q: []elem{
						{bitrate: 900},
						{bitrate: 750},
						{bitrate: 780},
						{bitrate: 1100},
						{bitrate: 950},
						{bitrate: 700},
						{bitrate: 800},
						{bitrate: 900},
						{bitrate: 1000},
						{bitrate: 1100},
						// avg = 898
					},
				},
			},
			args: args{
				currentBitrate: 1350,
			},
			want: []int{943, 1003, 1060, 1085},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &BucketsManagerCtx{
				bitrateHistory: tt.fields.bitrateHistory,
			}

			for i := 0; i < len(tt.want); i++ {
				if got := m.normaliseBitrate(tt.args.currentBitrate); got != tt.want[i] {
					t.Errorf("normaliseBitrate() [%d] = %v, want %v", i, got, tt.want[i])
				}
			}
		})
	}
}
