// telemetry package is used to send telemetry data to a remote server
// only general information about runtime and version is sent, anonymously
package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	REPORT_URL      = "http://127.0.0.1:8080/" // TODO: change to real url
	REPORT_INTERVAL = 1 * time.Hour
	REPORT_DEBUG    = false
)

func logMsg(msg string) {
	if REPORT_DEBUG {
		log.Println("[TELEMETRY] " + msg)
	}
}

func send(data any) {
	logMsg(fmt.Sprintf("sending report: %s", data))

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

	// set headers
	req.Header.Set("Content-Type", "application/json")

	// create http request to send to telemetry data
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logMsg(fmt.Sprintf("error sending http request: %s", err))
		return
	}
	defer res.Body.Close()

	// read the response
	data, err = io.ReadAll(res.Body)
	if err != nil {
		logMsg(fmt.Sprintf("error reading http response: %s", err))
		return
	}

	logMsg(fmt.Sprintf("received response: %s", data))
}

func init() {
	if !ENABLED {
		logMsg("telemetry disabled")
		return
	} else {
		logMsg("telemetry enabled")
	}

	started := time.Now()

	var err error
	sessionID, err = utils.NewUID(16)
	if err != nil {
		logMsg(fmt.Sprintf("error generating session id: %s", err))
	}

	// create random generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	go func() {
		// wait random time (within 30s) to avoid sending multiple reports at the same time
		timeOffset := time.Duration(r.Intn(30)) * time.Second
		logMsg(fmt.Sprintf("waiting %s before sending first report", timeOffset))
		time.Sleep(timeOffset)

		hostname, err := os.Hostname()
		if err != nil {
			logMsg(fmt.Sprintf("error getting hostname: %s", err))
			hostname = "unknown"
		}

		// send init report
		send(map[string]any{
			"version":    "1", // version of the report format
			"type":       "init",
			"session_id": sessionID,
			"started":    started,
			"time":       time.Now(),
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
		})

		terminateSignals := make(chan os.Signal, 1)
		signal.Notify(terminateSignals, syscall.SIGINT, syscall.SIGTERM)

		// send heartbeat reports every REPORT_INTERVAL with random offset
		ticker := time.NewTicker(REPORT_INTERVAL + timeOffset)
		defer ticker.Stop()

		for {
			select {
			case <-terminateSignals:
				send(map[string]any{
					"version":    "1", // version of the report format
					"type":       "terminate",
					"session_id": sessionID,
					"started":    started,
					"time":       time.Now(),
				})
				return
			case <-ticker.C:
				send(map[string]any{
					"version":    "1", // version of the report format
					"type":       "heartbeat",
					"session_id": sessionID,
					"started":    started,
					"time":       time.Now(),
				})
			}
		}
	}()
}
