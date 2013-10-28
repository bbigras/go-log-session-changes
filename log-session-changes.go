package main

import (
	"database/sql"
	"fmt"
	"github.com/brunoqc/go-windows-session-notifications"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

const (
	APP_STARTED   = 1
	SESSION_EVENT = 2
)

func main() {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		log.Panic("APPDATA is not set")
	}

	dataPath := filepath.Join(appData, "go-log-session-changes")
	dbFilePath := filepath.Join(dataPath, "database.sqlite")

	if _, err := os.Stat(dataPath); err != nil {
		if os.IsNotExist(err) {
			// file does not exist
			log.Println("init data directory")
			errMkdirAll := os.MkdirAll(dataPath, 0600)
			if errMkdirAll != nil {
				log.Panicln("errMkdirAll", errMkdirAll)
			}
		} else {
			log.Panic(err)
		}
	}

	m_user, errUser := user.Current()
	if errUser != nil {
		log.Panicln(errUser)
	}

	hostname, errHostname := os.Hostname()
	if errHostname != nil {
		log.Panic(errHostname)
	}

	// db, err := sql.Open("sqlite3", "./foo.db")
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbFilePath))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create table if needed
	sql := `create table if not exists log (id integer not null primary key AUTOINCREMENT, hostname text, username text, dateEvent datetime, type integer, UMsg integer, Param integer);`
	_, err = db.Exec(sql)
	if err != nil {
		log.Printf("%q: %s\n", err, sql)
		return
	}

	_, errExec := db.Exec(`INSERT INTO log (hostname, username, dateEvent, type, UMsg, Param) VALUES (?, ?, ?, ?, ?, ?)`, hostname, m_user.Username, time.Now(), APP_STARTED, nil, nil)
	if errExec != nil {
		log.Fatal(errExec)
	}

	quit := make(chan int)

	chanMessages := make(chan session_notifications.Message, 1000)
	closeChan := make(chan int)

	go func() {
		for {
			select {
			case m := <-chanMessages:
				log.Println("received", m.UMsg, m.Param)

				// UMsg integer, WParam integer
				_, errExec := db.Exec(`INSERT INTO log (hostname, username, dateEvent, type, UMsg, Param) VALUES (?, ?, ?, ?, ?, ?)`, hostname, m_user.Username, time.Now(), SESSION_EVENT, m.UMsg, m.Param)
				if errExec != nil {
					log.Fatal(errExec)
				}
			}
		}
	}()

	session_notifications.Subscribe(chanMessages, closeChan)

	// ctrl+c to quit
	<-quit
}
