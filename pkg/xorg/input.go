package xorg

import (
	"net"
)

const (
	XI_TouchBegin  = 18
	XI_TouchUpdate = 19
	XI_TouchEnd    = 20
)

type InputDriverMessage struct {
	_type    uint16
	touchId  uint32
	x        int32 // can be negative?
	y        int32 // can be negative?
	pressure uint16
}

func (msg *InputDriverMessage) Unpack(buffer []byte) {
	msg._type = uint16(buffer[0])
	msg.touchId = uint32(buffer[1]) | (uint32(buffer[2]) << 8)
	msg.x = int32(buffer[3]) | (int32(buffer[4]) << 8) | (int32(buffer[5]) << 16) | (int32(buffer[6]) << 24)
	msg.y = int32(buffer[7]) | (int32(buffer[8]) << 8) | (int32(buffer[9]) << 16) | (int32(buffer[10]) << 24)
	msg.pressure = uint16(buffer[11])
}

func (msg *InputDriverMessage) Pack() []byte {
	var buffer [12]byte

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

	return buffer[:]
}

type InputDriver struct {
	socket string
	conn   net.Conn
}

func NewInputDriver(socket string) *InputDriver {
	return &InputDriver{
		socket: socket,
	}
}

func (d *InputDriver) Connect() error {
	c, err := net.Dial("unix", d.socket)
	if err != nil {
		return err
	}
	d.conn = c
	return nil
}

func (d *InputDriver) Close() error {
	return d.conn.Close()
}

func (d *InputDriver) SendTouchBegin(touchId uint32, x, y int, pressure uint16) error {
	msg := InputDriverMessage{
		_type:    XI_TouchBegin,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *InputDriver) SendTouchUpdate(touchId uint32, x, y int, pressure uint16) error {
	msg := InputDriverMessage{
		_type:    XI_TouchUpdate,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *InputDriver) SendTouchEnd(touchId uint32, x, y int, pressure uint16) error {
	msg := InputDriverMessage{
		_type:    XI_TouchEnd,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}
