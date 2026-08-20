# Building a Minimal TheRock Distribution for RDC on Debian 13

This document describes how to build the smallest TheRock feature set intended
for `rdc-exporter`. It is self-contained: it covers the Debian 13 build
environment, TheRock source checkout, configuration, build, artifact checks,
and integration with this repository.

## Validation status

The feature selection documented here has been validated with TheRock 7.13,
`gfx950`, and Ubuntu 24.04. That build completed with 255 super-project steps in
about 16 minutes on a large build server and produced a 9.0 GiB TheRock
distribution.

The Debian 13 environment in this document is an adaptation for alignment with
the final Debian 13 distroless image. Debian 13 has been used successfully for
other TheRock 7.13 builds, but the exact combination of Debian 13 plus the
minimal-RDC recipe has **not yet been validated end to end**. Treat the commands
as the intended reproducible recipe and record any differences found during the
first build.

## What "minimal" means

The configuration starts with all optional feature groups disabled and then
enables RDC:

```text
-DTHEROCK_ENABLE_ALL=OFF
-DTHEROCK_ENABLE_RDC=ON
```

TheRock recursively enables everything declared by RDC's `REQUIRES` graph. The
expected feature closure for TheRock 7.13 is:

```text
SYSDEPS SYSDEPS_LIBMNL SYSDEPS_LIBNL BASE COMPILER CORE_AMDSMI
CORE_RUNTIME ELFIO KPACK HIP_RUNTIME AQLPROFILE ROCPROFV3 RDC
```

This closure includes the components required by the exporter at build time or
runtime:

| Component | Why it remains enabled |
| --- | --- |
| RDC and `librdc_bootstrap` | CGO API used directly by `rdc-exporter` |
| amd-smi | Clock, temperature, power, memory, utilization, and ECC telemetry |
| rocprofiler-sdk and AQLProfile | `RDC_FI_PROF_*` performance counters |
| ROCR/HSA runtime | GPU discovery and queue/runtime support |
| HIP runtime | Required by the RDC dependency graph |
| amd-comgr and AMD LLVM runtime | Runtime code-object and profiling support |
| bundled sysdeps | Portable libdrm, libdw, NUMA, compression, and related closure |

Math, ML, and communication libraries such as rocBLAS, hipBLASLt, MIOpen,
Composable Kernel, RCCL, rocRAND, and rocFFT remain disabled.

"Minimal" therefore means the minimum supported **TheRock feature graph** for
RDC. It does not mean the build avoids compiling AMD LLVM, HIP, headers, static
libraries, or development artifacts. Those are still produced because RDC's
TheRock graph requires them. The final distroless image is reduced separately by
copying only the runtime closure with `scripts/prepare-runtime-root.sh`.

TheRock builds the ROCm user-space stack only. It does not build the host
`amdgpu` kernel module, KFD, DRM device nodes, or GPU firmware. Runtime testing
requires a host that already exposes `/dev/kfd` and `/dev/dri`.

## 1. Create the Debian 13 builder image

Run the following from the `rdc-exporter` repository root. The inline Dockerfile
keeps this procedure independent of any external build-environment repository.

```bash
docker build -t therock-rdc-builder:debian13 - <<'EOF'
FROM debian:13

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      automake \
      binutils \
      bison \
      build-essential \
      ca-certificates \
      ccache \
      cmake \
      curl \
      file \
      flex \
      g++ \
      gfortran \
      git \
      golang-go \
      libatomic1 \
      libegl-dev \
      libnuma-dev \
      libtool \
      make \
      ninja-build \
      patchelf \
      pigz \
      pkg-config \
      python3-dev \
      python3-venv \
      rsync \
      texinfo \
      wget \
      xxd \
      xz-utils && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /work
CMD ["/bin/bash"]
EOF
```

The Debian-specific package name is `libegl-dev`; older Ubuntu instructions may
use `libegl1-mesa-dev`. Debian 13 provides GCC 14, glibc 2.41, CMake, Ninja,
Python 3.13, and ccache through APT.

For a reproducible release builder, pin `debian:13` to a reviewed digest and
record that digest in the build log.

## 2. Start the build container

Keep sources and build output on a host-mounted path so they survive container
replacement. The repository is mounted separately so the resulting TheRock
distribution can be used to compile and package `rdc-exporter`.

```bash
export THEROCK_WORK="$PWD/therock-rdc-work"
mkdir -p "$THEROCK_WORK"

docker run --rm -it \
  --name therock-rdc-build \
  --shm-size=32g \
  --cap-add=SYS_PTRACE \
  --security-opt seccomp=unconfined \
  -v "$THEROCK_WORK:/work" \
  -v "$PWD:/src/rdc-exporter" \
  therock-rdc-builder:debian13
```

Compilation itself does not require a GPU. To perform GPU checks in the same
container, also pass:

```text
--device=/dev/kfd --device=/dev/dri
```

All remaining commands in sections 3 through 7 run inside the container.

## 3. Fetch TheRock 7.13 sources

Pin a release tag or full commit. Do not build an unrecorded moving branch.

```bash
export THEROCK_REF=therock-7.13

git clone --depth 1 --branch "$THEROCK_REF" \
  https://github.com/ROCm/TheRock.git /work/TheRock
cd /work/TheRock

git rev-parse HEAD | tee /work/therock.commit
python3 build_tools/fetch_sources.py --jobs 12
```

`fetch_sources.py` retrieves the nested LLVM, ROCm libraries, ROCm systems, and
IREE sources and applies patches carried by TheRock. Those upstream patches are
part of the normal build and are not local source modifications. Source fetches
can consume tens of gigabytes and take 30–60 minutes depending on the network.

Before making local changes, verify the checkout is clean:

```bash
git status --short
```

The normal strategy is to complete the build using packages, environment
variables, and CMake options only. Patch TheRock or a subproject only after the
same root cause has survived multiple distinct configuration/environment fixes.

## 4. Prepare patchelf and Python

TheRock bundles portable system dependencies by default and requires its pinned
`patchelf` behavior. Install the version supplied by the selected TheRock source
instead of relying only on Debian's package:

```bash
cd /work/TheRock
env INSTALL_PREFIX=/usr/local ./dockerfiles/install_pinned_patchelf.sh
patchelf --version
```

Create the Python environment and install TheRock's pinned requirements:

```bash
python3 -m venv /work/.venv
. /work/.venv/bin/activate
python -m pip install --upgrade pip
python -m pip install -r /work/TheRock/requirements.txt
```

TheRock 7.13 accepts the Debian 13 Python environment. TheRock 7.14 and later
may impose different Python ABI requirements; inspect the selected tag's
official build container before changing `THEROCK_REF`.

## 5. Configure the minimal RDC feature set

Configure a clean build tree. The example targets `gfx950` for MI350X/MI355X.

```bash
cd /work/TheRock
. /work/.venv/bin/activate

eval "$(python build_tools/setup_ccache.py)"
export CCACHE_SLOPPINESS=include_file_ctime
export CCACHE_MAXSIZE=80G

cmake -B build -S . -GNinja \
  -DCMAKE_BUILD_TYPE=RelWithDebInfo \
  -Damd-llvm_BUILD_TYPE=Release \
  -Dtherock-host-blas_BUILD_TYPE=Release \
  -Dtherock-SuiteSparse_BUILD_TYPE=Release \
  -DTHEROCK_ENABLE_ALL=OFF \
  -DTHEROCK_ENABLE_RDC=ON \
  -DTHEROCK_SPLIT_DEBUG_INFO=OFF \
  -DTHEROCK_MINIMAL_DEBUG_INFO=OFF \
  -DTHEROCK_QUIET_INSTALL=OFF \
  -DCMAKE_C_COMPILER_LAUNCHER=ccache \
  -DCMAKE_CXX_COMPILER_LAUNCHER=ccache \
  -DBUILD_TESTING=OFF \
  -DTHEROCK_AMDGPU_FAMILIES=gfx950-dcgpu
```

Important choices:

- Use semicolons, not commas, when selecting multiple GPU families.
- A single family does not require `THEROCK_AMDGPU_DIST_BUNDLE_NAME`.
- For multiple families, set an explicit transparent bundle name, for example:

  ```text
  -DTHEROCK_AMDGPU_FAMILIES="gfx94X-dcgpu;gfx950-dcgpu"
  -DTHEROCK_AMDGPU_DIST_BUNDLE_NAME="gfx942-gfx950"
  ```

- `BUILD_TESTING=OFF` prevents unrelated test targets from expanding the build.
- `THEROCK_MINIMAL_DEBUG_INFO=OFF` preserves the expected optimized
  `RelWithDebInfo` flags; the option's name is easy to misread.
- `THEROCK_SPLIT_DEBUG_INFO=OFF` avoids split-debug/kpack failures and must be
  selected before the first build. Do not reuse a build tree previously created
  with this option enabled.
- `THEROCK_BUNDLE_SYSDEPS` remains enabled by default so the resulting ROCm
  distribution carries its portable system-library closure.

Review the configure output and confirm that the enabled-feature list contains
RDC, ROCPROFV3, AQLPROFILE, HIP_RUNTIME, CORE_RUNTIME, CORE_AMDSMI, COMPILER,
and SYSDEPS without the math/ML/communication stacks.

## 6. Build

Choose parallelism according to available memory. `-j 128` was safe on a host
with hundreds of CPU threads and ample RAM; use a lower value on smaller hosts.

```bash
cd /work/TheRock
. /work/.venv/bin/activate

eval "$(python build_tools/setup_ccache.py)" >/dev/null 2>&1 || true
export CCACHE_SLOPPINESS=include_file_ctime
export CCACHE_MAXSIZE=80G

set -o pipefail
cmake --build build -j 128 2>&1 | tee /work/therock-rdc-build.log
```

Ninja's fatal entries begin with `FAILED:`. Check them explicitly after the
build:

```bash
grep -c '^FAILED:' /work/therock-rdc-build.log
```

A successful command exit plus a count of zero is the expected result. Warnings
from localization files, generated library versions, or nanobind leak reporting
are not necessarily fatal; evaluate the failed Ninja target rather than matching
the word `error` indiscriminately.

## 7. Validate the TheRock distribution

The assembled distribution is normally under `build/dist/rocm`:

```bash
export ROCM_DIST=/work/TheRock/build/dist/rocm

cat "$ROCM_DIST/.info/version"
cat "$ROCM_DIST/share/therock/dist_info.json"

ls -l "$ROCM_DIST"/lib/librdc_bootstrap.so*
ls -l "$ROCM_DIST"/lib/librdc.so*
ls -l "$ROCM_DIST"/lib/rdc/librdc_rocp.so*
ls -l "$ROCM_DIST"/lib/librocprofiler-sdk.so*
ls -l "$ROCM_DIST"/lib/libamd_smi.so*
ls -l "$ROCM_DIST"/lib/libamd_comgr.so*
ls -l "$ROCM_DIST"/lib/libhsa-runtime64.so*
ls -l "$ROCM_DIST"/bin/rocm-smi
```

Confirm the selected GPU target and the highest glibc symbol requirement:

```bash
python - <<'PY'
import json
from pathlib import Path

info = json.loads(
    Path("/work/TheRock/build/dist/rocm/share/therock/dist_info.json").read_text()
)
print(info["dist_amdgpu_targets"])
PY

objdump -T "$ROCM_DIST"/lib/libhsa-runtime64.so.* \
  | grep -oE 'GLIBC_[0-9.]+' \
  | sort -V \
  | tail -1
```

Do not assume the required glibc version equals Debian 13's glibc 2.41. TheRock
bundled sysdeps can reduce the external requirement. Measure the produced
libraries and record the result with the artifact.

If the container has GPU devices, verify user-space access:

```bash
export LD_LIBRARY_PATH="$ROCM_DIST/lib:$ROCM_DIST/lib/rdc:$ROCM_DIST/lib/llvm/lib:$ROCM_DIST/lib/rocm_sysdeps/lib"

"$ROCM_DIST/bin/rocm-smi" --showproductname --showuse
"$ROCM_DIST/lib/llvm/bin/amdgpu-arch"
```

## 8. Build and test rdc-exporter against this distribution

The project's CGO directives currently use `/opt/rocm`. Point that path at the
new distribution, then build from the repository mounted earlier:

```bash
ln -sfn "$ROCM_DIST" /opt/rocm

cd /src/rdc-exporter
go version
make build

ldd bin/rdc-exporter
```

The Go toolchain must satisfy the version declared in `go.mod`.

For a direct GPU smoke test:

```bash
export LD_LIBRARY_PATH="/opt/rocm/lib:/opt/rocm/lib/rdc:/opt/rocm/lib/llvm/lib:/opt/rocm/lib/rocm_sysdeps/lib"

./bin/rdc-exporter
```

In another terminal, query `http://localhost:5000/metrics`. Confirm that values
change over time rather than merely checking that HTTP remains alive.

## 9. Assemble the distroless image

Exit the builder container after the source build. From the repository root on
the host, prepare the final runtime root and build the image:

```bash
make prepare-runtime \
  THEROCK_ROCM_ROOT="$THEROCK_WORK/TheRock/build/dist/rocm"

make image \
  ROCM_VERSION=7.13.0 \
  ROCM_ARCHS=gfx950 \
  THEROCK_COMMIT="$(cat "$THEROCK_WORK/therock.commit")"
```

`prepare-runtime` intentionally keeps `librdc_rocp`, rocprofiler-sdk, COMGR,
LLVM runtime, HSA, amd-smi, and `libatomic.so.1`. It removes development-only
material such as headers, static libraries, the HIP compiler, CMake/pkg-config
metadata, tests, and debug symbols from the final image.

Run the static image verifier:

```bash
make image-verify \
  ROCM_VERSION=7.13.0 \
  ROCM_ARCHS=gfx950 \
  THEROCK_COMMIT="$(cat "$THEROCK_WORK/therock.commit")"
```

Resolve the image tag, start it on a GPU host, and verify both the exporter and
direct `rocm-smi` execution:

```bash
export IMAGE_TAG="$(make -s print-image ROCM_VERSION=7.13.0)"

docker run -d --rm \
  --name rdc-exporter-therock \
  --privileged \
  -p 5000:5000 \
  "$IMAGE_TAG"

curl --fail --silent --show-error http://127.0.0.1:5000/metrics
docker exec rdc-exporter-therock rocm-smi --showproductname --showuse
docker logs rdc-exporter-therock
```

Confirm all expected GPUs are present, profiling initialization succeeds, and
the logs do not contain a PMC packet fatal. Scrape `/metrics` more than once and
verify that values continue changing. Remove the test container when finished:

```bash
docker rm --force rdc-exporter-therock
```

## Profiling and PMC limits

Keeping the profiling runtime is intentional. Do not remove the ROCP plugin or
`libatomic.so.1` to avoid profiling failures; doing so silently makes important
metrics such as `RDC_FI_PROF_SM_ACTIVE` unavailable.

The number of profiling counters that one GPU can collect in a single PMC
packet is hardware-dependent. A large `RDC_FI_PROF_*` field group can cause:

```text
Could not create PMC packets! AQLProfile Return Code: 4096
```

The project's default set uses 10 telemetry fields and six profiling fields,
including `RDC_FI_PROF_SM_ACTIVE`. This set was stable on the 8-GPU MI355X
baseline. Add other profiling fields gradually and monitor metric freshness;
an HTTP 200 response alone does not prove that collection is still advancing.

Failure to load `librdc_rvs.so` is expected when RVS is not included and does not
prevent telemetry or profiling collection.

## Troubleshooting checklist

### Unknown AMDGPU family

```text
THEROCK_AMDGPU_FAMILIES value 'gfx94X-dcgpu,gfx950-dcgpu' unknown
```

Use a semicolon-separated CMake list instead of a comma-separated string.

### Missing bundle name

```text
THEROCK_AMDGPU_DIST_BUNDLE_NAME must be set explicitly
```

Set an explicit bundle name whenever more than one family is selected.

### Split-debug `fatbin.o` failure

Configure with `-DTHEROCK_SPLIT_DEBUG_INFO=OFF` in a new build tree. Changing
the option in a partially built tree can leave stale debug artifacts and produce
the same failure again.

### `-latomic` link failure

The Debian builder installs `libatomic1` and GCC through `build-essential`. If a
link still reports `unable to find library -latomic`, confirm the compiler's
development library path and that the build is using Debian's GCC toolchain.

### Reconfiguring after a global feature change

Do not rely on a partially populated build tree when changing options that alter
feature, install, or stage behavior. Preserve ccache, remove the `build/`
directory, and configure again to avoid conclusions based on stale artifacts.

### Exporter starts but metrics freeze

Inspect the exporter log for AQLProfile RC 4096 and reduce the simultaneous
profiling field count. Verify freshness by comparing samples across multiple
scrapes rather than checking process state only.

## Reproducibility record

Record at least the following with every build:

- Debian 13 builder image digest.
- TheRock tag and full commit.
- `THEROCK_AMDGPU_FAMILIES` and bundle name, if any.
- Complete CMake command and build parallelism.
- `build/dist/rocm/.info/version`.
- `dist_amdgpu_targets` from `share/therock/dist_info.json`.
- Highest measured glibc symbol requirement.
- TheRock distribution size and final image size.
- `rdc-exporter` commit.
- GPU model/count, `/metrics` freshness, `rocm-smi` result, and profiling field set.
- Any PMC/AQLProfile warning or fatal result.
