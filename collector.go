package main

import (
	"bufio"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "rockchip"

var (
	socInfo = prometheus.NewDesc(namespace+"_soc_info",
		"Board and SoC identification from the device tree.", []string{"model", "soc", "compatible", "serial"}, nil)

	gpuInfo = prometheus.NewDesc(namespace+"_gpu_info",
		"Mali GPU model.", []string{"device", "model"}, nil)
	gpuLoad = prometheus.NewDesc(namespace+"_gpu_load_ratio",
		"Load of a Mali GPU, from 0 to 1.", []string{"device"}, nil)

	npuInfo = prometheus.NewDesc(namespace+"_npu_info",
		"RKNPU driver version.", []string{"version"}, nil)
	npuLoad = prometheus.NewDesc(namespace+"_npu_load_ratio",
		"Load of an NPU core, from 0 to 1.", []string{"core"}, nil)

	rgaInfo = prometheus.NewDesc(namespace+"_rga_info",
		"RGA driver version.", []string{"version"}, nil)
	rgaLoad = prometheus.NewDesc(namespace+"_rga_load_ratio",
		"Load of an RGA scheduler, from 0 to 1.", []string{"scheduler", "core"}, nil)

	vpuInfo = prometheus.NewDesc(namespace+"_vpu_info",
		"MPP driver version (commit).", []string{"version"}, nil)
	vpuLoad = prometheus.NewDesc(namespace+"_vpu_load_ratio",
		"Load of a VPU engine (decoder, encoder, JPEG, IEP) over the MPP load interval, from 0 to 1.", []string{"device"}, nil)
	vpuUtilization = prometheus.NewDesc(namespace+"_vpu_utilization_ratio",
		"Utilization of a VPU engine over the MPP load interval, from 0 to 1.", []string{"device"}, nil)

	regulatorVoltage = prometheus.NewDesc(namespace+"_regulator_voltage_volts",
		"Output voltage of a regulator (vdd_cpu_big, vdd_gpu, vdd_npu, ...).", []string{"regulator"}, nil)
)

// clocks maps devfreq devices to components by the node name in "<address>.<node>" or "<node>",
// with devfreq as the fallback for unknown devices.
var clocks = []struct {
	prefixes []string
	clock
}{
	{[]string{"gpu"}, newClock("gpu", "GPU")},
	{[]string{"npu"}, newClock("npu", "NPU")},
	{[]string{"rkvenc", "rkvdec", "vepu", "vdpu", "jpeg", "iep", "av1d", "avsd"}, newClock("vpu", "VPU engine")},
	{[]string{"dmc"}, newClock("memory", "memory controller")},
	{[]string{"vop"}, newClock("display", "display controller")},
	{nil, newClock("devfreq", "devfreq device")},
}

// clock holds the devfreq metrics of a component, shaped like node_exporter's node_cpu_scaling_*.
type clock struct {
	frequency, frequencyMin, frequencyMax, governor *prometheus.Desc
}

func newClock(component, name string) clock {
	prefix := namespace + "_" + component + "_"
	return clock{
		frequency:    prometheus.NewDesc(prefix+"frequency_hertz", "Current frequency of a "+name+".", []string{"device"}, nil),
		frequencyMin: prometheus.NewDesc(prefix+"frequency_min_hertz", "Minimum frequency of a "+name+".", []string{"device"}, nil),
		frequencyMax: prometheus.NewDesc(prefix+"frequency_max_hertz", "Maximum frequency of a "+name+".", []string{"device"}, nil),
		governor:     prometheus.NewDesc(prefix+"governor", "Frequency governor of a "+name+", 1 for the active one.", []string{"device", "governor"}, nil),
	}
}

func clockOf(device string) clock {
	node := device[strings.Index(device, ".")+1:]
	for _, c := range clocks {
		for _, prefix := range c.prefixes {
			if strings.HasPrefix(node, prefix) {
				return c.clock
			}
		}
	}
	return clocks[len(clocks)-1].clock
}

var (
	npuCoreRe   = regexp.MustCompile(`Core(\d+):\s*(\d+)%`)
	npuSingleRe = regexp.MustCompile(`(\d+)%`)
	rgaSchedRe  = regexp.MustCompile(`scheduler\[(\d+)\]:\s*(\S+)`)
	rgaLoadRe   = regexp.MustCompile(`load\s*=\s*(\d+)%`)
	gpuIDRe     = regexp.MustCompile(`\s+0x[0-9A-Fa-f]+$`)
	vpuLoadRe   = regexp.MustCompile(`(?m)^(\S+)\s+load:\s*([\d.]+)%\s+utilization:\s*([\d.]+)%`)
)

// Collector reads Rockchip metrics not covered by node_exporter on every scrape.
type Collector struct {
	SysFS   string
	ProcFS  string
	DebugFS string
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{socInfo, gpuInfo, gpuLoad, npuInfo, npuLoad, rgaInfo, rgaLoad,
		vpuInfo, vpuLoad, vpuUtilization, regulatorVoltage} {
		ch <- d
	}
	for _, c := range clocks {
		ch <- c.frequency
		ch <- c.frequencyMin
		ch <- c.frequencyMax
		ch <- c.governor
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.collectSoC(ch)
	c.collectDevfreq(ch)
	c.collectGPU(ch)
	c.collectNPU(ch)
	c.collectRGA(ch)
	c.collectVPU(ch)
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
		clock := clockOf(device)
		if current := read(filepath.Join(dir, "governor")); current != "" {
			governors := strings.Fields(read(filepath.Join(dir, "available_governors")))
			if !slices.Contains(governors, current) {
				governors = append(governors, current)
			}
			for _, governor := range governors {
				active := 0.0
				if governor == current {
					active = 1
				}
				ch <- prometheus.MustNewConstMetric(clock.governor, prometheus.GaugeValue, active, device, governor)
			}
		}
		for file, desc := range map[string]*prometheus.Desc{
			"cur_freq": clock.frequency, "min_freq": clock.frequencyMin, "max_freq": clock.frequencyMax,
		} {
			if v, ok := parseFloat(read(filepath.Join(dir, file))); ok {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, device)
			}
		}
	}
}

// devfreq "load" is not used: drivers without busy tracking (NPU, VENC, VOP) report 100,
// and DMC load depends on the time since the last read.
func (c *Collector) collectGPU(ch chan<- prometheus.Metric) {
	dirs, _ := filepath.Glob(filepath.Join(c.SysFS, "class/devfreq/*/device"))
	for _, dir := range dirs {
		device := filepath.Base(filepath.Dir(dir))
		// Format: "Mali-G610 4 cores r0p0 0x0A080607", without the trailing product ID
		if model := read(filepath.Join(dir, "gpuinfo")); model != "" {
			ch <- prometheus.MustNewConstMetric(gpuInfo, prometheus.GaugeValue, 1, device, gpuIDRe.ReplaceAllString(model, ""))
		}
		// Format: "<percent>", averaged over utilisation_period ms by the Mali kbase driver
		if v, ok := parseFloat(read(filepath.Join(dir, "utilisation"))); ok {
			ch <- prometheus.MustNewConstMetric(gpuLoad, prometheus.GaugeValue, v/100, device)
		}
	}
}

func (c *Collector) collectNPU(ch chan<- prometheus.Metric) {
	// Format: "RKNPU driver: v0.9.8"
	if version := c.readDriver("rknpu", "version"); version != "" {
		ch <- prometheus.MustNewConstMetric(npuInfo, prometheus.GaugeValue, 1, afterColon(version))
	}
	// Format: "NPU load:  Core0:  5%, Core1:  0%, Core2:  0%," or "NPU load:  5%"
	load := c.readDriver("rknpu", "load")
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
	// Format: "RGA multicore Device Driver: v1.3.13"
	if version := c.readDriver("rkrga", "driver_version"); version != "" {
		ch <- prometheus.MustNewConstMetric(rgaInfo, prometheus.GaugeValue, 1, afterColon(version))
	}
	// Format: "scheduler[0]: rga3_core0" followed by "load = 5%"
	var scheduler, core string
	sc := bufio.NewScanner(strings.NewReader(c.readDriver("rkrga", "load")))
	for sc.Scan() {
		if m := rgaSchedRe.FindStringSubmatch(sc.Text()); m != nil {
			scheduler, core = m[1], m[2]
		} else if m := rgaLoadRe.FindStringSubmatch(sc.Text()); m != nil && core != "" {
			v, _ := strconv.ParseFloat(m[1], 64)
			ch <- prometheus.MustNewConstMetric(rgaLoad, prometheus.GaugeValue, v/100, scheduler, core)
		}
	}
}

func (c *Collector) collectVPU(ch chan<- prometheus.Metric) {
	// Format: "90eefde08d7d author: ... video: rockchip: mpp: ...", led by the driver commit
	if version := strings.Fields(read(filepath.Join(c.ProcFS, "mpp_service/version"))); len(version) > 0 {
		ch <- prometheus.MustNewConstMetric(vpuInfo, prometheus.GaugeValue, 1, version[0])
	}
	// Format: "fdbd0000.rkvenc-core      load:  17.41% utilization:  17.09%"
	for _, m := range vpuLoadRe.FindAllStringSubmatch(read(filepath.Join(c.ProcFS, "mpp_service/load")), -1) {
		load, _ := strconv.ParseFloat(m[2], 64)
		utilization, _ := strconv.ParseFloat(m[3], 64)
		ch <- prometheus.MustNewConstMetric(vpuLoad, prometheus.GaugeValue, load/100, m[1])
		ch <- prometheus.MustNewConstMetric(vpuUtilization, prometheus.GaugeValue, utilization/100, m[1])
	}
}

// EnableVPULoad sets the MPP load interval (ms) when it is unset, as the driver only measures VPU load once it is set.
func (c *Collector) EnableVPULoad(interval int) {
	path := filepath.Join(c.ProcFS, "mpp_service/load_interval")
	if interval == 0 || read(path) != "0" {
		return
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(interval)), 0); err != nil {
		slog.Warn("enabling VPU load failed", "path", path, "err", err)
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

// readDriver reads a file of a Rockchip driver from debugfs, or from procfs on kernels built with its PROC_FS option.
func (c *Collector) readDriver(driver, file string) string {
	if s := read(filepath.Join(c.DebugFS, driver, file)); s != "" {
		return s
	}
	return read(filepath.Join(c.ProcFS, driver, file))
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

// afterColon returns the trimmed text after the last colon, like the version in "RKNPU driver: v0.9.8".
func afterColon(s string) string {
	return strings.TrimSpace(s[strings.LastIndex(s, ":")+1:])
}

func parseFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
