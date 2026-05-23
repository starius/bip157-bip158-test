// Command fake-adapter exposes the test adapter API over a deterministic
// chainlab fixture. It is a harness self-test target, not a Bitcoin client.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/bip157-bip158-test/suite/adaptertest/fake"
	"github.com/bip157-bip158-test/suite/chainlab"
)

func main() {
	var listen string
	flag.StringVar(&listen, "listen", "127.0.0.1:0", "HTTP listen address")
	flag.Parse()

	fixture, err := chainlab.BuildLongWalletFixture(chainlab.DefaultLongChainHeight)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build fixture: %v\n", err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler: fake.NewServer(fixture).Handler(),
	}
	fmt.Printf("listening=http://%s\n", listener.Addr())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}
