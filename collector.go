package main

import (
	"bufio"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "rockchip"

var (
	socInfo = prometheus.NewDesc(namespace+"_soc_info",
		"Board and SoC identification from the device tree.", []string{"model", "soc", "compatible", "serial"}, nil)

	devfreqFrequency = prometheus.NewDesc(namespace+"_devfreq_frequency_hertz",
		"Current frequency of a devfreq device (GPU, NPU, DMC, ...).", []string{"device"}, nil)
	devfreqMinFrequency = prometheus.NewDesc(namespace+"_devfreq_min_frequency_hertz",
		"Minimum frequency of a devfreq device.", []string{"device"}, nil)
	devfreqMaxFrequency = prometheus.NewDesc(namespace+"_devfreq_max_frequency_hertz",
		"Maximum frequency of a devfreq device.", []string{"device"}, nil)
	devfreqLoad = prometheus.NewDesc(namespace+"_devfreq_load_ratio",
		"Load of a devfreq device, from 0 to 1 (Rockchip BSP kernels only).", []string{"device"}, nil)
	devfreqInfo = prometheus.NewDesc(namespace+"_devfreq_info",
		"Governor of a devfreq device.", []string{"device", "governor"}, nil)

	npuInfo = prometheus.NewDesc(namespace+"_npu_info",
		"RKNPU driver version.", []string{"version"}, nil)
	npuLoad = prometheus.NewDesc(namespace+"_npu_load_ratio",
		"Load of an NPU core, from 0 to 1.", []string{"core"}, nil)

	rgaLoad = prometheus.NewDesc(namespace+"_rga_load_ratio",
		"Load of an RGA scheduler, from 0 to 1.", []string{"scheduler", "core"}, nil)

	regulatorVoltage = prometheus.NewDesc(namespace+"_regulator_voltage_volts",
		"Output voltage of a regulator (vdd_cpu_big, vdd_gpu, vdd_npu, ...).", []string{"regulator"}, nil)
)

var (
	devfreqLoadRe = regexp.MustCompile(`^(\d+)@`)
	npuCoreRe     = regexp.MustCompile(`Core(\d+):\s*(\d+)%`)
	npuSingleRe   = regexp.MustCompile(`(\d+)%`)
	rgaSchedRe    = regexp.MustCompile(`scheduler\[(\d+)\]:\s*(\S+)`)
	rgaLoadRe     = regexp.MustCompile(`load\s*=\s*(\d+)%`)
)

// Collector reads Rockchip metrics not covered by node_exporter on every scrape.
type Collector struct {
	SysFS   string
	ProcFS  string
	DebugFS string
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{socInfo, devfreqFrequency, devfreqMinFrequency, devfreqMaxFrequency,
		devfreqLoad, devfreqInfo, npuInfo, npuLoad, rgaLoad, regulatorVoltage} {
		ch <- d
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.collectSoC(ch)
	c.collectDevfreq(ch)
	c.collectNPU(ch)
	c.collectRGA(ch)
	c.collectRegulators(ch)
}

func (c *Collector) collectSoC(ch chan<- prometheus.Metric) {
	dt := filepath.Join(c.ProcFS, "device-tree")
	compatible := strings.Split(strings.Trim(read(filepath.Join(dt, "compatible")), "\x00"), "\x00")
	if compatible[0] == "" {
		return
	}
	soc := ""
	for _, v := range compatible {
		if strings.HasPrefix(v, "rockchip,") {
			soc = strings.TrimPrefix(v, "rockchip,")
		}
	}
	model := strings.Trim(read(filepath.Join(dt, "model")), "\x00")
	serial := strings.Trim(read(filepath.Join(dt, "serial-number")), "\x00")
	ch <- prometheus.MustNewConstMetric(socInfo, prometheus.GaugeValue, 1, model, soc, compatible[0], serial)
}

func (c *Collector) collectDevfreq(ch chan<- prometheus.Metric) {
	dirs, _ := filepath.Glob(filepath.Join(c.SysFS, "class/devfreq/*"))
	for _, dir := range dirs {
		device := filepath.Base(dir)
		for file, desc := range map[string]*prometheus.Desc{
			"cur_freq": devfreqFrequency, "min_freq": devfreqMinFrequency, "max_freq": devfreqMaxFrequency,
		} {
			if v, ok := parseFloat(read(filepath.Join(dir, file))); ok {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, device)
			}
		}
		// Format: "<percent>@<freq>Hz"
		if m := devfreqLoadRe.FindStringSubmatch(read(filepath.Join(dir, "load"))); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			ch <- prometheus.MustNewConstMetric(devfreqLoad, prometheus.GaugeValue, v/100, device)
		}
		if governor := read(filepath.Join(dir, "governor")); governor != "" {
			ch <- prometheus.MustNewConstMetric(devfreqInfo, prometheus.GaugeValue, 1, device, governor)
		}
	}
}

func (c *Collector) collectNPU(ch chan<- prometheus.Metric) {
	dir := filepath.Join(c.DebugFS, "rknpu")
	// Format: "RKNPU driver: v0.9.8"
	if version := read(filepath.Join(dir, "version")); version != "" {
		version = strings.TrimSpace(version[strings.LastIndex(version, ":")+1:])
		ch <- prometheus.MustNewConstMetric(npuInfo, prometheus.GaugeValue, 1, version)
	}
	// Format: "NPU load:  Core0:  5%, Core1:  0%, Core2:  0%," or "NPU load:  5%"
	load := read(filepath.Join(dir, "load"))
	if cores := npuCoreRe.FindAllStringSubmatch(load, -1); cores != nil {
		for _, m := range cores {
			v, _ := strconv.ParseFloat(m[2], 64)
			ch <- prometheus.MustNewConstMetric(npuLoad, prometheus.GaugeValue, v/100, m[1])
		}
	} else if m := npuSingleRe.FindStringSubmatch(load); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		ch <- prometheus.MustNewConstMetric(npuLoad, prometheus.GaugeValue, v/100, "0")
	}
}

func (c *Collector) collectRGA(ch chan<- prometheus.Metric) {
	// Format: "scheduler[0]: rga3_core0" followed by "load = 5%"
	var scheduler, core string
	sc := bufio.NewScanner(strings.NewReader(read(filepath.Join(c.DebugFS, "rkrga/load"))))
	for sc.Scan() {
		if m := rgaSchedRe.FindStringSubmatch(sc.Text()); m != nil {
			scheduler, core = m[1], m[2]
		} else if m := rgaLoadRe.FindStringSubmatch(sc.Text()); m != nil && core != "" {
			v, _ := strconv.ParseFloat(m[1], 64)
			ch <- prometheus.MustNewConstMetric(rgaLoad, prometheus.GaugeValue, v/100, scheduler, core)
		}
	}
}

func (c *Collector) collectRegulators(ch chan<- prometheus.Metric) {
	dirs, _ := filepath.Glob(filepath.Join(c.SysFS, "class/regulator/*"))
	for _, dir := range dirs {
		name := read(filepath.Join(dir, "name"))
		if v, ok := parseFloat(read(filepath.Join(dir, "microvolts"))); ok && name != "" {
			ch <- prometheus.MustNewConstMetric(regulatorVoltage, prometheus.GaugeValue, v/1e6, name)
		}
	}
}

// read returns the trimmed file content, or "" when it is missing or unreadable.
func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Debug("read failed", "path", path, "err", err)
		}
		return ""
	}
	return strings.TrimSpace(string(b))
}

func parseFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
