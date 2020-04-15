package tracing

import (
	"io"
	"net/http"
	"time"
	"log"

	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"

	"github.com/uber/jaeger-lib/metrics/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New(serviceName string) (opentracing.Tracer, io.Closer) {
	cfg := config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  "tower:5775", // localhost:5775
		},
	}
	tracer, closer, err := cfg.New(
		serviceName,
		config.Logger(jaeger.StdLogger),
		config.Metrics(prometheus.New()),
	)
	if err != nil {
		log.Fatal(err)
	}


	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8090", nil)

	return tracer, closer
}
