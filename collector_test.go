package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCollector(t *testing.T) {
	root := writeTree(t, map[string]string{
		"proc/device-tree/compatible":                        "radxa,rock-5b\x00rockchip,rk3588\x00",
		"proc/device-tree/model":                             "Radxa ROCK 5B\x00",
		"proc/device-tree/serial-number":                     "abc123\x00",
		"sys/class/devfreq/fb000000.gpu/cur_freq":            "300000000\n",
		"sys/class/devfreq/fb000000.gpu/min_freq":            "300000000\n",
		"sys/class/devfreq/fb000000.gpu/max_freq":            "1000000000\n",
		"sys/class/devfreq/fb000000.gpu/load":                "100@300000000Hz\n",
		"sys/class/devfreq/fb000000.gpu/device/utilisation":  "45\n",
		"sys/class/devfreq/fb000000.gpu/device/gpuinfo":      "Mali-G610 4 cores r0p0 0x0A080607\n",
		"sys/class/devfreq/fb000000.gpu/governor":            "simple_ondemand\n",
		"sys/class/devfreq/fb000000.gpu/available_governors": "userspace simple_ondemand\n",
		"sys/class/devfreq/dmc/cur_freq":                     "2400000000\n",
		"sys/class/devfreq/dmc/governor":                     "performance\n",
		"sys/class/devfreq/fdbd0000.rkvenc-core/cur_freq":    "786431991\n",
		"sys/class/devfreq/ff000000.unknown/cur_freq":        "100000000\n",
		"sys/class/regulator/regulator.1/name":               "vdd_gpu_s0\n",
		"sys/class/regulator/regulator.1/microvolts":         "675000\n",
		"debug/rknpu/version":                                "RKNPU driver: v0.9.8\n",
		"debug/rknpu/load":                                   "NPU load:  Core0: 12%, Core1:  0%, Core2:  0%,\n",
		"proc/mpp_service/version":                           "90eefde08d7d author: Chandler Chen 2026-06-01 video: rockchip: mpp: fix vepu510\n",
		"debug/rkrga/driver_version":                         "RGA multicore Device Driver: v1.3.13\n",
		"proc/mpp_service/load":                              "fdbd0000.rkvenc-core      load:  17.41% utilization:  17.09%\nfdc38100.rkvdec-core      load:   0.00% utilization:   0.00%\n",
		"debug/rkrga/load":                                   "num of scheduler = 2\n================= load ==================\nscheduler[0]: rga3_core0\n\t load = 7%\n-----------------------------------\nscheduler[1]: rga2\n\t load = 0%\n",
	})

	c := &Collector{
		SysFS:   filepath.Join(root, "sys"),
		ProcFS:  filepath.Join(root, "proc"),
		DebugFS: filepath.Join(root, "debug"),
	}
	want := `
		# HELP rockchip_gpu_info Mali GPU model.
		# TYPE rockchip_gpu_info gauge
		rockchip_gpu_info{device="fb000000.gpu",model="Mali-G610 4 cores r0p0"} 1
		# HELP rockchip_gpu_load_ratio Load of a Mali GPU, from 0 to 1.
		# TYPE rockchip_gpu_load_ratio gauge
		rockchip_gpu_load_ratio{device="fb000000.gpu"} 0.45
		# HELP rockchip_gpu_frequency_hertz Current frequency of a GPU.
		# TYPE rockchip_gpu_frequency_hertz gauge
		rockchip_gpu_frequency_hertz{device="fb000000.gpu"} 3e+08
		# HELP rockchip_gpu_frequency_min_hertz Minimum frequency of a GPU.
		# TYPE rockchip_gpu_frequency_min_hertz gauge
		rockchip_gpu_frequency_min_hertz{device="fb000000.gpu"} 3e+08
		# HELP rockchip_gpu_frequency_max_hertz Maximum frequency of a GPU.
		# TYPE rockchip_gpu_frequency_max_hertz gauge
		rockchip_gpu_frequency_max_hertz{device="fb000000.gpu"} 1e+09
		# HELP rockchip_gpu_governor Frequency governor of a GPU, 1 for the active one.
		# TYPE rockchip_gpu_governor gauge
		rockchip_gpu_governor{device="fb000000.gpu",governor="simple_ondemand"} 1
		rockchip_gpu_governor{device="fb000000.gpu",governor="userspace"} 0
		# HELP rockchip_memory_frequency_hertz Current frequency of a memory controller.
		# TYPE rockchip_memory_frequency_hertz gauge
		rockchip_memory_frequency_hertz{device="dmc"} 2.4e+09
		# HELP rockchip_memory_governor Frequency governor of a memory controller, 1 for the active one.
		# TYPE rockchip_memory_governor gauge
		rockchip_memory_governor{device="dmc",governor="performance"} 1
		# HELP rockchip_vpu_frequency_hertz Current frequency of a VPU engine.
		# TYPE rockchip_vpu_frequency_hertz gauge
		rockchip_vpu_frequency_hertz{device="fdbd0000.rkvenc-core"} 7.86431991e+08
		# HELP rockchip_devfreq_frequency_hertz Current frequency of a devfreq device.
		# TYPE rockchip_devfreq_frequency_hertz gauge
		rockchip_devfreq_frequency_hertz{device="ff000000.unknown"} 1e+08
		# HELP rockchip_npu_info RKNPU driver version.
		# TYPE rockchip_npu_info gauge
		rockchip_npu_info{version="v0.9.8"} 1
		# HELP rockchip_npu_load_ratio Load of an NPU core, from 0 to 1.
		# TYPE rockchip_npu_load_ratio gauge
		rockchip_npu_load_ratio{core="0"} 0.12
		rockchip_npu_load_ratio{core="1"} 0
		rockchip_npu_load_ratio{core="2"} 0
		# HELP rockchip_regulator_voltage_volts Output voltage of a regulator (vdd_cpu_big, vdd_gpu, vdd_npu, ...).
		# TYPE rockchip_regulator_voltage_volts gauge
		rockchip_regulator_voltage_volts{regulator="vdd_gpu_s0"} 0.675
		# HELP rockchip_rga_info RGA driver version.
		# TYPE rockchip_rga_info gauge
		rockchip_rga_info{version="v1.3.13"} 1
		# HELP rockchip_rga_load_ratio Load of an RGA scheduler, from 0 to 1.
		# TYPE rockchip_rga_load_ratio gauge
		rockchip_rga_load_ratio{core="rga2",scheduler="1"} 0
		rockchip_rga_load_ratio{core="rga3_core0",scheduler="0"} 0.07
		# HELP rockchip_soc_info Board and SoC identification from the device tree.
		# TYPE rockchip_soc_info gauge
		rockchip_soc_info{compatible="radxa,rock-5b",model="Radxa ROCK 5B",serial="abc123",soc="rk3588"} 1
		# HELP rockchip_vpu_info MPP driver version (commit).
		# TYPE rockchip_vpu_info gauge
		rockchip_vpu_info{version="90eefde08d7d"} 1
		# HELP rockchip_vpu_load_ratio Load of a VPU engine (decoder, encoder, JPEG, IEP) over the MPP load interval, from 0 to 1.
		# TYPE rockchip_vpu_load_ratio gauge
		rockchip_vpu_load_ratio{device="fdbd0000.rkvenc-core"} 0.1741
		rockchip_vpu_load_ratio{device="fdc38100.rkvdec-core"} 0
		# HELP rockchip_vpu_utilization_ratio Utilization of a VPU engine over the MPP load interval, from 0 to 1.
		# TYPE rockchip_vpu_utilization_ratio gauge
		rockchip_vpu_utilization_ratio{device="fdbd0000.rkvenc-core"} 0.1709
		rockchip_vpu_utilization_ratio{device="fdc38100.rkvdec-core"} 0
	`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorProcFS(t *testing.T) {
	root := writeTree(t, map[string]string{
		"proc/rknpu/load":       "NPU load:  33%\n",
		"proc/mpp_service/load": "please set load_interval first!!!\ne.g. set 1000ms to load_interval:\necho 1000 > /proc/mpp_service/load_interval\n",
		"proc/rkrga/load":       "scheduler[0]: rga2\n\t load = 4%\n",
	})

	c := &Collector{
		SysFS:   filepath.Join(root, "sys"),
		ProcFS:  filepath.Join(root, "proc"),
		DebugFS: filepath.Join(root, "debug"),
	}
	want := `
		# HELP rockchip_npu_load_ratio Load of an NPU core, from 0 to 1.
		# TYPE rockchip_npu_load_ratio gauge
		rockchip_npu_load_ratio{core="0"} 0.33
		# HELP rockchip_rga_load_ratio Load of an RGA scheduler, from 0 to 1.
		# TYPE rockchip_rga_load_ratio gauge
		rockchip_rga_load_ratio{core="rga2",scheduler="0"} 0.04
	`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}

// writeTree creates the given files under a temporary directory and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	root := t.TempDir()
	for path, content := range files {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestEnableVPULoad(t *testing.T) {
	for _, tt := range []struct{ current, want string }{{"0", "1000"}, {"500", "500"}} {
		root := writeTree(t, map[string]string{"mpp_service/load_interval": tt.current})
		(&Collector{ProcFS: root}).EnableVPULoad(1000)
		if got := read(filepath.Join(root, "mpp_service/load_interval")); got != tt.want {
			t.Errorf("load_interval %s: got %s, want %s", tt.current, got, tt.want)
		}
	}
}
