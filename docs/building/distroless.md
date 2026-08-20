# Building the TheRock Distroless Image

The release image combines a pre-built `rdc-exporter` binary with a selected
runtime closure from a TheRock ROCm distribution. The final stage is
`gcr.io/distroless/python3-debian13`: RDC requires glibc and dynamically loaded
ROCm libraries, while direct `rocm-smi` support additionally requires Python.
`static-debian13` cannot run this CGO binary, and `base-debian13` does not
contain the Python interpreter used by `rocm-smi`.

This is an artifact-oriented image build. The Dockerfile does not compile
TheRock or the exporter. Prepare both inputs first, then assemble the image.
For the complete Debian 13 source-build procedure, see
[`minimal-therock-rdc.md`](minimal-therock-rdc.md).

## Inputs

- A TheRock ROCm distribution built with RDC and the target GPU architecture.
  Its root must contain `.info/version`, `lib/librdc_bootstrap.so`, the RDC ROCP
  plugin, rocprofiler-sdk, COMGR/LLVM runtime libraries, and `rocm-smi`.
- Go plus the same RDC headers and libraries exposed at `/opt/rocm`, to build
  the CGO exporter binary with `make build`.
- Docker with BuildKit enabled.

The tested baseline is TheRock 7.13 for `gfx950` on 8 AMD Instinct MI355X GPUs.
Do not mix the exporter build headers, runtime root, or labels from different
TheRock builds.

## Prepare the runtime root

Set `THEROCK_ROCM_ROOT` to the root of the extracted TheRock distribution. The
destination must not already exist because the script rejects a non-empty root
rather than mixing files from two builds.

```bash
make prepare-runtime \
  THEROCK_ROCM_ROOT=/path/to/therock-rocm
```

The generated `runtime-root/` contains only final-image runtime material. It
excludes headers, static libraries, the HIP compiler, CMake/pkg-config metadata,
tests, and debug symbols. It intentionally retains the profiling closure:
`librdc_rocp`, rocprofiler-sdk, COMGR, and the LLVM runtime are required for
`RDC_FI_PROF_*` fields and account for most of the image size.

The root also contains the `rocm-smi` Python entry point and
`librocm_smi64`. The script rewrites its `#!/usr/bin/env python3` shebang to
`#!/usr/bin/python3` because distroless has no `/usr/bin/env`.

## Build the exporter and image

The CGO directives currently expect RDC under `/opt/rocm`. Run the following on
a compatible build host where the selected TheRock distribution is available at
that path:

```bash
make build

make image \
  ROCM_VERSION=7.13.0 \
  ROCM_ARCHS=gfx950 \
  THEROCK_COMMIT=<full-therock-commit>
```

`make image` places `bin/rdc-exporter` and `runtime-root/` into the distroless
image and records both source commits as OCI labels. Override `IMAGE_TAG` when
publishing to another registry.

The distroless base and Debian native-runtime builder are pinned by digest in
the Dockerfile. Update those pins deliberately and repeat the GPU verification
when changing them.

## Static image verification

```bash
make image-verify \
  ROCM_VERSION=7.13.0 \
  ROCM_ARCHS=gfx950 \
  THEROCK_COMMIT=<full-therock-commit>
```

The verifier exports the image without starting it and checks for the exporter,
`rocm-smi`, RDC bootstrap, the ROCP plugin, and rocprofiler metrics data. This
does not replace a GPU runtime test.

## GPU runtime verification

Run the image with the device nodes used by RDC:

```bash
docker run -d --rm --name rdc-exporter \
  --device=/dev/kfd \
  --device=/dev/dri \
  --cap-add SYS_PTRACE \
  -p 5000:5000 \
  <image>

curl localhost:5000/metrics
docker exec rdc-exporter rocm-smi --showproductname --showuse
```

The default field list contains 10 telemetry fields and six profiling fields,
including `RDC_FI_PROF_SM_ACTIVE`. On the MI355X baseline it produced 128
samples across eight GPUs, and `SM_ACTIVE` became non-zero under a VALU workload.

Profiling fields consume hardware performance-monitor counters. The runtime
supports other `RDC_FI_PROF_*` fields, but adding too many to one field group can
trigger `Could not create PMC packets` with AQLProfile return code 4096. Add
counters gradually and monitor freshness as described in
[`docs/issues/0001-profiling-fields-pmc-packet-overflow.md`](../issues/0001-profiling-fields-pmc-packet-overflow.md).

## Size baseline

Docker's local unpacked size for the tested images was:

| Variant | Size |
| --- | ---: |
| `base-debian13`, telemetry runtime, no `rocm-smi` | 146.47 MiB |
| `python3-debian13`, telemetry runtime with `rocm-smi` | 200.21 MiB |
| Final profiling-enabled image | 566.88 MiB |

Python plus the `rocm-smi` closure added about 53.74 MiB. The additional
366.67 MiB in the final image is primarily the COMGR/LLVM/rocprofiler runtime
needed to keep profiling metrics available.
