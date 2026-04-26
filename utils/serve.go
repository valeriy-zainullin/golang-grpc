package utils

import (
	"context"
	"log"
	proto "main/gen"
	"main/models"
	"main/services"
	"net"
	"net/http"

	grpcGateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/redis/go-redis/v9"
)

const DefaultGrpcAddress = "0.0.0.0:50051"
const DefaultHttpRestAddress = "0.0.0.0:8081"

const DefaultGrpcInvokeAddress = "localhost:50051"
const DefaultHttpRestInvokeAddress = "localhost:8081"

const DefaultDBConnectionString = "user=postgres password=postgres dbname=postgres host=postgresql port=5432 sslmode=disable TimeZone=Europe/Moscow"

const DefaultRedisAddr = "redis:6379"
const TestRedisAddr = "redis-test:6379"

func migrate(db *gorm.DB) {
	err := db.AutoMigrate(&models.User{})
	if err != nil {
		log.Fatalf("Failed to apply migrations for users: %v", err)
	}

	err = db.AutoMigrate(&models.Post{})
	if err != nil {
		log.Fatalf("Failed to apply migrations for posts: %v", err)
	}

	err = db.AutoMigrate(&models.Like{})
	if err != nil {
		log.Fatalf("Failed to apply migrations for likes: %v", err)
	}

}

func ConnectToDB() *gorm.DB {
	connectionString := DefaultDBConnectionString
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	migrate(db)

	return db
}


func ConnectToRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     DefaultRedisAddr,
		Password: "",
		DB:       0,
	})
}

func ConnectToTestRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     DefaultRedisAddr,
		Password: "",
		DB:       1,
	})
}

/* Also useful: https://habr.com/ru/articles/658769/ */
func RunRestServer(grpcAddr, httpAddr string, exit chan struct{}, exited chan struct{}) {
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

	server := http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}
	err = server.ListenAndServe()

	go func() {
		<-exit
		server.Shutdown(context.TODO())
		close(exited)
	}()

	if err != nil {
		log.Fatalf("Http rest server failed to listen: %v", err)
	}
}

func RunGrpcServer(grpcAddr string, exit chan struct{}, exited chan struct{}) {
	grpcServer := grpc.NewServer()

	blog := services.Blog{}

	db := ConnectToDB()
	FillDB(db)
	blog.SetDB(db)

	rdb := ConnectToRedis()
	blog.SetRedisDB(rdb)

	proto.RegisterBlogServer(grpcServer, &blog)

	listener, err := net.Listen("tcp", grpcAddr)

	go func() {
		<-exit
		listener.Close()
	}()

	if err != nil {
		log.Fatalf("Grpc server failed to listen: %v", err)
	}

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("Grpc server failed to serve: %v", err)
	}

	close(exited)
}
