package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/fujiwara/rrdialer"
	"github.com/go-sql-driver/mysql"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checkerDSN := "root@tcp(dummy)/test"
	opt := rrdialer.NewOption()
	opt.Logger = log.New(os.Stderr, "", log.LstdFlags)
	opt.CheckFunc = rrdialer.NewMySQLCheckFunc(checkerDSN)
	addr := []string{"127.0.0.1:3306", "127.0.0.1:3307"}
	da := rrdialer.NewDialer(ctx, addr, opt)
	mysql.RegisterDial("roundrobin",
		func(_ string) (net.Conn, error) {
			return da.DialTimeout("tcp", 5*time.Second)
		},
	)
	c := time.Tick(time.Second)
	for _ = range c {
		ping()
	}
}

func ping() {
	db, err := sql.Open("mysql", "root@roundrobin(dummy)/test")
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT version()")
	if err != nil {
		log.Println(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("version is %s\n", version)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
