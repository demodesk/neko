package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/demodesk/neko/pkg/types"
	"github.com/demodesk/neko/pkg/utils"
)

type BatchRequest struct {
	Path   string          `json:"path"`
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

type BatchResponse struct {
	Path   string          `json:"path"`
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
	Status int             `json:"status"`
}

func (b *BatchResponse) Error(httpErr *utils.HTTPError) error {
	b.Status = httpErr.Code
	httpErr.Code = 0

	data, err := json.Marshal(httpErr)
	if err != nil {
		return err
	}

	b.Body = data
	return nil
}

type batchHandler struct {
	Router     types.Router
	PathPrefix string
	Excluded   []string
}

func (b *batchHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	// parse BatchRequests
	var requests []BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		return err
	}

	// execute BatchRequests
	responses := make([]BatchResponse, len(requests))
	for i, request := range requests {
		res := BatchResponse{
			Path:   request.Path,
			Method: request.Method,
		}

		if !strings.HasPrefix(request.Path, b.PathPrefix) {
			res.Error(utils.HttpBadRequest("this path is not allowed in batch requests"))
			responses[i] = res
			continue
		}

		if exists, _ := utils.ArrayIn(request.Path, b.Excluded); exists {
			res.Error(utils.HttpBadRequest("this path is excluded from batch requests"))
			responses[i] = res
			continue
		}

		// do request
		req, err := http.NewRequest(request.Method, request.Path, bytes.NewBuffer(request.Body))
		if err != nil {
			return err
		}

		// copy headers
		for k, vv := range r.Header {
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}

		// execute request
		rr := newResponseRecorder()
		b.Router.ServeHTTP(rr, req)

		// read response
		body, err := io.ReadAll(rr.Body)
		if err != nil {
			return err
		}

		// write response
		responses[i] = BatchResponse{
			Path:   request.Path,
			Method: request.Method,
			Body:   body,
			Status: rr.Code,
		}
	}

	// write BatchResponses
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(responses)
}

type responseRecorder struct {
	Code      int
	HeaderMap http.Header
	Body      *bytes.Buffer
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		Code:      http.StatusOK,
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
}

func (w *responseRecorder) Header() http.Header {
	return w.HeaderMap
}

func (w *responseRecorder) Write(b []byte) (int, error) {
	return w.Body.Write(b)
}

func (w *responseRecorder) WriteHeader(code int) {
	w.Code = code
}
