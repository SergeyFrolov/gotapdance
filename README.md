<p align="center">
<a href="https://refraction.network"><img src="https://user-images.githubusercontent.com/5443147/30133006-7c3019f4-930f-11e7-9f60-3df45ee13d9d.png" alt="refract"></a>
<h1 class="header-title" align="center">TapDance Client</h1>

<p align="center">TapDance is a free-to-use anti-censorship technology, protected from enumeration attacks.</p>
<p align="center">
<a href="https://travis-ci.org/sergeyfrolov/gotapdance"><img src="https://travis-ci.org/sergeyfrolov/gotapdance.svg?label=build"></a>
<a href="https://godoc.org/github.com/sergeyfrolov/gotapdance/tapdance"><img src="https://img.shields.io/badge/godoc-reference-blue.svg"></a>
	<a href="https://goreportcard.com/report/github.com/sergeyfrolov/gotapdance"><img src="https://goreportcard.com/badge/github.com/sergeyfrolov/gotapdance"></a>
</p>

# Build
## Download Golang and TapDance and dependencies
1. Install [Golang](https://golang.org/dl/) (currently tested against 1.8-1.9 versions), set GOPATH:

 ```bash
GOPATH="${HOME}/go/"
```

2. Get source code for Go Tapdance and all dependencies:

 ```bash
go get github.com/sergeyfrolov/gotapdance github.com/sirupsen/logrus \
           github.com/agl/ed25519/extra25519 golang.org/x/crypto/curve25519 \
           github.com/refraction-networking/utls github.com/sergeyfrolov/bsbuffer \
           github.com/golang/protobuf/proto
```
Ignore the "no buildable Go source files" warning.

If you have outdated versions of libraries above you might want to do `go get -u all`

## Usage

 There are several ways to use TapDance:

 * [Command Line Interface client](cli)

 * Mobile: native applications in Java/Objective C for Android or iOS. Golang bindings are used as a shared library.

   * [Android application in Java](android)
    
   * iOS version: coming ~~soon~~ eventually

   * [Golang Bindings](gobind)
 
 * Use tapdance directly from other Golang program:

```Golang
package main

import (
	"github.com/sergeyfrolov/gotapdance/tapdance"
	"fmt"
)

func main() {
	tdConn, err := tapdance.Dial("tcp", "censoredsite.com:80")
	if err != nil {
		fmt.Printf("tapdance.Dial() failed: %+v\n", err)
		return
	}
	// tdConn implements standard net.Conn, allowing to use it like any other Golang conn with
	// Write(), Read(), Close() etc. It also allows to pass tdConn to functions that expect
	// net.Conn, such as tls.Client() making it easy to do tls handshake over TapDance conn.
	_, err = tdConn.Write([]byte("GET / HTTP/1.1\nHost: censoredsite.com\n\n"))
	if err != nil {
		fmt.Printf("tdConn.Write() failed: %+v\n", err)
		return
	}
	buf := make([]byte, 16384)
	_, err = tdConn.Read(buf)
	// ...
}
```


 # Links
 
 [Refraction Networking](https://refraction.network) is an umberlla term for the family of similarly working technnologies.
 
 TapDance station code released for FOCI'17 on github: [refraction-networking/tapdance](https://github.com/refraction-networking/tapdance) 
 
 Original 2014 paper: ["TapDance: End-to-Middle Anticensorship without Flow Blocking"](https://ericw.us/trow/tapdance-sec14.pdf)
 
 Newer(2017) paper that shows TapDance working at high-scale: ["An ISP-Scale Deployment of TapDance"](https://sfrolov.io/papers/foci17-paper-frolov_0.pdf)
