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

func logEvent(host, user string, timestamp time.Time, leType int, umsg, param *int) error {
	db, err := sql.Open("sqlite3", sqliteStr)
	if err != nil {
		return err
	}
	defer db.Close()

	_, errExec := db.Exec("INSERT INTO log (hostname, username, dateEvent, type, UMsg, Param) VALUES (?, ?, ?, ?, ?, ?);", host, user, timestamp, leType, umsg, param)
	if errExec != nil {
		return errExec
	}

	return nil
}

func startUp() error {
	db, err := sql.Open("sqlite3", sqliteStr)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("create table if not exists log (id integer not null primary key AUTOINCREMENT, hostname text, username text, dateEvent datetime, type integer, UMsg integer, Param integer);")
	if err != nil {
		return err
	}

	return logEvent(hostname, username, time.Now(), AppStarted, nil, nil)
}

func processMsg(m session_notifications.Message) error {
	defer close(m.ChanOk)

	log.Println("received", m.UMsg, m.Param)
	return logEvent(hostname, username, time.Now(), SessionEvent, &m.UMsg, &m.Param)
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

	log.Println("startup")

	errSetGlobal := setGlobal(dataPath)
	if errSetGlobal != nil {
		log.Fatal(errSetGlobal)
	}

	errStartup := startUp()
	if errStartup != nil {
		log.Println("errStartup", errStartup)
	}

	// db, err := sql.Open("sqlite3", "./foo.db")
	db, err := sql.Open("sqlite3", sqliteStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	quit := make(chan int)

	chanMessages := make(chan session_notifications.Message, 1000)
	closeChan := make(chan int)

	go func() {
		for {
			select {
			case m := <-chanMessages:
				errProcess := processMsg(m)
				if errProcess != nil {
					log.Println("errProcess", errProcess)
				}
			}
		}
	}()

	session_notifications.Subscribe(chanMessages, closeChan)

	// ctrl+c to quit
	<-quit
}
