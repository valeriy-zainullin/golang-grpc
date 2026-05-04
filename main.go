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
	metricsListenAddr := "0.0.0.0:8000" // TODO: listen only to localhost and forward port with a command.

	restGrpcInvokeAddr := utils.DefaultGrpcInvokeAddress

	exit := make(chan struct{})
	restExited := make(chan struct{})
	grpcExited := make(chan struct{})
	metricsExited := make(chan struct{})

	go utils.RunGrpcServer(grpcListenAddr, exit, grpcExited)
	go utils.RunRestServer(restGrpcInvokeAddr, restListenAddr, exit, restExited)
	go utils.RunMetricsServer(metricsListenAddr, exit, metricsExited)

	log.Println("gRPC server listening on ", grpcListenAddr)
	log.Println("REST api server listening on ", restListenAddr)
	log.Println("Metrics api server listening on ", metricsListenAddr)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	close(exit)

	<-restExited
	log.Println("REST api server exited")

	<-grpcExited
	log.Println("gRPC server exited")

	<-metricsExited
	log.Println("Metrics api server exited")
}
