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

The ROCm 10.0.0 release input is a patched all-GPU TheRock distribution. Runtime
validation was performed on eight `gfx942` GPUs. Do not mix the
exporter build headers, runtime root, or labels from different TheRock builds.

The validated artifact is:

- Build-host path: `/dockerdata/tasks/149-build-the-rock/10.0/delivery/rocm10.0.0-all-gpu-minimal-rdc-clock-index-fix-no-emulation-glibc2.38.tar.gz`
- SHA-256: `a1e075511bb479e9baae21abc6786d52a60bf7dc0c7a2946c64f891a43179788`
- TheRock commit: `16adc4d875fd4f65ea23c7c84e1c66706fde3047`
- rocm-systems commit: `6b0e43f341195e203754e08f850e437ff2fc09f9`
- RDC clock-index patch SHA-256: `bd6f313b66a8e8ccf276e1a33c0d12dcd04d8c9e6d1866239b8712511619519a`
- Repository patch: [`patches/rdc/rdc-clock-index-bounds-therock-10.0-6b0e43f3.patch`](../../patches/rdc/rdc-clock-index-bounds-therock-10.0-6b0e43f3.patch)
- Patched `librdc.so.1.3` SHA-256: `7c764ab6c23de64ff45e88c9b65be5073502e889192626c4e9679496a065402f`
- Maximum measured glibc requirement: `GLIBC_2.38`
- GPU emulation support: disabled

Its `share/therock/dist_info.json` records 28 GPU targets. The compact
`ROCM_ARCHS=all-gpu` image label refers to that complete target set; it does not
change the container platform from `linux/amd64`.

The artifact includes the RDC clock-index bounds fix documented in
[`docs/issues/0002-mem-clock-profiling-multigpu-segfault.md`](../issues/0002-mem-clock-profiling-multigpu-segfault.md).
Native RDC and the final image both completed the combined default set on all
eight GPUs. Native RDC produced non-zero, changing `RDC_FI_PROF_SM_ACTIVE`
values under load; final-image measurements are recorded below. Idle
memory-clock samples can be reported as unavailable, but they no longer
terminate RDC.

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
  ROCM_VERSION=10.0.0 \
  ROCM_ARCHS=all-gpu \
  THEROCK_COMMIT=<full-therock-commit>
```

`make image` places `bin/rdc-exporter` and `runtime-root/` into the distroless
image and records both source commits as OCI labels. Override `IMAGE_TAG` when
publishing to another registry.

The distroless base and Debian native-runtime builder are pinned by digest in
the Dockerfile. Update those pins deliberately and repeat the GPU verification
when changing them.

During the 2026-09-17 ROCm 10.0.0 qualification, the build host could not reach
the Debian package mirror used by the pinned native-runtime stage. The release
candidate therefore reused the three native runtime files previously produced
from that same pinned Debian base. Their content was byte-identical in the
published ROCm 7.14.0 and 7.14.1 images:

| File | SHA-256 |
| --- | --- |
| `libatomic.so.1` | `9558489f171274c220104894258f483055853ed8a23d4ff754c850ce55765aa2` |
| `libstdc++.so.6` | `972bb2a18b71140dab0240f8a1f68ab3fb1d56bcd4c4f824a91b70888faf5a00` |
| `libgcc_s.so.1` | `30c61ab012a4241bed033725a09b61f5fdd3bb7df95ee852d0b096520524c7af` |

The repository Dockerfile and its pinned base digests were not changed. A
normal connected rebuild should continue to use `make image`; the offline
assembly was a release-host network workaround, not a new build contract.

## Static image verification

```bash
make image-verify \
  ROCM_VERSION=10.0.0 \
  ROCM_ARCHS=all-gpu \
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
including `RDC_FI_PROF_SM_ACTIVE`. On the ROCm 10.0.0 MI308X (`gfx942`)
baseline, native RDC completed 20 rounds for every GPU with the full field set.
Under a sustained GPU 0 VALU workload, native `GPU_UTIL` was 100 and native
`VALUBusy` ranged from 21.331% to 29.652% while the workload was active.

The candidate image produced 128 samples (16 fields by eight GPUs) on every
qualified scrape, and image-local `rocm-smi` found all eight GPUs. Across two
RDC update windows under load, GPU 0 `valubusy` changed from
`29.28883367251129` to `29.50381430223124`, while `active_cycles` changed from
`57,122,976` to `57,172,185` and `gpu_util` remained 100. The default watch
period is 10 seconds, so two HTTP scrapes inside one period can legitimately
show the same cached sample.

Profiling fields consume hardware performance-monitor counters. The runtime
supports other `RDC_FI_PROF_*` fields, but adding too many to one field group can
trigger `Could not create PMC packets` with AQLProfile return code 4096. Add
counters gradually and monitor freshness as described in
[`docs/issues/0001-profiling-fields-pmc-packet-overflow.md`](../issues/0001-profiling-fields-pmc-packet-overflow.md).

## Size baseline

Docker's local unpacked size for the historical ROCm 7.13 images was:

| Variant | Size |
| --- | ---: |
| `base-debian13`, telemetry runtime, no `rocm-smi` | 146.47 MiB |
| `python3-debian13`, telemetry runtime with `rocm-smi` | 200.21 MiB |
| Final profiling-enabled image | 566.88 MiB |

Python plus the `rocm-smi` closure added about 53.74 MiB. The additional
366.67 MiB in the final image is primarily the COMGR/LLVM/rocprofiler runtime
needed to keep profiling metrics available.

The validated patched ROCm 10.0.0 all-GPU candidate was 211,029,557 bytes
(201.25 MiB) by Docker's local size report and is published as
`ghcr.io/maple52046/rdc-exporter:v1-rocm10.0.0-20260917`.
