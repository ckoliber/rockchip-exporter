# rockchip-exporter

Prometheus exporter for Rockchip SoCs (RK3588, RK3568, ...), exposing what [node_exporter](https://github.com/prometheus/node_exporter) does not cover. Run it next to node_exporter, which already covers CPU frequency, thermal zones and hwmon.

## Metrics

| Metric                                       | Labels                                 | Source                                                                     |
| -------------------------------------------- | -------------------------------------- | -------------------------------------------------------------------------- |
| `rockchip_soc_info`                          | `model`, `soc`, `compatible`, `serial` | `/proc/device-tree`                                                        |
| `rockchip_devfreq_frequency_hertz`           | `device`                               | `/sys/class/devfreq/*/cur_freq` (GPU, NPU, DMC, ...)                       |
| `rockchip_devfreq_{min,max}_frequency_hertz` | `device`                               | `/sys/class/devfreq/*/{min,max}_freq`                                      |
| `rockchip_devfreq_load_ratio`                | `device`                               | `/sys/class/devfreq/*/device/utilisation` (Mali GPU, Rockchip BSP kernels) |
| `rockchip_devfreq_info`                      | `device`, `governor`                   | `/sys/class/devfreq/*/governor`                                            |
| `rockchip_npu_info`                          | `version`                              | `/sys/kernel/debug/rknpu/version`                                          |
| `rockchip_npu_load_ratio`                    | `core`                                 | `/sys/kernel/debug/rknpu/load`                                             |
| `rockchip_rga_load_ratio`                    | `scheduler`, `core`                    | `/sys/kernel/debug/rkrga/load`                                             |
| `rockchip_regulator_voltage_volts`           | `regulator`                            | `/sys/class/regulator/*/microvolts`                                        |

## Support

The exporter reads generic kernel interfaces and skips missing ones, so it runs on any Rockchip board with a device-tree kernel (Linux 3.2 or newer). Which metrics appear depends on the kernel drivers:

| Metrics                                     | Needs                                                     | Typically available on                                 |
| ------------------------------------------- | --------------------------------------------------------- | ------------------------------------------------------ |
| `rockchip_soc_info`, `rockchip_regulator_*` | device-tree kernel                                        | all                                                    |
| `rockchip_devfreq_*` frequency and governor | devfreq drivers (GPU, DMC, NPU, ...)                      | Rockchip BSP kernels; mainline with Panfrost/Lima GPUs |
| `rockchip_devfreq_load_ratio`               | Mali kbase driver of Rockchip BSP kernels (Mali-T/G GPUs) | RK356x, RK3576, RK3588, ...                            |
| `rockchip_npu_*`                            | RKNPU2 driver                                             | RK3562, RK356x, RK3576, RK3588                         |
| `rockchip_rga_load_ratio`                   | multicore RGA driver (BSP 5.10 or newer)                  | RK356x, RK3576, RK3588                                 |

NPU and RGA files are read from debugfs (needs root), or from `/proc/rknpu` and `/proc/rkrga` on kernels built with `CONFIG_ROCKCHIP_RKNPU_PROC_FS` and `CONFIG_ROCKCHIP_RGA_PROC_FS`. Binaries are built for `amd64`, `arm64` and `armv7`, images for `amd64` and `arm64`. Tested on RK3588 (FriendlyElec CM3588, Rockchip BSP 6.1).

## Usage

```sh
docker run -d --name rockchip-exporter -p 9988:9988 \
  -v /proc:/host/proc:ro -v /sys:/host/sys:ro -v /sys/kernel/debug:/host/sys/kernel/debug:ro \
  ghcr.io/ckoliber/rockchip-exporter:0.1.0 \
  --path.procfs=/host/proc --path.sysfs=/host/sys --path.debugfs=/host/sys/kernel/debug
```

Or run the binary from the [releases](https://github.com/ckoliber/rockchip-exporter/releases) (`linux-amd64`, `linux-arm64`, `linux-armv7`):

```sh
curl -sL https://github.com/ckoliber/rockchip-exporter/releases/download/0.1.0/rockchip-exporter-0.1.0.linux-arm64.tar.gz | tar xz
sudo ./rockchip-exporter-0.1.0.linux-arm64/rockchip-exporter
```

Flags: `--web.listen-address` (`:9988`), `--web.telemetry-path` (`/metrics`), `--path.procfs`, `--path.sysfs`, `--path.debugfs`.

## Development

```sh
mise run go:test
```
