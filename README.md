# rockchip-exporter

Prometheus exporter for Rockchip SoCs (RK3588, RK3568, ...), exposing what [node_exporter](https://github.com/prometheus/node_exporter) does not cover. Run it next to node_exporter, which already covers CPU frequency, thermal zones and hwmon.

## Metrics

| Metric                                       | Labels                                 | Source                                               |
| -------------------------------------------- | -------------------------------------- | ---------------------------------------------------- |
| `rockchip_soc_info`                          | `model`, `soc`, `compatible`, `serial` | `/proc/device-tree`                                  |
| `rockchip_devfreq_frequency_hertz`           | `device`                               | `/sys/class/devfreq/*/cur_freq` (GPU, NPU, DMC, ...) |
| `rockchip_devfreq_{min,max}_frequency_hertz` | `device`                               | `/sys/class/devfreq/*/{min,max}_freq`                |
| `rockchip_devfreq_load_ratio`                | `device`                               | `/sys/class/devfreq/*/load` (Rockchip BSP kernels)   |
| `rockchip_devfreq_info`                      | `device`, `governor`                   | `/sys/class/devfreq/*/governor`                      |
| `rockchip_npu_info`                          | `version`                              | `/sys/kernel/debug/rknpu/version`                    |
| `rockchip_npu_load_ratio`                    | `core`                                 | `/sys/kernel/debug/rknpu/load`                       |
| `rockchip_rga_load_ratio`                    | `scheduler`, `core`                    | `/sys/kernel/debug/rkrga/load`                       |
| `rockchip_regulator_voltage_volts`           | `regulator`                            | `/sys/class/regulator/*/microvolts`                  |

Missing sources are skipped, so the same binary runs on any board. NPU and RGA load come from debugfs and need root.

## Usage

```sh
docker run -d --name rockchip-exporter -p 9988:9988 \
  -v /proc:/host/proc:ro -v /sys:/host/sys:ro -v /sys/kernel/debug:/host/sys/kernel/debug:ro \
  ghcr.io/ckoliber/rockchip-exporter:0.1.0 \
  --path.procfs=/host/proc --path.sysfs=/host/sys --path.debugfs=/host/sys/kernel/debug
```

Flags: `--web.listen-address` (`:9988`), `--web.telemetry-path` (`/metrics`), `--path.procfs`, `--path.sysfs`, `--path.debugfs`.

## Development

```sh
mise run go:test
mise run docker:dev
```
