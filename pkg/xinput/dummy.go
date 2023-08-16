package xinput

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

func (d *dummy) TouchBegin(touchId uint32, x, y int, pressure uint16) error {
	return nil
}

func (d *dummy) TouchUpdate(touchId uint32, x, y int, pressure uint16) error {
	return nil
}

func (d *dummy) TouchEnd(touchId uint32, x, y int, pressure uint16) error {
	return nil
}
