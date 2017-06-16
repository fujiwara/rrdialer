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
// upstream addresses
addrs := []string{"slave1:3306", "slave2:3307"},

// health checker
opt := rrdialer.NewOption()
// health checker uses "tcp" as usual
opt.CheckFunc = rrdialer.NewMySQLCheckFunc("test:test@tcp(dummy)/test")

da := rrdialer.NewDialer(ctx, addrs, opt)

// register a custom network "roundrobin" for rrdialer into mysql
dsn := "test:test@roundrobin(dummy)/test"
cfg, _ := mysql.ParseDSN(dsn)
mysql.RegisterDial("roundrobin",
	func(_ string) (net.Conn, error) {
		return da.DialTimeout("tcp", cfg.Timeout)
	},
)

// connect via rrdialer
db, _ := sql.Open("mysql", dsn)
```

### Example for TCP

```go
addrs := []string{"host1:8888", "host2:8888"},

opt := rrdialer.NewOption()
opt.CheckFunc = rrdialer.NewTCPCheckFunc()

da := rrdialer.NewDialer(ctx, addrs, opt)
conn, err := da.Dial("tcp")
```

### Example for HTTP health check

```go
addrs := []string{"backend1:80", "backend2:80"}

checkReq, _ := http.NewRequest("GET", "http://example.com/ping", nil)
opt := rrdialer.NewOption()
opt.CheckFunc = rrdialer.NewHTTPCheckFunc(checkReq)

da := rrdialer.NewDialer(ctx, addrs, opt)

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

// client connects to a server via rrdialer
res, err := client.Get("http://example.com/foo/bar")
```

## LICENSE

The MIT License (MIT)

Copyright (c) 2017 FUJIWARA Shunichiro / (c) 2017 KAYAC Inc.
