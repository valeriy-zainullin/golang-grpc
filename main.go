package main

import (
	"log"
	"main/utils"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	grpcAddr := utils.DefaultGrpcAddress
	httpAddr := utils.DefaultHttpRestAddress

	go func() {
		go utils.RunRestServer(grpcAddr, httpAddr)
		log.Printf("REST api server listening on %s", httpAddr)
	}()

	go func() {
		go utils.RunGrpcServer(grpcAddr)
		log.Println("gRPC server listening on " + grpcAddr)
	}()

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	<-exit
}
