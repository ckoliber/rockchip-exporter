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
		"proc/device-tree/compatible":                       "radxa,rock-5b\x00rockchip,rk3588\x00",
		"proc/device-tree/model":                            "Radxa ROCK 5B\x00",
		"proc/device-tree/serial-number":                    "abc123\x00",
		"sys/class/devfreq/fb000000.gpu/cur_freq":           "300000000\n",
		"sys/class/devfreq/fb000000.gpu/min_freq":           "300000000\n",
		"sys/class/devfreq/fb000000.gpu/max_freq":           "1000000000\n",
		"sys/class/devfreq/fb000000.gpu/load":               "100@300000000Hz\n",
		"sys/class/devfreq/fb000000.gpu/device/utilisation": "45\n",
		"sys/class/devfreq/fb000000.gpu/governor":           "simple_ondemand\n",
		"sys/class/regulator/regulator.1/name":              "vdd_gpu_s0\n",
		"sys/class/regulator/regulator.1/microvolts":        "675000\n",
		"debug/rknpu/version":                               "RKNPU driver: v0.9.8\n",
		"debug/rknpu/load":                                  "NPU load:  Core0: 12%, Core1:  0%, Core2:  0%,\n",
		"debug/rkrga/load":                                  "num of scheduler = 2\n================= load ==================\nscheduler[0]: rga3_core0\n\t load = 7%\n-----------------------------------\nscheduler[1]: rga2\n\t load = 0%\n",
	})

	c := &Collector{
		SysFS:   filepath.Join(root, "sys"),
		ProcFS:  filepath.Join(root, "proc"),
		DebugFS: filepath.Join(root, "debug"),
	}
	want := `
		# HELP rockchip_devfreq_frequency_hertz Current frequency of a devfreq device (GPU, NPU, DMC, ...).
		# TYPE rockchip_devfreq_frequency_hertz gauge
		rockchip_devfreq_frequency_hertz{device="fb000000.gpu"} 3e+08
		# HELP rockchip_devfreq_info Governor of a devfreq device.
		# TYPE rockchip_devfreq_info gauge
		rockchip_devfreq_info{device="fb000000.gpu",governor="simple_ondemand"} 1
		# HELP rockchip_devfreq_load_ratio Load of a devfreq device, from 0 to 1 (Mali GPU on Rockchip BSP kernels only).
		# TYPE rockchip_devfreq_load_ratio gauge
		rockchip_devfreq_load_ratio{device="fb000000.gpu"} 0.45
		# HELP rockchip_devfreq_max_frequency_hertz Maximum frequency of a devfreq device.
		# TYPE rockchip_devfreq_max_frequency_hertz gauge
		rockchip_devfreq_max_frequency_hertz{device="fb000000.gpu"} 1e+09
		# HELP rockchip_devfreq_min_frequency_hertz Minimum frequency of a devfreq device.
		# TYPE rockchip_devfreq_min_frequency_hertz gauge
		rockchip_devfreq_min_frequency_hertz{device="fb000000.gpu"} 3e+08
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
		# HELP rockchip_rga_load_ratio Load of an RGA scheduler, from 0 to 1.
		# TYPE rockchip_rga_load_ratio gauge
		rockchip_rga_load_ratio{core="rga2",scheduler="1"} 0
		rockchip_rga_load_ratio{core="rga3_core0",scheduler="0"} 0.07
		# HELP rockchip_soc_info Board and SoC identification from the device tree.
		# TYPE rockchip_soc_info gauge
		rockchip_soc_info{compatible="radxa,rock-5b",model="Radxa ROCK 5B",serial="abc123",soc="rk3588"} 1
	`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorProcFS(t *testing.T) {
	root := writeTree(t, map[string]string{
		"proc/rknpu/load": "NPU load:  33%\n",
		"proc/rkrga/load": "scheduler[0]: rga2\n\t load = 4%\n",
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
