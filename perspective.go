//go:generate go run assets_gen.go

package main

import (
	"database/sql"
	"flag"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	mysqldrv "github.com/go-sql-driver/mysql" // for DSN parsing only
	_ "github.com/mattn/go-sqlite3"
	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/backend"
	"github.com/wansing/perspective/classes"
	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/sqldb"
	"github.com/wansing/perspective/sqldb/mysql"
	"github.com/wansing/perspective/sqldb/sqlite3"
	"gopkg.in/ini.v1"
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

func main() {

	log.SetFlags(0) // no log prefixes required, on most systems systemd-journald adds them

	var base = flag.String("base", "", "A `prefix` like \"/foo\" which is stripped off every request URI and prepended to every link. Your reverse proxy must not strip it. So if you're using nginx, the \"proxy_pass\" value should not end with a slash.")
	var dbDriver = flag.String("db-driver", "sqlite3", `database driver, can be "mysql" (untested), "postgres" (untested) or "sqlite3"`)
	var dbDSN = flag.String("db-dsn", "perspective.sqlite3", "database data source name")
	var listen = flag.String("listen", "127.0.0.1:8080", "`ip:port` to listen at")
	var hmacSecret = flag.String("hmacsecret", "", "secret key for HMAC signatures of resized images")
	flag.Parse()

	// <body> is like mainRoute.Include("/", "path/foo/bar", "body")
	rootTemplate := template.Must(template.New("").Parse(`
{{ define "base" -}}
<!DOCTYPE html>
<html{{ with .GetGlobal "lang" }} lang="{{ . }}"{{ end }}>
	<head>
		<base href="` + *base + `">
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
</html>
{{ end }}`))

	if *dbDriver == "mysql" {

		var mergedCfg = &mysqldrv.Config{}

		// load ~/.my.cnf if it exists

		homedir, err := os.UserHomeDir()
		if err != nil {
			log.Printf("error getting home directory: %v", err)
			return
		}

		if iniCfg, err := ini.InsensitiveLoad(homedir + "/.my.cnf"); err == nil {

			mergedCfg.DBName = iniCfg.Section("client").Key("database").String()
			mergedCfg.Passwd = iniCfg.Section("client").Key("password").String()
			mergedCfg.User = iniCfg.Section("client").Key("user").String()

			if socket := iniCfg.Section("client").Key("socket").String(); socket != "" {
				mergedCfg.Addr = socket
				mergedCfg.Net = "unix"
			} else {
				port := iniCfg.Section("client").Key("port").String()
				if port == "" {
					port = "3306"
				}
				mergedCfg.Addr = "localhost:" + port
				mergedCfg.Net = "tcp"
			}

		} else {
			if !os.IsNotExist(err) {
				log.Printf("error loading ~/.my.cnf: %v", err)
				return
			}
		}

		// load argument

		argCfg, err := mysqldrv.ParseDSN(*dbDSN)
		if err != nil {
			log.Printf("error parsing mysql dsn argument: %v", err)
			return
		}

		if argCfg.DBName != "" {
			mergedCfg.DBName = argCfg.DBName
		}

		if argCfg.Passwd != "" {
			mergedCfg.Passwd = argCfg.Passwd
		}

		if argCfg.User != "" {
			mergedCfg.User = argCfg.User
		}

		if argCfg.Addr != "" || argCfg.Net != "" {
			mergedCfg.Addr = argCfg.Addr
			mergedCfg.Net = argCfg.Net
		}

		// fixed collation and fallback database name

		mergedCfg.Collation = "utf8mb4_unicode_ci" // this is crucial, else "Error 1267: Illegal mix of collations" in Prepare

		if mergedCfg.DBName == "" {
			mergedCfg.DBName = "cms"
		}

		*dbDSN = mergedCfg.FormatDSN()
	}

	var sqlDB, err = sql.Open(*dbDriver, *dbDSN)
	if err != nil {
		log.Printf("could not open sql database: %v", err)
		return
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("could not ping sql database: %v", err)
		return
	}

	authDB := &auth.AuthDB{}
	authDB.GroupDB = sqldb.NewGroupDB(sqlDB)
	authDB.UserDB = sqldb.NewUserDB(sqlDB)
	authDB.WorkflowDB = sqldb.NewWorkflowDB(sqlDB)

	var sessionStore scs.Store
	switch *dbDriver {
	case "mysql":
		sessionStore = mysql.NewSessionStore(sqlDB)
	case "sqlite3":
		sessionStore = sqlite3.NewSessionStore(sqlDB)
	default:
		log.Println("unknown database backend")
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
	db.NodeDB = sqldb.NewNodeDB(sqlDB)
	db.IndexDB = sqldb.NewIndexDB(sqlDB)

	db.Auth = *authDB
	db.HMACSecret = *hmacSecret
	db.SqlDB = sqlDB

	defer func() {
		log.Println("closing database")
		sqlDB.Close()
	}()

	// mux
	//
	// golang mux recovers from panics, so the program won't crash

	var waitingControllers sync.WaitGroup

	handleStrip(*base+"/assets", http.FileServer(assets))
	handleStrip(*base+"/backend", backend.NewBackendRouter(db))
	handleStrip(*base+"/static", http.FileServer(http.Dir("static")))
	handleStrip(*base+"/upload", db.Uploads)

	handleStrip(
		*base,
		http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {

				waitingControllers.Add(1)
				defer waitingControllers.Done()

				var request = db.NewRequest(w, req)

				var mainRoute, err = core.NewRoute(request, req.URL.Path)
				if err != nil {
					http.NotFound(w, req)
					return
				}
				mainRoute.Execute = true
				defer mainRoute.Cleanup()

				if err := mainRoute.RootRecurse(); err != nil {
					http.NotFound(w, req)
				}

				// rootTemplate could be the content of a virtual node. But that would be much effort, so we just do this:

				if !mainRoute.IsHTML() {
					w.Write([]byte(mainRoute.Get("body")))
					return
				}

				if err = rootTemplate.ExecuteTemplate(w, "base", mainRoute); err != nil {
					http.NotFound(w, req)
					return
				}
			},
		),
	)

	// listener and listen

	sigintChannel := make(chan os.Signal, 1)

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("listening to %s", *listen)

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
