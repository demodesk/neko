//go:build libclipboard
// +build libclipboard

package desktop

import (
	"fmt"

	"github.com/demodesk/neko/pkg/clipboard"
	"github.com/demodesk/neko/pkg/types"
)

func (manager *DesktopManagerCtx) ClipboardGetText() (*types.ClipboardText, error) {
	text := clipboard.Read()
	return &types.ClipboardText{
		Text: text,
	}, nil
}

func (manager *DesktopManagerCtx) ClipboardSetText(data types.ClipboardText) error {
	clipboard.Write(data.Text)
	return nil
}

func (manager *DesktopManagerCtx) ClipboardGetBinary(mime string) ([]byte, error) {
	return nil, fmt.Errorf("not supported by libclipboard")
}

func (manager *DesktopManagerCtx) ClipboardSetBinary(mime string, data []byte) error {
	return fmt.Errorf("not supported by libclipboard")
}

func (manager *DesktopManagerCtx) ClipboardGetTargets() ([]string, error) {
	// libclipboard does not support multiple targets.
	return []string{"STRING"}, nil
}
