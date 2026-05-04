package main

import (
	"log"
	"main/utils"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	grpcListenAddr := utils.DefaultGrpcAddress
	restListenAddr := utils.DefaultHttpRestAddress

	restGrpcInvokeAddr := utils.DefaultGrpcInvokeAddress

	exit := make(chan struct{})
	restExited := make(chan struct{})
	grpcExited := make(chan struct{})

	go utils.RunGrpcServer(grpcListenAddr, exit, grpcExited)
	go utils.RunRestServer(restGrpcInvokeAddr, restListenAddr, exit, restExited)

	log.Println("gRPC server listening on ", grpcListenAddr)
	log.Println("REST api server listening on ", restListenAddr)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	close(exit)

	<-restExited
	log.Println("REST api server exited")

	<-grpcExited
	log.Println("gRPC server exited")
}
