package utils

import (
	"context"
	"log"
	proto "main/gen"
	blog "main/services"
	"net"
	"net/http"

	grpcGateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const DefaultGrpcAddress = "localhost:50051"
const DefaultHttpRestAddress = "localhost:8081"

/* Also useful: https://habr.com/ru/articles/658769/ */
func RunRestServer(grpcAddr, httpAddr string) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := grpcGateway.NewServeMux()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	err := proto.RegisterBlogHandlerFromEndpoint(ctx, mux, grpcAddr, opts)
	if err != nil {
		log.Fatalf("could not register handler: %v", err)
	}

	fileServer := http.FileServer(http.Dir("./swagger-ui"))
	stripPrefixHandler := http.StripPrefix("/swagger-ui/", fileServer)
	mux.HandlePath("GET", "/swagger-ui", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		w.Header().Add("Location", "/swagger-ui/")
		w.WriteHeader(308) // Permanent redirect
	})

	mux.HandlePath("GET", "/swagger-ui/{_}", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		stripPrefixHandler.ServeHTTP(w, r)
	})
	mux.HandlePath("GET", "/swagger.json", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		http.ServeFile(w, r, "./gen/blog.swagger.json")
	})

	err = http.ListenAndServe(httpAddr, mux)

	if err != nil {
		log.Fatalf("Http rest server failed to listen: %v", err)
	}
}

func RunGrpcServer(grpcAddr string) {
	grpcServer := grpc.NewServer()

	blog := blog.Blog{}

	/* Should be in db initialization later on. */
	blog.CreatePost(context.TODO(), &proto.Post{
		Id:        &proto.PostId{Value: 1},
		Author:    &proto.User{},
		Body:      "Hello, world! This is the first post of this blog.",
		CreatedAt: 0, /* A very old post, in fact, no internet at that time even */
		NumLiked:  0,
		Liked:     []*proto.User{},
	})

	proto.RegisterBlogServer(grpcServer, &blog)

	_, err := net.Listen("tcp", grpcAddr)

	if err != nil {
		log.Fatalf("Grpc server failed to listen: %v", err)
	}
}
