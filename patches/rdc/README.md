# RDC clock-index bounds patch for TheRock 7.14.1

This directory contains the exact-base backport used to build the patched ROCm
7.14.1 TheRock distribution for this project. The patch changes ROCm's RDC
source; it is not applied to the Go exporter source or to an
already-built runtime image.

## Applicability and provenance

| Item | Value |
| --- | --- |
| TheRock ref | `therock-7.14` |
| TheRock commit | `f51dc6c91e0d3214f22853fd5cb3f96dbc7d2c4b` |
| rocm-systems commit | `ca887ee80abfb82671fe1d6d8da708a713438e05` |
| Patch | [`rdc-clock-index-bounds-therock-7.14.1-ca887ee8.patch`](rdc-clock-index-bounds-therock-7.14.1-ca887ee8.patch) |
| Patch SHA-256 | `213ef61a7a564e4c9f664d48c42e6e6d4629602f926cae50a6591fba5875c299` |
| Patched source | `projects/rdc/rdc_libs/rdc/src/RdcMetricFetcherImpl.cc` |
| Validated hardware | 8× AMD Instinct `gfx942` GPUs |

Do not apply this file to an arbitrary rocm-systems revision merely because
`git apply --check` accepts its context. Confirm the complete TheRock and
rocm-systems commits first. A later source baseline may contain the upstream fix
already or may require a separately reviewed backport.

## Failure being fixed

On the validated eight-`gfx942` GPU system, `rdcd` could terminate with `SIGSEGV` when
`RDC_FI_MEM_CLOCK` (field 101) and any tested `RDC_FI_PROF_*` field were active
in the same RDC process. The two-field `101,800` watch was sufficient to
reproduce the failure.

AMD SMI's `amdsmi_frequencies_t` contract permits `current` to be
`(uint32_t)-1` (`UINT32_MAX`) when a clock domain is asleep, power-gated, or has
no current level reported by the kernel. `amdsmi_get_clk_freq()` can still
return `AMDSMI_STATUS_SUCCESS` in that state. The affected RDC code checked the
call status but then used `current` directly:

```cpp
if (value->status == AMDSMI_STATUS_SUCCESS) {
  value->value.l_int = f.frequency[f.current];
}
```

The captured crash had `num_supported == 3` and
`current == 0xffffffff`. The resulting `frequency[current]` read was out of
bounds and killed the whole RDC daemon or embedded RDC process. Profiling made
the state reliably observable in the tested multi-GPU setup, but the direct
defect is the unchecked, documented AMD SMI sentinel.

The full investigation and core-dump evidence are in
[`docs/issues/0002-mem-clock-profiling-multigpu-segfault.md`](../../docs/issues/0002-mem-clock-profiling-multigpu-segfault.md).

## Fix behavior

The patch reads `frequency[current]` only when all of these conditions hold:

- `num_supported > 0`;
- `num_supported <= AMDSMI_MAX_NUM_FREQUENCIES`;
- `current < num_supported`;
- `current < AMDSMI_MAX_NUM_FREQUENCIES`.

When AMD SMI reports success with an invalid count or index, RDC now sets the
sample status to `AMDSMI_STATUS_NO_DATA` and leaves the frequency array unread.
The following update cycle can retry normally. The change does not modify the
RDC public API, ABI, field IDs, or structures, and it deliberately avoids a new
per-sample log that could flood logs while GPUs are idle.

An idle GPU may consequently expose memory clock as unavailable/`N/A`. RDC can
also log status 13 with AMD SMI code 40 for that sample. That is the expected
safe result; `rdcd` and the exporter must remain alive and continue updating all
other fields.

## Applying the patch

Fetch the exact TheRock source and its pinned component repositories first:

```bash
git clone --branch therock-7.14 \
  https://github.com/ROCm/TheRock.git /work/TheRock

git -C /work/TheRock checkout \
  f51dc6c91e0d3214f22853fd5cb3f96dbc7d2c4b

(
  cd /work/TheRock
  python3 build_tools/fetch_sources.py --jobs 24
)

test "$(git -C /work/TheRock rev-parse HEAD)" = \
  f51dc6c91e0d3214f22853fd5cb3f96dbc7d2c4b
test "$(git -C /work/TheRock/rocm-systems rev-parse HEAD)" = \
  ca887ee80abfb82671fe1d6d8da708a713438e05
test -z "$(git -C /work/TheRock/rocm-systems status --short)"
```

Set `RDC_EXPORTER_ROOT` to this repository's absolute path, verify the patch
checksum, and apply it inside the fetched rocm-systems repository:

```bash
export RDC_EXPORTER_ROOT=/path/to/rdc-exporter
export RDC_PATCH="$RDC_EXPORTER_ROOT/patches/rdc/rdc-clock-index-bounds-therock-7.14.1-ca887ee8.patch"

printf '%s  %s\n' \
  213ef61a7a564e4c9f664d48c42e6e6d4629602f926cae50a6591fba5875c299 \
  "$RDC_PATCH" | sha256sum -c -

git -C /work/TheRock/rocm-systems apply --check "$RDC_PATCH"
git -C /work/TheRock/rocm-systems apply "$RDC_PATCH"
git -C /work/TheRock/rocm-systems diff --check
git -C /work/TheRock/rocm-systems apply --reverse --check "$RDC_PATCH"
```

The reverse check after application proves that the expected change is present.
It does not undo the patch. Continue with the minimal RDC build described in
[`docs/building/minimal-therock-rdc.md`](../../docs/building/minimal-therock-rdc.md).

## Validation requirements

Do not qualify a patched archive from build success alone. At minimum:

1. Record the TheRock commit, rocm-systems commit, patch checksum, archive
   checksum, and patched `librdc.so.1.3` checksum.
2. Check `rdci` and `rdcd` for unresolved dynamic libraries.
3. Start a fresh `rdcd` for every case; do not reuse watches between cases.
4. Run the minimal multi-GPU reproducer:

   ```bash
   rdci dmon --host localhost:<port> -u \
     -e 101,800 -i 0,1,2,3,4,5,6,7 -d 1000 -c 12 -t
   ```

5. Run the complete project field set on all eight GPUs while GPU 0 executes a
   HIP workload:

   ```text
   100,101,200,201,300,500,501,502,600,601,800,805,804,801,803,812
   ```

6. Confirm that `rdcd` remains alive, every GPU continues producing samples,
   and GPU 0 has non-zero profiling activity.
7. Check the client output, server log, and kernel log for connection refusal,
   core dumps, `librdc` segmentation faults, PMC packet failures, and
   AQLProfile return code 4096.

Do not use the `rdci` exit status as the only assertion: affected clients were
observed returning status 0 after `rdcd` had already crashed.

The patched ROCm 7.14.1 artifact passed the full 16-field test on eight
`gfx942` GPUs. Its distroless exporter candidate produced 128 GPU series;
under a sustained GPU 0 workload, `gpu_util` was 100 and `valubusy` changed
from `65.54684016745742` to `69.32468342459957` across consecutive qualified
scrapes.
