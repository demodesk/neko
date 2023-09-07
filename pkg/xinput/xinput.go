/* custom xf86 input driver communication protocol */
package xinput

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type driver struct {
	mu     sync.Mutex
	socket string
	conn   net.Conn

	debounceTouchIds map[uint32]time.Time
}

func NewDriver(socket string) Driver {
	return &driver{
		socket: socket,

		debounceTouchIds: make(map[uint32]time.Time),
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

func (d *driver) Debounce(duration time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	t := time.Now()
	for touchId, start := range d.debounceTouchIds {
		if t.Sub(start) < duration {
			continue
		}

		msg := Message{
			_type:   msgTouchEndWithoutPayload,
			touchId: touchId,
		}
		_, _ = d.conn.Write(msg.Pack())
		delete(d.debounceTouchIds, touchId)
	}
}

func (d *driver) TouchBegin(touchId uint32, x, y int, pressure uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.debounceTouchIds[touchId]; ok {
		return fmt.Errorf("debounced touch id %v", touchId)
	}

	d.debounceTouchIds[touchId] = time.Now()

	msg := Message{
		_type:    msgTouchBegin,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) TouchUpdate(touchId uint32, x, y int, pressure uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.debounceTouchIds[touchId]; !ok {
		return fmt.Errorf("unknown touch id %v", touchId)
	}

	d.debounceTouchIds[touchId] = time.Now()

	msg := Message{
		_type:    msgTouchUpdate,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) TouchEnd(touchId uint32, x, y int, pressure uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.debounceTouchIds[touchId]; !ok {
		return fmt.Errorf("unknown touch id %v", touchId)
	}

	delete(d.debounceTouchIds, touchId)

	msg := Message{
		_type:    msgTouchEndWithPayload,
		touchId:  touchId,
		x:        int32(x),
		y:        int32(y),
		pressure: pressure,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) Move(x, y int) error {
	msg := Message{
		_type: msgPointerMotion,
		x:     int32(x),
		y:     int32(y),
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) ButtonDown(button uint32) error {
	msg := Message{
		_type:  msgButtonDown,
		button: button,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) ButtonUp(button uint32) error {
	msg := Message{
		_type:  msgButtonUp,
		button: button,
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}

func (d *driver) Scroll(x, y int) error {
	msg := Message{
		_type: msgScrollMotion,
		x:     int32(x),
		y:     int32(y),
	}
	_, err := d.conn.Write(msg.Pack())
	return err
}
