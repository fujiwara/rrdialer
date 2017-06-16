package rrdialer

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/go-sql-driver/mysql"
)

// NewMySQLCheckFunc makes CheckFunc for MySQL server
func NewMySQLCheckFunc(dsn string) CheckFunc {
	return func(ctx context.Context, addr string) error {
		conf, err := mysql.ParseDSN(dsn)
		if err != nil {
			return err
		}
		conf.Addr = addr
		db, err := sql.Open("mysql", conf.FormatDSN())
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	}
}

// NewMySQLCheckFunc makes CheckFunc for general TCP server
// This func tries connect and close simply.
func NewTCPCheckFunc() CheckFunc {
	return func(ctx context.Context, addr string) error {
		var da net.Dialer
		conn, err := da.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}

// NewHTTPCheckFunc makes CheckFunc for HTTP server.
// This func send request to server and check HTTP status code.
// A status code over 400 makes failure.
func NewHTTPCheckFunc(req *http.Request) CheckFunc {
	return func(ctx context.Context, addr string) error {
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					var da net.Dialer
					return da.DialContext(ctx, network, addr)
				},
			},
		}
		res, err := client.Do(req.WithContext(ctx))
		if err != nil {
			return err
		}
		defer func() {
			io.Copy(ioutil.Discard, res.Body)
			res.Body.Close()
		}()
		if res.StatusCode >= 400 {
			return fmt.Errorf("HTTP status code: %d", res.StatusCode)
		}
		return nil
	}
}
