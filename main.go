package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	addr := flag.String("web.listen-address", ":9988", "Address to listen on")
	path := flag.String("web.telemetry-path", "/metrics", "Path to expose metrics on")
	c := &Collector{}
	flag.StringVar(&c.SysFS, "path.sysfs", "/sys", "sysfs mountpoint")
	flag.StringVar(&c.ProcFS, "path.procfs", "/proc", "procfs mountpoint")
	flag.StringVar(&c.DebugFS, "path.debugfs", "/sys/kernel/debug", "debugfs mountpoint (NPU and RGA load, needs root)")
	flag.Parse()

	reg := prometheus.NewRegistry()
	reg.MustRegister(c, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	http.Handle(*path, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	slog.Info("listening", "addr", *addr, "path", *path)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
