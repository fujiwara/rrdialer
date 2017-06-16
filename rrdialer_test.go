package rrdialer_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/fujiwara/rrdialer"
)

func TestResolve(t *testing.T) {
	d := rrdialer.NewDialer([]string{"localhost:80", "www.example.com:80"}, nil)
	for i := 0; i < 10; i++ {
		address, err := d.Get()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", address)
	}
}

func testServer(id int, n int, ch chan string) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	ch <- l.Addr().String()
	for i := 0; i < n; i++ {
		conn, _ := l.Accept()
		go func() {
			fmt.Fprintf(conn, "hello %d", id)
			conn.Close()
		}()
	}
	l.Close()
}

func TestConnectSingle(t *testing.T) {
	ch := make(chan string)
	go testServer(0, 9999, ch)
	addr := <-ch
	t.Logf("addr: %s", addr)
	d := rrdialer.NewDialer([]string{addr}, nil)
	a, _ := d.Get()
	conn, err := net.Dial("tcp", a)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello 0" {
		t.Errorf("unexpected response: %s", b)
	}
	t.Logf("response: %s", b)
	conn.Close()
}

func TestConnectMulti(t *testing.T) {
	ch := make(chan string)
	addrs := make([]string, 0)
	for i := 0; i < 4; i++ {
		go testServer(i, 9999, ch)
	}
	for i := 0; i < 4; i++ {
		addr := <-ch
		addrs = append(addrs, addr)
	}
	t.Logf("addrs: %s", addrs)

	m := regexp.MustCompile(`^hello \d+`)
	d := rrdialer.NewDialer(addrs, nil)

	for i := 0; i < 16; i++ {
		addr, _ := d.Get()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ioutil.ReadAll(conn)
		if err != nil {
			t.Fatal(err)
		}
		if !m.Match(b) {
			t.Errorf("unexpected response: %s", b)
		}
		t.Logf("response: %s", b)
		conn.Close()
	}
}

func TestConnectEject(t *testing.T) {
	ch := make(chan string)
	addrs := make([]string, 0)
	go testServer(0, 2, ch)
	go testServer(1, 9999, ch)
	for i := 0; i < 2; i++ {
		addr := <-ch
		addrs = append(addrs, addr)
	}
	t.Logf("addrs: %s", addrs)

	m := regexp.MustCompile(`^hello \d+`)
	opt := rrdialer.NewOption()
	opt.Logger = log.New(os.Stderr, "", log.Ldate)
	opt.EjectTimeout = 3 * time.Second
	opt.EjectThreshold = 2
	opt.CheckInterval = 1 * time.Second

	t.Logf("%#v", opt)
	return

	d := rrdialer.NewDialer(addrs, opt)
	for i := 0; i < 16; i++ {
		conn, err := d.Dial("tcp")
		if err != nil {
			t.Fatal(err)
		}
		b, err := ioutil.ReadAll(conn)
		if err != nil {
			t.Fatal(err)
		}
		if i < 4 {
			if !m.Match(b) {
				t.Errorf("unexpected response: %s", b)
			}
		} else {
			if string(b) != "hello 1" {
				t.Errorf("unexpected response: %s", b)
			}
		}
		t.Logf("response: %s", b)
		conn.Close()
		time.Sleep(300 * time.Millisecond)
	}
}

func TestConnectCheck(t *testing.T) {
	ch := make(chan string)
	addrs := make([]string, 0)
	go testServer(0, 2, ch)
	go testServer(1, 9999, ch)
	for i := 0; i < 2; i++ {
		addr := <-ch
		addrs = append(addrs, addr)
	}
	t.Logf("addrs: %s", addrs)

	m := regexp.MustCompile(`^hello \d+`)
	checker, err := rrdialer.NewTCPChecker()
	if err != nil {
		t.Fatal(err)
	}
	opt := rrdialer.NewOption()
	opt.Logger = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	opt.EjectTimeout = 3 * time.Second
	opt.EjectThreshold = 2
	opt.CheckInterval = 1 * time.Second
	opt.Checker = checker

	d := rrdialer.NewDialer(addrs, opt)
	for i := 0; i < 16; i++ {
		addr, _ := d.Get()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			// refetch
			addr, _ = d.Get()
			conn, err = net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
		}

		b, err := ioutil.ReadAll(conn)
		if err != nil {
			t.Fatal(err)
		}
		if i < 4 {
			if !m.Match(b) {
				t.Errorf("unexpected response: %s", b)
			}
		} else {
			if string(b) != "hello 1" {
				t.Errorf("unexpected response: %s", b)
			}
		}
		t.Logf("response: %s", b)
		conn.Close()
		time.Sleep(300 * time.Millisecond)
	}
}
