package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/canonical/go-dqlite/app"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func main() {
	var api string
	var db string
	var join *[]string
	var dir string

	cmd := &cobra.Command{
		Use:   "dqlite-demo",
		Short: "Demo application using dqlite",
		Long: `This demo shows how to integrate a Go application with dqlite.

Complete documentation is available at https://github.com/canonical/go-dqlite`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(dir, db)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return errors.Wrapf(err, "can't create %s", dir)
			}
			app, err := app.New(dir, app.WithAddress(db), app.WithCluster(*join))
			if err != nil {
				return err
			}

			db, err := app.Open(context.Background(), "demo")
			if err != nil {
				return err
			}

			if _, err := db.Exec(schema); err != nil {
				return err
			}

			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				key := strings.TrimLeft(r.URL.Path, "/")
				result := ""
				switch r.Method {
				case "GET":
					row := db.QueryRow(query, key)
					if err := row.Scan(&result); err != nil {
						result = fmt.Sprintf("Error: %s", err.Error())
					}
					break
				case "PUT":
					result = "done"
					value, _ := ioutil.ReadAll(r.Body)
					if _, err := db.Exec(update, key, value); err != nil {
						result = fmt.Sprintf("Error: %s", err.Error())
					}
				default:
					result = fmt.Sprintf("Error: unsupported method %q", r.Method)

				}
				fmt.Fprintf(w, "%s\n", result)
			})

			listener, err := net.Listen("tcp", api)
			if err != nil {
				return err
			}

			go http.Serve(listener, nil)

			ch := make(chan os.Signal)
			signal.Notify(ch, unix.SIGPWR)
			signal.Notify(ch, unix.SIGINT)
			signal.Notify(ch, unix.SIGQUIT)
			signal.Notify(ch, unix.SIGTERM)

			<-ch

			listener.Close()
			db.Close()
			app.Close()

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&api, "api", "a", "", "address used to expose the demo API")
	flags.StringVarP(&db, "db", "d", "", "address used for internal database replication")
	join = flags.StringSliceP("join", "j", nil, "database addresses of existing nodes")
	flags.StringVarP(&dir, "dir", "D", "/tmp/dqlite-demo", "data directory")

	cmd.MarkFlagRequired("api")
	cmd.MarkFlagRequired("db")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

const (
	schema = "CREATE TABLE IF NOT EXISTS model (key TEXT, value TEXT, UNIQUE(key))"
	query  = "SELECT value FROM model WHERE key = ?"
	update = "INSERT OR REPLACE INTO model(key, value) VALUES(?, ?)"
)
