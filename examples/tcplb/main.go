package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/fujiwara/rrdialer"
)

func main() {
	ctx := context.Background()
	upstreams := []string{"127.0.0.1:5000", "127.0.0.1:5001"}
	opt := rrdialer.NewOption()
	opt.CheckFunc = rrdialer.NewTCPCheckFunc()
	opt.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime)
	opt.NextUpstream = true
	da := rrdialer.NewDialer(ctx, upstreams, opt)

	ln, err := net.Listen("tcp", "127.0.0.1:8888")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handler(conn, da)
	}
}

func handler(conn net.Conn, da *rrdialer.Dialer) {
	defer conn.Close()
	upConn, err := da.DialTimeout("tcp", time.Second)
	if err != nil {
		log.Println("upstream error", err)
		return
	}
	defer upConn.Close()
	log.Printf("client %s connect to upstream %s", conn.RemoteAddr(), upConn.RemoteAddr())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(conn, upConn)
	}()
	go func() {
		defer wg.Done()
		io.Copy(upConn, conn)
	}()
	wg.Wait()
}
