package desktop

import "github.com/demodesk/neko/pkg/xinput"

func (manager *DesktopManagerCtx) inputRelToAbs(x, y int) (int, int) {
	return (x * xinput.AbsX) / manager.screenSize.Width, (y * xinput.AbsY) / manager.screenSize.Height
}

func (manager *DesktopManagerCtx) TouchBegin(touchId uint32, x, y int, pressure uint8) error {
	mu.Lock()
	defer mu.Unlock()

	x, y = manager.inputRelToAbs(x, y)
	return manager.input.TouchBegin(touchId, x, y, pressure)
}

func (manager *DesktopManagerCtx) TouchUpdate(touchId uint32, x, y int, pressure uint8) error {
	mu.Lock()
	defer mu.Unlock()

	x, y = manager.inputRelToAbs(x, y)
	return manager.input.TouchUpdate(touchId, x, y, pressure)
}

func (manager *DesktopManagerCtx) TouchEnd(touchId uint32, x, y int, pressure uint8) error {
	mu.Lock()
	defer mu.Unlock()

	x, y = manager.inputRelToAbs(x, y)
	return manager.input.TouchEnd(touchId, x, y, pressure)
}
