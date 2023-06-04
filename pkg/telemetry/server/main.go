package main

// write a simple program that saves data received from HTTP to postgresql database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var (
	dbConn string
	bind   string
)

func init() {
	// get database connection string
	dbConn = os.Getenv("DB_CONN")
	if dbConn == "" {
		log.Fatal("DB_CONN is not set")
	}

	// get bind address
	bind = os.Getenv("BIND")
	if bind == "" {
		bind = ":8080"
	}
}

func main() {
	// create db connection
	db, err := sql.Open("postgres", dbConn)
	if err != nil {
		log.Fatalf("failed to open db connection: %v", err)
	}
	defer db.Close()

	// check db connection

	// wait for db to be ready (max 10 seconds)
	for i := 0; i < 10; i++ {
		err := db.Ping()
		if err == nil {
			break
		}

		log.Printf("failed to ping db: %v, retrying... (%dx)", err, i)
		time.Sleep(time.Second)
	}

	// create table if it does not exist
	if err := createTableIfNotExists(db); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// check if its a POST request
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// check if content type is application/json
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "content type is not application/json", http.StatusBadRequest)
			return
		}

		// read request body
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "unable to read request body", http.StatusBadRequest)
			return
		}

		// insert data into table
		if err := insertIntoTable(db, data); err != nil {
			log.Printf("failed to insert into table: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// write response
		fmt.Fprintf(w, "ok")
	})

	// start http server
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createTableIfNotExists(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS telemetry (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL,
			started TIMESTAMP NOT NULL,
			last_active TIMESTAMP NOT NULL,
			stopped TIMESTAMP,
			os VARCHAR(255) NOT NULL,
			arch VARCHAR(255) NOT NULL,
			cpus INTEGER NOT NULL,
			go_version VARCHAR(255) NOT NULL,
			compiler VARCHAR(255) NOT NULL,
			git_commit VARCHAR(255) NOT NULL,
			git_branch VARCHAR(255) NOT NULL,
			git_tag VARCHAR(255) NOT NULL,
			build_date VARCHAR(255) NOT NULL,
			binary_name VARCHAR(255) NOT NULL,
			hostname VARCHAR(255) NOT NULL
		);
	`)

	return err
}

type Data struct {
	Version   string    `json:"version"`
	Type      string    `json:"type"`
	SessionId string    `json:"session_id"`
	Started   time.Time `json:"started"`
	Time      time.Time `json:"time"`

	// init
	Os         string `json:"os"`
	Arch       string `json:"arch"`
	Cpus       int    `json:"cpus"`
	GoVersion  string `json:"go_version"`
	Compiler   string `json:"compiler"`
	GitCommit  string `json:"git_commit"`
	GitBranch  string `json:"git_branch"`
	GitTag     string `json:"git_tag"`
	BuildDate  string `json:"build_date"`
	BinaryName string `json:"binary_name"`
	Hostname   string `json:"hostname"`
}

func insertIntoTable(db *sql.DB, data []byte) error {
	// parse json
	var d Data
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}

	// TODO: support multiple versions
	if d.Version != "1" {
		return fmt.Errorf("unknown version: %s", d.Version)
	}

	var err error
	switch d.Type {
	case "init":
		_, err = db.Exec(`
			INSERT INTO telemetry (
				session_id, started, last_active,
				os, arch, cpus, go_version, compiler,
				git_commit, git_branch, git_tag, build_date,
				binary_name, hostname
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
			)
		`,
			d.SessionId, d.Started, d.Time,
			d.Os, d.Arch, d.Cpus, d.GoVersion, d.Compiler,
			d.GitCommit, d.GitBranch, d.GitTag, d.BuildDate,
			d.BinaryName, d.Hostname,
		)
	case "terminate":
		_, err = db.Exec(`
			UPDATE telemetry SET stopped = $1 WHERE session_id = $2 AND started = $3
		`, d.Time, d.SessionId, d.Started)
	case "heartbeat":
		_, err = db.Exec(`
			UPDATE telemetry SET last_active = $1 WHERE session_id = $2 AND started = $3
		`, d.Time, d.SessionId, d.Started)
	default:
		err = fmt.Errorf("unknown type: %s", d.Type)
	}

	return err
}
