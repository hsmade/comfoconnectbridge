package instrumentation

import (
	"io"
	"net/http"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"

	"github.com/uber/jaeger-lib/metrics/prometheus"

	"github.com/hsmade/comfoconnectbridge/pkg/helpers"
)

func EnableTracing(serviceName string, hostPort string) io.Closer {
	cfg := config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  hostPort,
		},
	}
	tracer, closer, err := cfg.New(
		serviceName,
		config.Logger(jaeger.StdLogger),
		config.Metrics(prometheus.New()),
	)
	if err != nil {
		helpers.StackLogger().Fatal(err)
	}

	opentracing.SetGlobalTracer(tracer)
	return closer
}

func EnableMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8090", nil)
}
