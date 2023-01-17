package payload

const (
	OP_CURSOR_POSITION = 0x01
	OP_CURSOR_IMAGE    = 0x02
	OP_PONG            = 0x03
)

type CursorPosition struct {
	Header

	X uint16
	Y uint16
}

type CursorImage struct {
	Header

	Width  uint16
	Height uint16
	Xhot   uint16
	Yhot   uint16
}

type Pong struct {
	Ping

	// server's timestamp split into two uint32
	ServerTs1 uint32
	ServerTs2 uint32
}
