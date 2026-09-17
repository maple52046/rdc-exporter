---
name: build-docker-image
description: >-
  Explicit-only workflow that builds and verifies the TheRock-based Debian
  13 distroless rdc-exporter image through the Makefile, using a matching
  TheRock ROCm root and commit while preserving rocm-smi and profiling.
---
# Build Docker Image

Build the TheRock-based Debian 13 distroless `rdc-exporter` image through the
repository `Makefile`. Run every command from the repository root.

## When to use

This skill is explicit-only. Use it only after the user invokes
`$build-docker-image` to build, verify, tag, or push the `rdc-exporter` image.

## Required decisions and inputs

Resolve these values before building; do not infer a release or commit:

- `THEROCK_ROCM_ROOT`: extracted TheRock ROCm distribution with RDC.
- `THEROCK_COMMIT`: full commit that produced that distribution.
- `ROCM_VERSION` and `ROCM_ARCHS`: defaults are `7.14.1` and `all-gpu`; confirm
  that they describe the supplied distribution.
- Optional `IMAGE_TAG`; otherwise the Makefile derives the GHCR tag.

The validated ROCm 7.14.1 baseline is the all-GPU artifact with TheRock commit
`f51dc6c91e0d3214f22853fd5cb3f96dbc7d2c4b`, artifact SHA-256
`0be3665633164f78c2d82fc13e9d105d2bdb87dd323f4be295946426260a3fef`,
and patched `librdc.so.1.3` SHA-256
`1096eafa7df169aa60c700954a68bd4a7f6c6aac4e99dcbc4f4a72a78c2abe74`.
Do not substitute an unpatched 7.14 artifact.

The exporter binary must be compiled against the same RDC headers and libraries
that supply the final runtime. Its cgo directives currently require that build
to be available at `/opt/rocm`.

Read [`docs/building/distroless.md`](../../../docs/building/distroless.md) before
changing the Dockerfile, runtime closure, base digest, or profiling field set.
Read [`docs/building/minimal-therock-rdc.md`](../../../docs/building/minimal-therock-rdc.md)
when producing the TheRock input from source.

## Invocation

```text
$build-docker-image [--date <YYYYMMDD>] [--no-verify] [--push] [--tag <full-tag>]
```

- `--date`: pass `BUILD_DATE=<date>` to `make`.
- `--no-verify`: skip static image-content verification.
- `--tag`: pass `IMAGE_TAG=<full-tag>` only when explicitly supplied.
- `--push`: push only after build and requested verification succeed.

## Procedure

1. Resolve the image tag:

   ```bash
   make -s print-image $MAKE_VARS
   ```

2. Prepare `runtime-root/` when it does not exist:

   ```bash
   make prepare-runtime \
     THEROCK_ROCM_ROOT="$THEROCK_ROCM_ROOT"
   ```

   Never reuse a runtime root from another TheRock build. The preparation script
   rejects non-empty destinations; move or remove the old generated root only
   after confirming its exact path.

3. Ensure `bin/rdc-exporter` was built against the same TheRock distribution.
   Run `make build` inside the matching build environment where that distribution
   is exposed at `/opt/rocm`; the standalone source-build guide gives the full
   Debian 13 container workflow. Do not compile against one RDC release and
   package another.

4. Build the image:

   ```bash
   make image \
     THEROCK_COMMIT="$THEROCK_COMMIT" \
     ROCM_VERSION="$ROCM_VERSION" \
     ROCM_ARCHS="$ROCM_ARCHS" \
     $MAKE_VARS
   ```

5. Unless `--no-verify` is set, verify required paths without expecting a shell
   in the distroless image:

   ```bash
   ./scripts/verify-distroless-image.sh "$TAG"
   ```

6. When a GPU node is available, run both functional checks. Mount `/dev/kfd`
   and `/dev/dri`, start the exporter, confirm fresh `/metrics`, and then run:

   ```bash
   docker exec rdc-exporter rocm-smi --showproductname --showuse
   ```

   Confirm `RDC_FI_PROF_SM_ACTIVE` exists and becomes non-zero under a suitable
   GPU workload. Watch logs for `Could not create PMC packets` or AQLProfile
   return code 4096. Do not silently reduce a release's default profiling field
   set; stop publication and report the incompatibility.

7. With `--push`, push the resolved tag only after all requested checks pass:

   ```bash
   docker push "$TAG"
   ```

8. Report the tag, image ID and local size, both source commits, GPU architecture,
   static verification result, `rocm-smi` result, metric count/freshness, and any
   PMC limitation observed.

## Important boundaries

- The Dockerfile assembles artifacts; it does not compile TheRock or download a
  ROCm package repository.
- `runtime-root/` deliberately excludes headers, static libraries, HIP compiler,
  development metadata, tests, and debug symbols.
- Do not remove `librdc_rocp`, rocprofiler-sdk, COMGR, LLVM runtime, or
  `libatomic.so.1`: they preserve `RDC_FI_PROF_*` support.
- Do not replace `python3-debian13` with `base-debian13` while direct `rocm-smi`
  execution is required.
