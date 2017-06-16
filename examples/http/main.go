package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/fujiwara/rrdialer"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opt := rrdialer.NewOption()
	opt.NextUpstream = true
	da := rrdialer.NewDialer(ctx, []string{"127.0.0.1:5000", "127.0.0.1:5001"}, opt)

	c := time.Tick(time.Second)
	for _ = range c {
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					return da.DialContext(ctx, network)
				},
				Dial: func(network, _ string) (net.Conn, error) {
					return da.Dial(network)
				},
			},
		}

		res, err := client.Get("http://example.com/")
		if err != nil {
			log.Println(err)
			continue
		}
		fmt.Println("Status", res.StatusCode)
		io.Copy(os.Stdout, res.Body)
		res.Body.Close()
	}
}
