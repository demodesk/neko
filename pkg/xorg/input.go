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
	_type    int
	touchId  int
	x        int
	y        int
	pressure int
}

func (msg *InputDriverMessage) Unpack(buffer []byte) {
	msg._type = int(buffer[0])
	msg.touchId = int(buffer[1])
	msg.x = int(buffer[2])<<8 | int(buffer[3])
	msg.y = int(buffer[4])<<8 | int(buffer[5])
	msg.pressure = int(buffer[6])<<8 | int(buffer[7])
}

func (msg *InputDriverMessage) Pack() []byte {
	var buffer [8]byte

	buffer[0] = byte(msg._type)
	buffer[1] = byte(msg.touchId)
	buffer[2] = byte(msg.x >> 8)
	buffer[3] = byte(msg.x & 0xFF)
	buffer[4] = byte(msg.y >> 8)
	buffer[5] = byte(msg.y & 0xFF)
	buffer[6] = byte(msg.pressure >> 8)
	buffer[7] = byte(msg.pressure & 0xFF)

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

func (d *InputDriver) SendTouchBegin(touchId int, x int, y int, pressure int) error {
	msg := InputDriverMessage{
		_type:    XI_TouchBegin,
		touchId:  touchId,
		x:        x,
		y:        y,
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *InputDriver) SendTouchUpdate(touchId int, x int, y int, pressure int) error {
	msg := InputDriverMessage{
		_type:    XI_TouchUpdate,
		touchId:  touchId,
		x:        x,
		y:        y,
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *InputDriver) SendTouchEnd(touchId int, x int, y int, pressure int) error {
	msg := InputDriverMessage{
		_type:    XI_TouchEnd,
		touchId:  touchId,
		x:        x,
		y:        y,
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}
