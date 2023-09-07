package xinput

import "time"

type dummy struct{}

func NewDummy() Driver {
	return &dummy{}
}

func (d *dummy) Connect() error {
	return nil
}

func (d *dummy) Close() error {
	return nil
}

func (d *dummy) Debounce(duration time.Duration) {}

func (d *dummy) TouchBegin(touchId uint32, x, y int, pressure uint8) error {
	return nil
}

func (d *dummy) TouchUpdate(touchId uint32, x, y int, pressure uint8) error {
	return nil
}

func (d *dummy) TouchEnd(touchId uint32, x, y int, pressure uint8) error {
	return nil
}

func (d *dummy) Move(x, y int) error {
	return nil
}

func (d *dummy) ButtonDown(button uint32) error {
	return nil
}

func (d *dummy) ButtonUp(button uint32) error {
	return nil
}

func (d *dummy) Scroll(x, y int) error {
	return nil
}
