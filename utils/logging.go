package utils

import (
	"context"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
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

type LoggingHook struct {
	logger *zap.Logger
}

func (hook LoggingHook) logDialStart(addr string) {
	defer hook.logger.Sync()
	hook.logger.Info(
		"Started connecting to redis server",
		zap.String("Address", addr),
	)
}

func (hook LoggingHook) logDialEnd(addr string, err error, duration time.Duration) {
	defer hook.logger.Sync()
	hook.logger.Info(
		"Finished connecting to redis server",
		zap.String("Address", addr),
		zap.Error(err),
		zap.Duration("Duration", duration),
	)
}

func (hook LoggingHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		start := time.Now()

		hook.logDialStart(addr)

		conn, err := next(ctx, network, addr)

		duration := time.Since(start)
		hook.logDialEnd(addr, err, duration)

		return conn, err
	}
}

func (hook LoggingHook) logProcessStart(cmd string) {
	defer hook.logger.Sync()
	hook.logger.Info(
		"Started redis query",
		zap.String("Command", cmd),
	)
}

func (hook LoggingHook) logProcessEnd(cmd string, err error, duration time.Duration) {
	defer hook.logger.Sync()
	hook.logger.Info(
		"Finished redis query",
		zap.String("Command", cmd),
		zap.Error(err),
		zap.Duration("Duration", duration),
	)
}

func (hook LoggingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()

		hook.logProcessStart(cmd.String())

		err := next(ctx, cmd)

		duration := time.Since(start)
		hook.logProcessEnd(cmd.String(), err, duration)

		return err
	}
}

func (hook LoggingHook) logPipelineStart(cmds []string) {
	defer hook.logger.Sync()
	hook.logger.Info(
		"Started redis pipeline",
		zap.Strings("Commands", cmds),
	)
}

func (hook LoggingHook) logPipelineEnd(cmds []string, err error, duration time.Duration) {
	defer hook.logger.Sync()
	hook.logger.Info(
		"Finished redis pipeline",
		zap.Strings("Commands", cmds),
		zap.Error(err),
		zap.Duration("Duration", duration),
	)
}

func (hook LoggingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()

		descriptions := make([]string, len(cmds))
		for _, cmd := range cmds {
			descriptions = append(descriptions, cmd.String())
		}
		hook.logPipelineStart(descriptions)

		err := next(ctx, cmds)

		duration := time.Since(start)
		hook.logPipelineEnd(descriptions, err, duration)

		return err
	}
}

func MakeRedisLoggingHook(logger *zap.Logger) redis.Hook {
	return &LoggingHook{logger: logger}
}
