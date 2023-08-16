/* custom xf86 input driver communication protocol */
package xinput

import (
	"net"
)

type driver struct {
	socket string
	conn   net.Conn
}

func NewDriver(socket string) Driver {
	return &driver{
		socket: socket,
	}
}

func (d *driver) Connect() error {
	c, err := net.Dial("unix", d.socket)
	if err != nil {
		return err
	}
	d.conn = c
	return nil
}

func (d *driver) Close() error {
	return d.conn.Close()
}

func (d *driver) TouchBegin(touchId uint32, x, y int, pressure uint16) error {
	msg := Message{
		_type:    XI_TouchBegin,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) TouchUpdate(touchId uint32, x, y int, pressure uint16) error {
	msg := Message{
		_type:    XI_TouchUpdate,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) TouchEnd(touchId uint32, x, y int, pressure uint16) error {
	msg := Message{
		_type:    XI_TouchEnd,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}
