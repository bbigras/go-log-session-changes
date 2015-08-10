package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/brunoqc/go-windows-session-notifications"
	_ "github.com/mattn/go-sqlite3"
	"github.com/natefinch/lumberjack"
)

const (
	// AppStarted is when the application starts
	AppStarted = 1
	// SessionEvent is when we get a session event (lock, unlock, logoff)
	SessionEvent = 2
)

var (
	sqliteStr string
	hostname  string
	username  string
)

func setGlobal(dataPath string) error {
	sqliteStr = fmt.Sprintf("file:%s?cache=shared&mode=rwc", filepath.Join(dataPath, "database.sqlite"))

	u, err := user.Current()
	if err != nil {
		return err
	}
	username = u.Username

	hostname, err = os.Hostname()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		log.Panic("APPDATA is not set")
	}

	dataPath := filepath.Join(appData, "go-log-session-changes")

	errMkdirAll := os.MkdirAll(dataPath, 0600)
	if errMkdirAll != nil {
		log.Panicln("errMkdirAll", errMkdirAll)
	}

	log.SetOutput(&lumberjack.Logger{
		Filename:   filepath.Join(dataPath, "go-log-session-changes.log"),
		MaxSize:    1, // megabytes
		MaxBackups: 3,
		MaxAge:     14, //days
	})

	errSetGlobal := setGlobal(dataPath)
	if errSetGlobal != nil {
		log.Fatal(errSetGlobal)
	}

	// db, err := sql.Open("sqlite3", "./foo.db")
	db, err := sql.Open("sqlite3", sqliteStr)
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

	_, errExec := db.Exec(`INSERT INTO log (hostname, username, dateEvent, type, UMsg, Param) VALUES (?, ?, ?, ?, ?, ?)`, hostname, username, time.Now(), AppStarted, nil, nil)
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
				_, errExec := db.Exec(`INSERT INTO log (hostname, username, dateEvent, type, UMsg, Param) VALUES (?, ?, ?, ?, ?, ?)`, hostname, username, time.Now(), SessionEvent, m.UMsg, m.Param)
				if errExec != nil {
					log.Fatal(errExec)
				}
				close(m.ChanOk)
			}
		}
	}()

	session_notifications.Subscribe(chanMessages, closeChan)

	// ctrl+c to quit
	<-quit
}
