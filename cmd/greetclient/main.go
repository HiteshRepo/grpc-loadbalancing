package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/hiteshrepo/grpc-loadbalancing/internal/app/greetclient"
)

func main() {
	a := greetclient.NewGreetClient()
	a.Start()
	<-interrupt()
	a.Shutdown()
}

func interrupt() chan os.Signal {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	return interrupt
}
