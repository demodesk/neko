package xinput

import "time"

const (
	// absolute coordinates used in driver
	AbsX = 0xffff
	AbsY = 0xffff
)

const (
	msgTouchBegin             = 1
	msgTouchUpdate            = 2
	msgTouchEndWithPayload    = 3
	msgTouchEndWithoutPayload = 4
	msgPointerMotion          = 5
	msgButtonDown             = 6
	msgButtonUp               = 7
	msgScrollMotion           = 8
)

type Message struct {
	_type    uint16
	touchId  uint32
	x        int32
	y        int32
	pressure uint8
	button   uint32
}

func (msg *Message) Unpack(buffer []byte) {
	msg._type = uint16(buffer[0])
	msg.touchId = uint32(buffer[1]) | (uint32(buffer[2]) << 8)
	msg.x = int32(buffer[3]) | (int32(buffer[4]) << 8) | (int32(buffer[5]) << 16) | (int32(buffer[6]) << 24)
	msg.y = int32(buffer[7]) | (int32(buffer[8]) << 8) | (int32(buffer[9]) << 16) | (int32(buffer[10]) << 24)
	msg.pressure = uint8(buffer[11])
	msg.button = uint32(buffer[12]) | (uint32(buffer[13]) << 8) | (uint32(buffer[14]) << 16) | (uint32(buffer[15]) << 24)
}

func (msg *Message) Pack() []byte {
	var buffer [16]byte

	buffer[0] = byte(msg._type)
	buffer[1] = byte(msg.touchId)
	buffer[2] = byte(msg.touchId >> 8)
	buffer[3] = byte(msg.x)
	buffer[4] = byte(msg.x >> 8)
	buffer[5] = byte(msg.x >> 16)
	buffer[6] = byte(msg.x >> 24)
	buffer[7] = byte(msg.y)
	buffer[8] = byte(msg.y >> 8)
	buffer[9] = byte(msg.y >> 16)
	buffer[10] = byte(msg.y >> 24)
	buffer[11] = byte(msg.pressure)
	buffer[12] = byte(msg.button)
	buffer[13] = byte(msg.button >> 8)
	buffer[14] = byte(msg.button >> 16)
	buffer[15] = byte(msg.button >> 24)

	return buffer[:]
}

type Driver interface {
	Connect() error
	Close() error
	// release touches, that were not updated for duration
	Debounce(duration time.Duration)
	// touch events
	TouchBegin(touchId uint32, x, y int, pressure uint8) error
	TouchUpdate(touchId uint32, x, y int, pressure uint8) error
	TouchEnd(touchId uint32, x, y int, pressure uint8) error
	// mouse events
	Move(x, y int) error
	ButtonDown(button uint32) error
	ButtonUp(button uint32) error
	Scroll(x, y int) error
}
