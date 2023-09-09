package room

import (
	"net/http"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/utils"
)

func (h *RoomHandler) keyboardMapSet(w http.ResponseWriter, r *http.Request) error {
	data := &types.KeyboardMap{}
	if err := utils.HttpJsonRequest(w, r, data); err != nil {
		return err
	}

	err := h.desktop.SetKeyboardMap(*data)
	if err != nil {
		return utils.HttpInternalServerError().WithInternalErr(err)
	}

	return utils.HttpSuccess(w)
}

func (h *RoomHandler) keyboardMapGet(w http.ResponseWriter, r *http.Request) error {
	data, err := h.desktop.GetKeyboardMap()

	if err != nil {
		return utils.HttpInternalServerError().WithInternalErr(err)
	}

	return utils.HttpSuccess(w, data)
}

func (h *RoomHandler) keyboardModifiersSet(w http.ResponseWriter, r *http.Request) error {
	data := &types.KeyboardModifiers{}
	if err := utils.HttpJsonRequest(w, r, data); err != nil {
		return err
	}

	h.desktop.SetKeyboardModifiers(*data)
	return utils.HttpSuccess(w)
}

func (h *RoomHandler) keyboardModifiersGet(w http.ResponseWriter, r *http.Request) error {
	data := h.desktop.GetKeyboardModifiers()
	return utils.HttpSuccess(w, data)
}
