package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/demodesk/neko"
	"github.com/demodesk/neko/pkg/utils"
)

// sessionID identifies current instance of neko
// since it has been started until it is stopped
var sessionID string

var (
	ENABLED = true

	REPORT_URL      = "http://127.0.0.1/" // TODO: change to real url
	REPORT_INTERVAL = 5 * time.Minute
	REPORT_ERRORS   = false
	REPORT_PARAMS   = map[string]string{}
)

func logMsg(msg string) {
	if REPORT_ERRORS {
		fmt.Println(msg)
	}
}

func send(data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		logMsg(fmt.Sprintf("error marshalling json: %s", err))
		return
	}

	// create http request to send to telemetry data
	req, err := http.NewRequest("POST", REPORT_URL, bytes.NewBuffer(raw))
	if err != nil {
		logMsg(fmt.Sprintf("error creating http request: %s", err))
		return
	}

	// create http request to send to telemetry data
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logMsg(fmt.Sprintf("error sending http request: %s", err))
		return
	}
	defer res.Body.Close()

	// read the response
	_, err = io.Copy(io.Discard, res.Body)
	if err != nil {
		logMsg(fmt.Sprintf("error reading http response: %s", err))
		return
	}
}

func init() {
	if !ENABLED {
		logMsg("telemetry disabled")
		return
	}

	var err error
	sessionID, err = utils.NewUID(16)
	if err != nil {
		logMsg(fmt.Sprintf("error generating session id: %s", err))
	}

	go func() {
		// wait random time to avoid sending all reports at the same time
		time.Sleep(time.Duration(rand.Intn(15)) * time.Second)

		hostname, err := os.Hostname()
		if err != nil {
			logMsg(fmt.Sprintf("error getting hostname: %s", err))
			hostname = "unknown"
		}

		// send init report
		send(map[string]any{
			"type":       "init",
			"session_id": sessionID,
			"time":       time.Now().UTC().Unix(),
			// runtime info
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"cpus":       runtime.NumCPU(),
			"go_version": runtime.Version(),
			"compiler":   runtime.Compiler,
			// version info
			"git_commit": neko.Version.GitCommit,
			"git_branch": neko.Version.GitBranch,
			"git_tag":    neko.Version.GitTag,
			"build_date": neko.Version.BuildDate,
			// extra info
			"binary_name": filepath.Base(os.Args[0]),
			"hostname":    hostname,
			"params":      REPORT_PARAMS,
		})

		terminateSignals := make(chan os.Signal, 1)
		signal.Notify(terminateSignals, syscall.SIGINT, syscall.SIGTERM)

		ticker := time.NewTicker(REPORT_INTERVAL)
		defer ticker.Stop()

		for {
			select {
			case sig := <-terminateSignals:
				send(map[string]any{
					"type":       "terminate",
					"session_id": sessionID,
					"time":       time.Now().UTC().Unix(),
					"signal":     sig.String(),
				})
				return
			case <-ticker.C:
				send(map[string]any{
					"type":       "heartbeat",
					"session_id": sessionID,
					"time":       time.Now().UTC().Unix(),
				})
			}
		}
	}()
}
