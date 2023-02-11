package desktop

import (
	"image"
	"os/exec"
	"regexp"
	"time"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/xorg"
)

func (manager *DesktopManagerCtx) Move(x, y int) {
	xorg.Move(x, y)
}

func (manager *DesktopManagerCtx) GetCursorPosition() (int, int) {
	return xorg.GetCursorPosition()
}

func (manager *DesktopManagerCtx) Scroll(x, y int) {
	xorg.Scroll(x, y)
}

func (manager *DesktopManagerCtx) ButtonDown(code uint32) error {
	return xorg.ButtonDown(code)
}

func (manager *DesktopManagerCtx) KeyDown(code uint32) error {
	return xorg.KeyDown(code)
}

func (manager *DesktopManagerCtx) ButtonUp(code uint32) error {
	return xorg.ButtonUp(code)
}

func (manager *DesktopManagerCtx) KeyUp(code uint32) error {
	return xorg.KeyUp(code)
}

func (manager *DesktopManagerCtx) ButtonPress(code uint32) error {
	xorg.ResetKeys()
	defer xorg.ResetKeys()

	return xorg.ButtonDown(code)
}

func (manager *DesktopManagerCtx) KeyPress(codes ...uint32) error {
	xorg.ResetKeys()
	defer xorg.ResetKeys()

	for _, code := range codes {
		if err := xorg.KeyDown(code); err != nil {
			return err
		}
	}

	if len(codes) > 1 {
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

func (manager *DesktopManagerCtx) ResetKeys() {
	xorg.ResetKeys()
}

// TODO: Remove this function.
func (manager *DesktopManagerCtx) ScreenConfigurations() map[int]types.ScreenConfiguration {
	return map[int]types.ScreenConfiguration{}
}

func (manager *DesktopManagerCtx) SetScreenSize(size types.ScreenSize) (types.ScreenSize, error) {
	mu.Lock()
	manager.emmiter.Emit("before_screen_size_change")

	defer func() {
		manager.emmiter.Emit("after_screen_size_change")
		mu.Unlock()
	}()

	w, h, r, err := xorg.ChangeScreenSize(size.Width, size.Height, size.Rate)
	return types.ScreenSize{
		Width:  w,
		Height: h,
		Rate:   r,
	}, err
}

func (manager *DesktopManagerCtx) GetScreenSize() *types.ScreenSize {
	return xorg.GetScreenSize()
}

func (manager *DesktopManagerCtx) SetKeyboardMap(kbd types.KeyboardMap) error {
	// TOOD: Use native API.
	cmd := exec.Command("setxkbmap", "-layout", kbd.Layout, "-variant", kbd.Variant)
	_, err := cmd.Output()
	return err
}

func (manager *DesktopManagerCtx) GetKeyboardMap() (*types.KeyboardMap, error) {
	// TOOD: Use native API.
	cmd := exec.Command("setxkbmap", "-query")
	res, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	kbd := types.KeyboardMap{}

	re := regexp.MustCompile(`layout:\s+(.*)\n`)
	arr := re.FindStringSubmatch(string(res))
	if len(arr) > 1 {
		kbd.Layout = arr[1]
	}

	re = regexp.MustCompile(`variant:\s+(.*)\n`)
	arr = re.FindStringSubmatch(string(res))
	if len(arr) > 1 {
		kbd.Variant = arr[1]
	}

	return &kbd, nil
}

func (manager *DesktopManagerCtx) SetKeyboardModifiers(mod types.KeyboardModifiers) {
	if mod.NumLock != nil {
		xorg.SetKeyboardModifier(xorg.KbdModNumLock, *mod.NumLock)
	}

	if mod.CapsLock != nil {
		xorg.SetKeyboardModifier(xorg.KbdModCapsLock, *mod.CapsLock)
	}
}

func (manager *DesktopManagerCtx) GetKeyboardModifiers() types.KeyboardModifiers {
	modifiers := xorg.GetKeyboardModifiers()

	NumLock := (modifiers & xorg.KbdModNumLock) != 0
	CapsLock := (modifiers & xorg.KbdModCapsLock) != 0

	return types.KeyboardModifiers{
		NumLock:  &NumLock,
		CapsLock: &CapsLock,
	}
}

func (manager *DesktopManagerCtx) GetCursorImage() *types.CursorImage {
	return xorg.GetCursorImage()
}

func (manager *DesktopManagerCtx) GetScreenshotImage() *image.RGBA {
	return xorg.GetScreenshotImage()
}
