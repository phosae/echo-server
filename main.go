package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// renderJSON renders 'v' as JSON and writes it as a response into w.
func renderJSON(w http.ResponseWriter, v interface{}) {
	js, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func main() {
	hello := func(response http.ResponseWriter, _ *http.Request) {
		if _, err := response.Write([]byte("hello world\n")); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	echo := func(response http.ResponseWriter, request *http.Request) {
		rawReq, err := httputil.DumpRequest(request, true)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		req2, _ := httputil.DumpRequest(request, true)
		log.Println(request.RemoteAddr, "echo", string(req2))

		if code := request.URL.Query().Get("code"); len(code) > 0 {
			statusCode, err := strconv.ParseInt(code, 10, 32)
			if err == nil {
				response.WriteHeader(int(statusCode))
			}
		}

		if d := request.URL.Query().Get("duration"); len(d) > 0 {
			duration, err := time.ParseDuration(d)
			if err == nil {
				time.Sleep(duration)
			}
		}

		if _, err := response.Write(rawReq); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	cpu := func(response http.ResponseWriter, _ *http.Request) {
		cpuinfos, err := cpu.Info()
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(response, cpuinfos)
	}
	vmem := func(response http.ResponseWriter, _ *http.Request) {
		vmem, _ := mem.VirtualMemory()
		if _, err := response.Write([]byte(vmem.String())); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	net := func(response http.ResponseWriter, _ *http.Request) {
		ifList, err := net.Interfaces()
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(response, ifList)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", echo)
	mux.HandleFunc("/hello", hello)
	mux.HandleFunc("/cpu", cpu)
	mux.HandleFunc("/mem", vmem)
	mux.HandleFunc("/net", net)
	mux.Handle("/metrics", promhttp.Handler())

	stop := SetupSignalHandler()
	server := &http.Server{Handler: instrumentMux(mux)}
	startErr := make(chan error)
	go func() {
		addr := ":8080"
		if envaddr := os.Getenv("LISTEN_ADDR"); envaddr != "" {
			addr = envaddr
		}
		log.Println("listening on", addr)
		server.Addr = addr
		startErr <- server.ListenAndServe()
	}()

	select {
	case err := <-startErr:
		log.Fatal(err)
	case <-stop:
	}

	shutdownGracePeriod := 10 * time.Second
	if shutdownEnv := os.Getenv("SHUTDOWN_DEADLINE"); shutdownEnv != "" {
		shutdownDuration, err := time.ParseDuration(shutdownEnv)
		if err == nil {
			shutdownGracePeriod = shutdownDuration
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	log.Println("try shutdown server gracefully...")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("err Shutdown server", err)
	} else {
		log.Println("server graceful shutdown ok")
	}
}

func instrumentMux(ha http.Handler, opts ...promhttp.Option) http.Handler {
	// in_flight_requests 10
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	// api_requests_total{code="200",method="get"} 4
	// api_requests_total{code="200",method="post"} 2
	// api_requests_total{code="300",method="get"} 1
	// api_requests_total{code="500",method="get"} 1
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	/*
		response_duration_seconds_bucket{handler="/",method="get",le="0.005"} 1
		...
		response_duration_seconds_bucket{handler="/",method="get",le="0.1"} 1
		...
		response_duration_seconds_bucket{handler="/",method="get",le="+Inf"} 3
		response_duration_seconds_sum{handler="/",method="get"} 4.00311846
		response_duration_seconds_count{handler="/",method="get"} 3


		response_duration_seconds_bucket{handler="/",method="post",le="0.005"} 2
		...
		response_duration_seconds_bucket{handler="/",method="post",le="+Inf"} 2
		response_duration_seconds_sum{handler="/",method="post"} 0.00035004199999999995
		response_duration_seconds_count{handler="/",method="post"} 2
	*/
	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "response_duration_seconds",
			Help:        "A histogram of request latencies.",
			Buckets:     []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			ConstLabels: prometheus.Labels{"handler": "/"},
		},
		[]string{"method"},
	)

	writeHeaderVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "write_header_duration_seconds",
			Help:        "A histogram of time to first write latencies.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"handler": "/"},
		},
		[]string{},
	)

	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{},
	)

	prometheus.MustRegister(inFlightGauge, counter, histVec, responseSize, writeHeaderVec)

	return promhttp.InstrumentHandlerInFlight(inFlightGauge,
		promhttp.InstrumentHandlerCounter(counter,
			promhttp.InstrumentHandlerDuration(histVec,
				promhttp.InstrumentHandlerTimeToWriteHeader(writeHeaderVec,
					promhttp.InstrumentHandlerResponseSize(responseSize, ha, opts...),
					opts...),
				opts...),
			opts...),
	)
}

func SetupSignalHandler() <-chan struct{} {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()
	return stop
}
