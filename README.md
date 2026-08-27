# AMD RDC Exporter

`rdc-exporter` is a Prometheus exporter for AMD GPUs. It uses ROCm Data Center
Tool (RDC) to collect GPU metrics and exposes them on `/metrics`. In Kubernetes,
it can also use the kubelet pod-resources API to attach workload labels such as
`namespace`, `pod`, and `container` to GPU metrics.

## Documentation

Start here:

| Topic | Document |
| --- | --- |
| Configuration guide (English) | [`docs/configuration/README.md`](docs/configuration/README.md) |
| Configuration guide (繁體中文) | [`docs/configuration/README_zhtw.md`](docs/configuration/README_zhtw.md) |
| Configuration guide (简体中文) | [`docs/configuration/README_zhcn.md`](docs/configuration/README_zhcn.md) |
| Kubernetes deployment guide (English) | [`docs/deployment/k8s/README.md`](docs/deployment/k8s/README.md) |
| Kubernetes deployment guide (繁體中文) | [`docs/deployment/k8s/README_zhtw.md`](docs/deployment/k8s/README_zhtw.md) |
| Kubernetes deployment guide (简体中文) | [`docs/deployment/k8s/README_zhcn.md`](docs/deployment/k8s/README_zhcn.md) |
| Helm chart | [`charts/rdc-exporter/README.md`](charts/rdc-exporter/README.md) |

The configuration guide explains the two-layer model (metric list + catalog),
how to select metrics, how to adjust value units with `scale`, and how this
differs from the NVIDIA DCGM exporter.

The Kubernetes guide covers the AMD GPU device-plugin, node-labeller,
`rdc-exporter` DaemonSet, ConfigMap-based metric selection, pod-resources socket
path, profiling counter limits, and a vLLM workload verification example.

## Release Images

Official release images are published to GitHub Container Registry (GHCR):

| Image | ROCm version | Release date |
| --- | --- | --- |
| `ghcr.io/maple52046/rdc-exporter:v1-rocm7.2.4-20260827` | 7.2.4 | 2026-08-27 |
| `ghcr.io/maple52046/rdc-exporter:v1-rocm7.2.2-20260609` | 7.2.2 | 2026-06-09 |

## Quickstart on a GPU Node

Start `rdc-exporter` on a GPU node:

```bash
docker run -dit --name rdc-exporter \
  --device=/dev/kfd \
  --device=/dev/dri \
  --cap-add SYS_PTRACE \
  -p 5000:5000 \
  ghcr.io/maple52046/rdc-exporter:v1-rocm7.2.4-20260827

curl localhost:5000/metrics
```

Example output:

```text
# HELP gpu_memory_usage Memory usage of the GPU instance
# TYPE gpu_memory_usage gauge
gpu_memory_usage{UUID="GPU-0011223344556677",gpu_index="0"} 1335.6769279999999
gpu_memory_usage{UUID="GPU-8899aabbccddeeff",gpu_index="1"} 1335.611392
```

At startup, the exporter reads `rocm-smi --json --showuniqueid` and formats each
hardware identity as `GPU-` followed by 16 hexadecimal digits. If discovery is
unavailable, the exporter continues running and exposes `UUID=""` while logging
a warning.

## Quickstart on Kubernetes

For production-like Kubernetes deployment, read the full guide first:

- [`docs/deployment/k8s/README.md`](docs/deployment/k8s/README.md)

Deploy with the Helm chart ([`charts/rdc-exporter`](charts/rdc-exporter)):

```bash
helm install rdc-exporter ./charts/rdc-exporter -n monitoring --create-namespace
curl localhost:5000/metrics
```

On k0s, override the host pod-resources socket path:

```bash
helm install rdc-exporter ./charts/rdc-exporter -n monitoring --create-namespace \
  --set kubelet.podResourcesSocket=/var/lib/k0s/kubelet/pod-resources/kubelet.sock
```

Or apply the minimal example manifest directly:

```bash
kubectl create namespace monitoring
kubectl -n monitoring apply -f example/rdc-exporter-daemonset.yml
curl localhost:5000/metrics
```

When workloads request GPUs through the AMD device-plugin
(`resources.limits.amd.com/gpu`), exported metrics can include workload labels:

```text
gpu_memory_usage{UUID="GPU-0011223344556677",container="vllm",gpu_index="0",namespace="default",pod="vllm-qwen-..."} 287252.5
```

## Usage

The examples below are a quick reference. For the full configuration model —
metric list vs. catalog, unit scaling, merge/overwrite, and a comparison with the
NVIDIA DCGM exporter — see the [configuration guide](docs/configuration/README.md).

### Monitoring Specific Metrics

Pass fields directly:

```bash
rdc-exporter -e RDC_FI_GPU_CLOCK,812
```

Or read fields from a file:

```bash
cat > metrics.txt <<EOF
RDC_FI_GPU_CLOCK
812
EOF

rdc-exporter -f metrics.txt
```

Each non-empty line is one RDC field name or numeric field ID. Lines beginning
with `#` are ignored.

### Monitoring Specific GPUs

```bash
rdc-exporter -i 0,1
```

### Scaling Metric Values

Use an external catalog file to override metric metadata such as scale. For
example, `RDC_FI_GPU_MEMORY_TOTAL` is scaled to MB by default. To report it in
bytes instead:

```yaml
metrics:
  - metric: RDC_FI_GPU_MEMORY_TOTAL
    scale: 1
```

Run `rdc-exporter` with the custom catalog:

```bash
rdc-exporter --catalog catalog.yml
```

## Building

The `Makefile` owns the ROCm, Go, and image-tag build parameters. The most
important ROCm value is `ROCM_DEB`: it is the actual AMD `amdgpu-install` package
URL used by the Docker build to configure the ROCm apt repository. Keep
`ROCM_VERSION` and `ROCM_DEB` in sync.

```bash
make build
make image
make image-verify
```

Override versions from the command line when needed:

```bash
make image \
  ROCM_VERSION=7.2.4 \
  ROCM_DEB=https://repo.radeon.com/amdgpu-install/7.2.4/ubuntu/noble/amdgpu-install_7.2.4.70204-1_all.deb \
  GO_VERSION=1.26.4
```
