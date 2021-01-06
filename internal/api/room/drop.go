package room

import (
	"os"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"net/http"

	"demodesk/neko/internal/utils"
)

const (
	// Maximum upload of 32 MB files.
	MAX_UPLOAD_SIZE = 32 << 20
)

func (h *RoomHandler) dropFiles(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(MAX_UPLOAD_SIZE)

	X, err := strconv.Atoi(r.FormValue("x"))
	if err != nil {
		utils.HttpBadRequest(w, err)
		return
	}

	Y, err := strconv.Atoi(r.FormValue("y"))
	if err != nil {
		utils.HttpBadRequest(w, err)
		return
	}

	req_files := r.MultipartForm.File["files"]
	if len(req_files) == 0 {
		utils.HttpBadRequest(w, "No file received.")
		return
	}

	dir, err := ioutil.TempDir("", "neko-drop-*")
	if err != nil {
		utils.HttpInternalServerError(w, err)
		return
	}

	files := []string{}
	for _, req_file := range req_files {
		path := path.Join(dir, req_file.Filename)

		srcFile, err := req_file.Open()
		if err != nil {
			utils.HttpInternalServerError(w, err)
			return
		}

		defer srcFile.Close()

		dstFile, err := os.OpenFile(path, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644)
		if err != nil {
			utils.HttpInternalServerError(w, err)
			return
		}

		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			utils.HttpInternalServerError(w, err)
			return
		}

		files = append(files, path)
	}

	h.desktop.DropFiles(X, Y, files)
	utils.HttpSuccess(w)
}
