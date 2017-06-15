package rrdialer

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	d := NewDialer("localhost:80", "www.example.com:80")
	for i := 0; i < 10; i++ {
		ta, err := d.pick()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", ta.address)
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
	d := NewDialer(addr)
	conn, err := d.Dial("tcp")
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
	d := NewDialer(addrs...)

	for i := 0; i < 16; i++ {
		conn, err := d.Dial("tcp")
		t.Log("connected", conn.RemoteAddr())
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
	d := NewDialer(addrs...)
	d.Logger = log.New(os.Stderr, "", 0)

	for i := 0; i < 16; i++ {
		conn, err := d.Dial("tcp")
		if err != nil {
			t.Fatal(err)
		}
		t.Log("connected", conn.RemoteAddr())
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
