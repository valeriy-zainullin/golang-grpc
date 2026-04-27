package utils

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func MakeLogger() *zap.Logger {
	config := zap.NewProductionConfig()

	// доп кастомизация
	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, _ := config.Build()

	return logger
}

func unaryLoggingInterceptor(logger *zap.Logger, ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	defer logger.Sync() // Make sure log entries are not lost on panics and exits.
	logger.Info("Started unary method", zap.String("Name", info.FullMethod))

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	statusCode := codes.OK
	if err != nil {
		s, _ := status.FromError(err)
		statusCode = s.Code()
	}

	logger.Info(
		"Finished unary method",
		zap.String("Name", info.FullMethod),
		zap.String("Status code", statusCode.String()),
		zap.Duration("Duration", duration),
		zap.Error(err),
	)

	return resp, err
}

func streamLoggingInterceptor(logger *zap.Logger, srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()

	defer logger.Sync() // Make sure log entries are not lost on panics and exits.
	logger.Info("Started stream method", zap.String("Name", info.FullMethod))

	err := handler(srv, stream)

	duration := time.Since(start)
	statusCode := codes.OK
	if err != nil {
		s, _ := status.FromError(err)
		statusCode = s.Code()
	}

	logger.Info(
		"Finished stream method",
		zap.String("Name", info.FullMethod),
		zap.String("Status code", statusCode.String()),
		zap.Duration("Duration", duration),
		zap.Error(err),
	)

	return err
}

func MakeUnaryLoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, error error) {
		return unaryLoggingInterceptor(logger, ctx, req, info, handler)
	}
}

func MakeStreamingLoggingInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return streamLoggingInterceptor(logger, srv, stream, info, handler)
	}
}
