package rrdialer

import (
	"context"
	"database/sql"
	"net"

	"github.com/go-sql-driver/mysql"
)

func NewMySQLChecker(dsn string) (Checker, error) {
	conf, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, addr string) error {
		conf.Addr = addr
		db, err := sql.Open("mysql", conf.FormatDSN())
		if err != nil {
			return err
		}
		return db.Ping()
	}, nil
}

func NewTCPChecker() (Checker, error) {
	return func(ctx context.Context, addr string) error {
		var da net.Dialer
		conn, err := da.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		return conn.Close()
	}, nil
}
