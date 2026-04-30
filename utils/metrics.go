package utils

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Method duration histogram
	MethodHandlingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_handling_seconds",
		Help:    "Histogram of GRPC method handling durations by method and status",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "status"})

	// Number of method calls
	MethodCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_calls_total",
		Help: "Total number of GRPC calls by method and status",
	}, []string{"method", "status"})

	// Both of MethodCalls and MethodHandlingDuration are useful for profiling
	//   and subsequent optimization. First, we can choose the most used method
	//   and then optimize execution for different status values.

	// It would be better to use 1% slowest calls metric, but that would
	//   also require setting up grafana.. So relying on these two for now.

)

func InitMetrics() {
	prometheus.MustRegister(MethodHandlingDuration)
	prometheus.MustRegister(MethodCalls)

}

func MakeUnaryMetricsInterceptor() grpc.UnaryServerInterceptor {
	return grpc_prometheus.UnaryServerInterceptor

}

func MakeStreamingMetricsInterceptor() grpc.StreamServerInterceptor {
	return grpc_prometheus.StreamServerInterceptor
}
