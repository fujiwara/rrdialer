# rrdialer

[![GoDoc](https://godoc.org/github.com/fujiwara/rrdialer?status.svg)](http://godoc.org/github.com/fujiwara/rrdialer)
[![Build Status](https://travis-ci.org/fujiwara/rrdialer.svg?branch=master)](https://travis-ci.org/fujiwara/rrdialer)

A round robin net dialer.

rrdialer has features below.

- Dials to one of specified hosts by round robin.
- Health checker by Go's function
  - MySQL
  - TCP
  - HTTP
  - or Your custom function

## Usage

### Without healthchecker

A simple usage.

```go
addrs := []string{"host1:8888", "host2:8888"},
da := rrdialer.NewDialer(ctx, addrs, nil)
conn, err := da.Dial("tcp") // connect to host1:8888 or host2:8888
```

### Example for MySQL

[go-sql-driver/mysql](https://godoc.org/github.com/go-sql-driver/mysql) has an interface to customize dialer.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

addrs := []string{"slave1:3306", "slave2:3307"},
checkerDSN := "test:test@tcp(dummy)/test" // health checker uses "tcp" as usual
opt := rrdialer.NewOption()
opt.CheckFunc = rrdialer.NewMySQLCheckFunc(checkerDSN)
da := rrdialer.NewDialer(ctx, addrs, opt)

// register network "roundrobin" for rrdialer
mysql.RegisterDial("roundrobin",
	func(_ string) (net.Conn, error) {
		return da.DialTimeout("tcp", 5*time.Second)
	},
)

// to use rrdailer, specify a registerd network "roundrobin".
db, _ := sql.Open("mysql", "test:test@roundrobin(dummy)/test")
```

### Example for TCP

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

addrs := []string{"host1:8888", "host2:8888"},
opt := rrdialer.NewOption()
opt.CheckFunc = rrdialer.NewTCPCheckFunc()
da := rrdialer.NewDialer(ctx, addrs, opt)
conn, err := da.Dial("tcp")
```

### Example for HTTP health check

```go
checkReq, _ := http.NewRequest("GET", "http://example.com/ping", nil)
opt := rrdialer.NewOption()
opt.CheckFunc = rrdialer.NewHTTPCheckFunc(checkReq)

da := rrdialer.NewDialer(ctx, []string{"backend1:80", "backend2:80"}, opt)
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
res, err := client.Get("http://example.com/foo/bar")
```

## LICENSE

The MIT License (MIT)

Copyright (c) 2017 FUJIWARA Shunichiro / (c) 2017 KAYAC Inc.
