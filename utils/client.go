package utils

import (
	"log"
	proto "main/gen"

	"google.golang.org/grpc"
)

func MakeBlogClient(grpcAddress string) proto.BlogClient {
	// Устанавливаем соединение
	conn, err := grpc.Dial(grpcAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Grpc client failed to connect to %s", grpcAddress)
	}

	client := proto.NewBlogClient(conn)

	return client
}
