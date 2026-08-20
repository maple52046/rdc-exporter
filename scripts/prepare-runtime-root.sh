#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <therock-rocm-root> <empty-runtime-root>" >&2
  exit 2
fi

ROCM_ROOT=$(cd "$1" && pwd)
RUNTIME_ROOT=$2

if [ ! -f "$ROCM_ROOT/.info/version" ]; then
  echo "not a TheRock ROCm root: $ROCM_ROOT" >&2
  exit 1
fi

mkdir -p "$RUNTIME_ROOT"
RUNTIME_ROOT=$(cd "$RUNTIME_ROOT" && pwd)
if find "$RUNTIME_ROOT" -mindepth 1 -print -quit | grep -q .; then
  echo "runtime root must be empty: $RUNTIME_ROOT" >&2
  exit 1
fi

DEST="$RUNTIME_ROOT/opt/rocm"
mkdir -p "$DEST/bin" "$DEST/lib/llvm/lib" "$DEST/lib/rdc" "$DEST/libexec" "$DEST/share"

# Directly loaded by rdc-exporter / librdc / librdc_rocr. Copy each SONAME
# symlink chain as well as the real file.
cp -a "$ROCM_ROOT"/lib/librdc_bootstrap.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/librdc.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/libamd_smi.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/libamd_comgr.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/libhsa-runtime64.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/librocprofiler-register.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/librocprofiler-sdk.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/librocm_smi64.so* "$DEST/lib/"
cp -a "$ROCM_ROOT"/lib/llvm/lib/libclang-cpp.so* "$DEST/lib/llvm/lib/"
cp -a "$ROCM_ROOT"/lib/llvm/lib/libLLVM.so* "$DEST/lib/llvm/lib/"
cp -a "$ROCM_ROOT"/lib/rdc/librdc_rocp.so* "$DEST/lib/rdc/"
cp -a "$ROCM_ROOT"/lib/rdc/librdc_rocr.so* "$DEST/lib/rdc/"

# rocm-smi is a Python entry point backed by librocm_smi64. The distroless
# Python image has Python but no /usr/bin/env, so make the copied entry point
# invoke the interpreter directly.
cp -a "$ROCM_ROOT/bin/rocm-smi" "$DEST/bin/"
cp -a "$ROCM_ROOT/libexec/rocm_smi" "$DEST/libexec/"
sed -i '1s|^#!/usr/bin/env python3$|#!/usr/bin/python3|' \
  "$DEST/libexec/rocm_smi/rocm_smi.py"

# Profiling support is intentional. The image keeps the ROCP/rocprofiler/COMGR
# runtime closure so callers can select any RDC_FI_PROF_* field. The default
# field list remains conservative because requesting too many PMC counters in
# one packet can make AQLProfile fail fatally on counter-limited GPUs.

# TheRock's portable sysdeps closure is small and version-dependent. Copying it
# as a unit is safer than hard-coding its transitive SONAME set.
cp -a "$ROCM_ROOT/lib/rocm_sysdeps" "$DEST/lib/"

# Runtime configuration, licenses, and provenance.
cp -a "$ROCM_ROOT/.info" "$DEST/"
cp -a "$ROCM_ROOT/share/doc" "$DEST/share/"
cp -a "$ROCM_ROOT/share/rdc" "$DEST/share/"
cp -a "$ROCM_ROOT/share/rocprofiler-sdk" "$DEST/share/"
cp -a "$ROCM_ROOT/share/therock" "$DEST/share/"

du -sh "$DEST"
