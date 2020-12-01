//go:generate go run assets_gen.go

package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wansing/perspective/backend"
	//"github.com/wansing/perspective/cache/gcachekbc"
	//"github.com/wansing/perspective/cache/maps"
	"github.com/wansing/perspective/classes"
	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/sqldb"
	"github.com/wansing/perspective/sqldb/mysql"
	"github.com/wansing/perspective/sqldb/sqlite3"
	"github.com/xo/dburl"
	"golang.org/x/crypto/ssh/terminal"
)

type prefixedResponseWriter struct {
	http.ResponseWriter
	prefix string // without trailing slash
}

// shadows the original WriteHeader func
func (w prefixedResponseWriter) WriteHeader(statusCode int) {
	// prepend prefix to Location header, so redirects work
	if w.prefix != "" {
		if location := w.Header().Get("Location"); len(location) > 0 && location[0] == '/' { // only absolute locations
			w.Header().Set("Location", w.prefix+location)
		}
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// prefix should be without trailing slash
func handleStrip(prefix string, handler http.Handler) {
	http.Handle(
		prefix+"/", // http mux needs trailing slash
		http.StripPrefix(
			prefix,
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w = &prefixedResponseWriter{w, prefix}
					handler.ServeHTTP(w, r)
				},
			),
		),
	)
}

func init() {
	log.SetFlags(0) // no log prefixes, on most systems systemd-journald adds them
}

func main() {

	var dbArg string // is in both FlagSets

	// default FlagSet

	// Your reverse proxy must not strip the prefix. So if you're using nginx, the "proxy_pass" value should not end with a slash."
	var base = flag.String("base", "", "strip off this `prefix` from every HTTP request and prepended it to every link")
	// MySQL: collation should be utf8mb4_unicode_ci
	flag.StringVar(&dbArg, "db", "sqlite3:perspective.sqlite3?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared", "sql database url, see github.com/xo/dburl")
	var hmacKey = flag.String("hmac", "", "use this secret HMAC `key` for serving resized images")
	var listenAddr = flag.String("listen", "127.0.0.1:8080", "serve HTTP content at this `ip:port`")

	// init FlagSet

	var initFlags = flag.NewFlagSet("init", flag.ExitOnError)

	initFlags.StringVar(&dbArg, "db", "sqlite3:perspective.sqlite3?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared", "sql database url, see github.com/xo/dburl") // copied from above
	var initInsert = initFlags.Bool("insert", false, "creates the given group or user")
	var initJoin = initFlags.Bool("join", false, "joins the given user to the given group")
	var initMakeAdmin = initFlags.Bool("make-admin", false, "gives admin permissions to the given group")
	var groupname = initFlags.String("group", "", "specifies a group `name`")
	var username = initFlags.String("user", "", "specifies a user `name`")

	if len(os.Args) > 1 && os.Args[1] == "init" {
		initFlags.Parse(os.Args[2:])
	} else {
		flag.Parse()
	}

	// database

	dbURL, err := dburl.Parse(dbArg)
	if err != nil {
		log.Printf("could not parse database url: %v", err)
		return
	}

	sqlDB, err := sql.Open(dbURL.Driver, dbURL.DSN)
	if err != nil {
		log.Printf("could not open sql database: %v", err)
		return
	}

	if err = sqlDB.Ping(); err != nil {
		log.Printf("could not ping sql database: %v", err)
		return
	}

	log.Printf("using database %s", dbURL.String())

	// base

	*base = strings.Trim(*base, "/")
	if *base != "" {
		*base = "/" + *base
	}

	// assemble stuff

	var sessionStore scs.Store
	switch dbURL.Driver {
	case "mysql":
		sessionStore = mysql.NewSessionStore(sqlDB)
	case "sqlite3":
		sessionStore = sqlite3.NewSessionStore(sqlDB)
	default:
		log.Printf("unknown database backend: %s", dbURL.Driver)
		return
	}

	db := &core.CoreDB{}
	err = db.Init(sessionStore, *base)
	if err != nil {
		log.Println(err) // log.Fatalln would not run deferred functions
		return
	}

	db.AccessDB = sqldb.NewAccessDB(sqlDB)
	db.ClassRegistry = classes.DefaultRegistry
	db.EditorsDB = sqldb.NewEditorsDB(sqlDB)
	/*db.NodeDB = maps.NewNodeCache(
		sqldb.NewNodeDB(sqlDB),
	)*/
	db.GroupDB = sqldb.NewGroupDB(sqlDB)
	db.IndexDB = sqldb.NewIndexDB(sqlDB)
	db.NodeDB = sqldb.NewNodeDB(sqlDB)
	db.UserDB = sqldb.NewUserDB(sqlDB)
	db.WorkflowDB = sqldb.NewWorkflowDB(sqlDB)

	db.HMACSecret = *hmacKey
	db.SqlDB = sqlDB

	defer func() {
		log.Println("closing database")
		sqlDB.Close()
	}()

	// init

	if initFlags.Parsed() {
		switch {
		case *initInsert:
			if *groupname != "" {
				insertGroup(db, *groupname)
			}
			if *username != "" {
				insertUser(db, *username)
			}
		case *initJoin:
			if *groupname != "" && *username != "" {
				join(db, *groupname, *username)
			}
		case *initMakeAdmin:
			if *groupname != "" {
				makeAdmin(db, *groupname)
			}
		}
		return
	}

	listen(db, *listenAddr, *base)
}

func insertGroup(db *core.CoreDB, name string) {
	if err := db.InsertGroup(name); err != nil {
		log.Printf(`error creating group "%s": %v`, name, err)
	}
}

func insertUser(db *core.CoreDB, name string) {

	fmt.Printf("password for user %s: ", name)
	pass1, err := terminal.ReadPassword(0)
	fmt.Println()
	if err != nil {
		log.Printf("error reading password: %v", err)
		return
	}

	fmt.Printf("repeat password: ")
	pass2, err := terminal.ReadPassword(0)
	fmt.Println()
	if err != nil {
		log.Printf("error reading password: %v", err)
		return
	}

	if !bytes.Equal(pass1, pass2) {
		log.Printf("passwords don't match")
		return
	}

	user, err := db.InsertUser(name)
	if err != nil {
		log.Printf("error creating user %s: %v", name, err)
		return
	}

	if err := db.SetPassword(user, string(pass1)); err != nil {
		log.Printf("error setting password: %v", err)
		return
	}
}

func join(db *core.CoreDB, groupname string, username string) {

	group, err := db.GetGroupByName(groupname)
	if err != nil {
		log.Printf("error getting group %s: %v", groupname, err)
		return
	}

	user, err := db.GetUserByName(username)
	if err != nil {
		log.Printf("error getting user %s: %v", username, err)
		return
	}

	if err := db.Join(group, user); err != nil {
		log.Printf("error joining: %v", err)
		return
	}
}

func makeAdmin(db *core.CoreDB, groupname string) {

	group, err := db.GetGroupByName(groupname)
	if err != nil {
		log.Printf("error getting group %s: %v", groupname, err)
		return
	}

	if err := db.AccessDB.InsertAccessRule(1, group.ID(), int(core.Admin)); err != nil {
		log.Printf(`error giving root admin permission to group: %v`, err)
		return
	}
}

func listen(db *core.CoreDB, addr string, base string) {

	// mux
	//
	// golang mux recovers from panics, so the program won't crash

	// <body> is like mainQuery.Include("/", "path/foo/bar", "body")
	var rootTemplate = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html{{ with .GetGlobal "lang" }} lang="{{ . }}"{{ end }}>
	<head>
		<base href="` + base + `">
		<meta charset="utf-8">
		{{ .Get "head" }}
		{{- if .HasGlobal "include-bootstrap-4-css" }}
			<link rel="stylesheet" type="text/css" href="/assets/bootstrap-4.4.1.min.css">
			<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
		{{ end -}}
		{{- if .HasGlobal "include-jquery-3" }}
			<!-- Bootstrap's JavaScript requires jQuery. jQuery must be included before Bootstrap's JavaScript. -->
			<script src="/assets/jquery-3.3.1.min.js"></script>
		{{ end -}}
		{{- if .HasGlobal "include-bootstrap-4-js" }}
			<script src="/assets/bootstrap-4.4.1.min.js"></script>
		{{ end -}}
		{{- if .HasGlobal "include-taboverride-4" }}
			<script src="/assets/taboverride-4.0.3.min.js"></script>
		{{ end -}}
	</head>
	<body>
		{{ .RenderNotifications }}
		{{ .Get "body" }}
	</body>
</html>`))

	var waitingControllers sync.WaitGroup

	handleStrip(base+"/assets", http.FileServer(assets))
	handleStrip(base+"/backend", backend.NewBackendRouter(db, base))
	handleStrip(base+"/static", http.FileServer(http.Dir("static")))
	handleStrip(base+"/upload", db.Uploads)

	handleStrip(
		base,
		http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {

				waitingControllers.Add(1)
				defer waitingControllers.Done()

				var request = db.NewRequest(w, req)

				var mainQuery = &core.Query{
					Request: request,
					Queue:   core.NewQueue("/" + core.RootSlug + req.URL.Path),
				}
				defer mainQuery.Cleanup()

				if err := mainQuery.Recurse(); err != nil {
					http.NotFound(w, req)
				}

				// rootTemplate could be the content of a virtual node. But that would be much effort, so we just do this:

				if mainQuery.IsHTML() {
					if err := rootTemplate.Execute(w, mainQuery); err != nil {
						http.NotFound(w, req)
					}
				} else {
					w.Write([]byte(mainQuery.Get("body")))
				}
			},
		),
	)

	// listener and listen

	sigintChannel := make(chan os.Signal, 1)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("listening to %s", addr)

	httpSrv := &http.Server{
		Handler:      db.SessionManager.LoadAndSave(http.DefaultServeMux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := httpSrv.Serve(listener); err != nil {

			// don't panic, we want a graceful shutdown
			if err != http.ErrServerClosed {
				log.Printf("error listening: %v", err)
			}

			// ensure graceful shutdown
			sigintChannel <- os.Interrupt
		}
	}()

	// graceful shutdown

	signal.Notify(sigintChannel, os.Interrupt, syscall.SIGTERM) // SIGINT (Interrupt) or SIGTERM
	<-sigintChannel

	log.Println("shutting down")
	httpSrv.Close()

	waitingControllers.Wait()
}
