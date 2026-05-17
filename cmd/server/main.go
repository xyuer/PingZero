package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/xyuer/PingZero/internal/relay"
)

func main() {
	addr := flag.String("listen", ":51820", "UDP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	server := relay.NewServer(*addr)
	if err := server.ListenAndServe(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
