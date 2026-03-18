package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/opencode-go/internal/cli"
	"github.com/opencode-go/internal/web"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	if len(os.Args) > 1 && os.Args[1] == "web" {
		if err := web.Start(ctx, ":8080"); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := cli.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
