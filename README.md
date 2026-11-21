# rockchip-exporter

Prometheus exporter for Rockchip SoCs (RK3588, RK3568, ...), exposing what [node_exporter](https://github.com/prometheus/node_exporter) does not cover. Run it next to node_exporter, which already covers CPU frequency, thermal zones and hwmon.

## Metrics

| Metric                                                            | Labels                                 | Source                                                                 |
| ----------------------------------------------------------------- | -------------------------------------- | ---------------------------------------------------------------------- |
| `rockchip_soc_info`                                               | `model`, `soc`, `compatible`, `serial` | `/proc/device-tree`                                                    |
| `rockchip_gpu_info`                                               | `device`, `model`                      | `/sys/class/devfreq/*/device/gpuinfo` (Mali, Rockchip BSP kernels)     |
| `rockchip_gpu_load_ratio`                                         | `device`                               | `/sys/class/devfreq/*/device/utilisation` (Mali, Rockchip BSP kernels) |
| `rockchip_npu_info`                                               | `version`                              | `/sys/kernel/debug/rknpu/version`                                      |
| `rockchip_npu_load_ratio`                                         | `core`                                 | `/sys/kernel/debug/rknpu/load`                                         |
| `rockchip_rga_info`                                               | `version`                              | `/sys/kernel/debug/rkrga/driver_version`                               |
| `rockchip_rga_load_ratio`                                         | `scheduler`, `core`                    | `/sys/kernel/debug/rkrga/load`                                         |
| `rockchip_vpu_info`                                               | `version`                              | `/proc/mpp_service/version` (driver commit)                            |
| `rockchip_vpu_{load,utilization}_ratio`                           | `device`                               | `/proc/mpp_service/load` (decoders, encoders, JPEG, IEP)               |
| `rockchip_{gpu,npu,vpu,memory,display}_frequency_hertz`           | `device`                               | `/sys/class/devfreq/*/cur_freq`                                        |
| `rockchip_{gpu,npu,vpu,memory,display}_frequency_{min,max}_hertz` | `device`                               | `/sys/class/devfreq/*/{min,max}_freq`                                  |
| `rockchip_{gpu,npu,vpu,memory,display}_governor`                  | `device`, `governor`                   | `/sys/class/devfreq/*/{governor,available_governors}`                  |
| `rockchip_regulator_voltage_volts`                                | `regulator`                            | `/sys/class/regulator/*/microvolts`                                    |

Like `node_exporter`'s `node_cpu_*`, clocks belong to their component: devfreq devices are named after their device tree node (`fb000000.gpu`, `fdab0000.npu`, `fdbd0000.rkvenc-core`, `dmc`, `fdd90000.vop`), which maps them to `gpu`, `npu`, `vpu` (`rkvenc`, `rkvdec`, `vepu`, `vdpu`, `jpeg`, `iep`, `av1d`, `avsd`), `memory` (`dmc`) and `display` (`vop`). Other devfreq devices are exported as `rockchip*devfreq*_`. Governors are one series per available governor, 1 for the active one.

## Support

The exporter reads generic kernel interfaces and skips missing ones, so it runs on any Rockchip board with a device-tree kernel (Linux 3.2 or newer). Which metrics appear depends on the kernel drivers:

| Metrics                                         | Needs                                                     | Typically available on                                 |
| ----------------------------------------------- | --------------------------------------------------------- | ------------------------------------------------------ |
| `rockchip_soc_info`, `rockchip_regulator_*`     | device-tree kernel                                        | all                                                    |
| `rockchip_*_frequency_*`, `rockchip_*_governor` | devfreq drivers (GPU, DMC, NPU, ...)                      | Rockchip BSP kernels; mainline with Panfrost/Lima GPUs |
| `rockchip_gpu_*`                                | Mali kbase driver of Rockchip BSP kernels (Mali-T/G GPUs) | RK356x, RK3576, RK3588, ...                            |
| `rockchip_npu_*`                                | RKNPU2 driver                                             | RK3562, RK356x, RK3576, RK3588                         |
| `rockchip_rga_*`                                | multicore RGA driver (BSP 5.10 or newer)                  | RK356x, RK3576, RK3588                                 |
| `rockchip_vpu_*`                                | MPP driver                                                | Rockchip BSP kernels                                   |

NPU and RGA files are read from debugfs (needs root), or from `/proc/rknpu` and `/proc/rkrga` on kernels built with `CONFIG_ROCKCHIP_RKNPU_PROC_FS` and `CONFIG_ROCKCHIP_RGA_PROC_FS`. The MPP driver only measures VPU load once `/proc/mpp_service/load_interval` is set, so the exporter sets it to `--collector.vpu.load-interval` (1000 ms) at startup when it is unset; this needs root. Binaries and images are built for `amd64`, `arm64` and `armv7`. Tested on RK3588 (FriendlyElec CM3588, Rockchip BSP 6.1).

## Usage

```sh
VERSION=$(curl -s https://api.github.com/repos/ckoliber/rockchip-exporter/releases/latest | sed -n 's/.*"tag_name": *"\(.*\)".*/\1/p')
docker run -d --name rockchip-exporter -p 9988:9988 \
  -v /proc:/host/proc:ro -v /sys:/host/sys:ro -v /sys/kernel/debug:/host/sys/kernel/debug:ro \
  ghcr.io/ckoliber/rockchip-exporter:$VERSION \
  --path.procfs=/host/proc --path.sysfs=/host/sys --path.debugfs=/host/sys/kernel/debug
```

Or run the binary from the [releases](https://github.com/ckoliber/rockchip-exporter/releases) (`linux-amd64`, `linux-arm64`, `linux-armv7`):

```sh
VERSION=$(curl -s https://api.github.com/repos/ckoliber/rockchip-exporter/releases/latest | sed -n 's/.*"tag_name": *"\(.*\)".*/\1/p')
curl -sL https://github.com/ckoliber/rockchip-exporter/releases/download/$VERSION/rockchip-exporter-$VERSION.linux-arm64.tar.gz | tar xz
sudo ./rockchip-exporter-$VERSION.linux-arm64/rockchip-exporter
```

Flags: `--web.listen-address` (`:9988`), `--web.telemetry-path` (`/metrics`), `--path.procfs`, `--path.sysfs`, `--path.debugfs`, `--collector.vpu.load-interval` (`1000`, `0` leaves it unset).

## Grafana

Import [`grafana/dashboard.json`](grafana/dashboard.json) and pick the Prometheus data source that holds the metrics.

## Development

```sh
mise run go:test
```
