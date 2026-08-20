# Issue 0001 — Too many profiling fields overflow the GPU PMC packet and silently wedge collection

- **Status:** Open
- **Severity:** High (default configuration is affected)
- **Component:** RDC profiling path (`RDC_FI_PROF_*`) via rocprofiler-sdk / AQLProfile
- **Affected:** rdc-exporter default field set on AMD Instinct MI355X (gfx950), ROCm 7.2.2
  image (`ghcr.io/maple52046/rdc-exporter:v1-rocm7.2.2-*`, verified 20260429 and 20260609)

## Summary
When rdc-exporter scrapes the **default** field set (which contains ~18 `RDC_FI_PROF_*`
profiling fields), the RDC profiling layer aborts with a fatal error while trying to build the
GPU performance-monitor-counter (PMC) packet. The abort happens inside an RDC worker thread, so
the process does **not** exit: the HTTP server keeps serving the **last (stale) snapshot**, and
the pod stays `Running` with `RESTARTS=0`. The exporter is effectively dead but looks healthy.

## Environment
- GPU: 8× AMD Instinct MI355X (gfx950)
- ROCm: 7.2.2 (rocprofiler-sdk `v1.1.0`), image base ubuntu24.04
- Orchestration: k0s (cri-dockerd); also reproduced with plain `docker run`
- Container: `privileged: true`, `SYS_PTRACE` added

## Symptom
Logs (appears ~11 s after start, then logging stops entirely):

```
... ERROR RdcRocpBase.cc(463): Error: Metric ValuPipeIssueUtil not found in sampled values.   # repeated
F<ts> aql_packet.cpp:136] Could not create PMC packets! AQLProfile Return Code: 4096
  Events: [0,0,3],[0,2,3],[0,13,6],[0,27,6],[0,29,6],[0,52,6],[0,51,6],[0,28,6],[0,30,6],[0,5,6],
  @ ... amd::rdc::CounterSampler::sample_counters_with_packing(...)
  @ ... amd::rdc::RdcRocpBase::rocp_lookup_bulk(...)
  @ ... rdc_telemetry_fields_value_get
  @ ... amd::rdc::RdcWatchTableImpl::rdc_field_update_all()
  @ ... std::__future_base::_State_baseV2::_M_do_set(...)        # runs in a worker thread
```

Observable effects:
- `GET /metrics` returns HTTP 200 but two scrapes seconds apart are **byte-identical** (frozen).
- Pod is `1/1 Running`, `RESTARTS=0` — Kubernetes cannot detect the wedge.

## Root cause
The default field set asks for ~18 `RDC_FI_PROF_*` fields. RDC packs the corresponding PMC
counters into a **single** profiling packet via
`CounterSampler::sample_counters_with_packing`. On gfx950 the number of counters that fit in one
PMC packet is limited; the default set exceeds it and AQLProfile fails packet creation
(`Return Code: 4096`) and calls `glog` `FATAL`. Because the failure occurs in an RDC
`std::async` worker (not the main goroutine/thread), the Go process keeps running while
collection never advances again.

This is **a counter-count / packing limit, not a permissions problem** (see "Ruled out" below).

## Evidence / threshold (counter count)
Measured on the same host/image (`-e`/`-f` to control the field list), idle GPUs, no contention
(`rocm-smi --showpids` = no KFD PIDs):

| profiling fields requested | result |
| --- | --- |
| 0 (telemetry only)        | OK — no fatal, values keep updating |
| 2                          | OK — no fatal, collection alive |
| 6                          | OK — no fatal, collection alive (values update under load) |
| ~18 (full default)         | FATAL — `Could not create PMC packets (RC 4096)`, collection frozen |

So the safe ceiling on this GPU is between 6 and ~18 simultaneous profiling counters.

## Ruled out (things that do NOT fix it)
- **Adding `SYS_PERFMON` capability:** the container is already `privileged`, so `CapEff`
  already includes `CAP_PERFMON` (bit 38); adding it explicitly is a no-op.
- **Lowering host `kernel.perf_event_paranoid` (tried `-1`):** clears the separate
  `could not be locked for profiling (capability SYS_PERFMON)` warning for *small* counter sets,
  but the full-set PMC packet fatal still occurs.
- **Different exporter build date** (20260429 vs 20260609): identical behavior — the fault is in
  the ROCm 7.2.2 rocprofiler/AQLProfile layer on gfx950, not the Go code revision.

## Impact
- Default deployment looks healthy but stops collecting within ~10–15 s.
- Because one bad packet aborts the whole scrape, it also kills the *telemetry* metrics that
  would otherwise be fine.

## Workaround
Limit the field list so the profiling count stays under the GPU's PMC packet ceiling
(telemetry + a small profiling subset). Provide fields via `-e` or, preferably, a file with `-f`
(e.g. a mounted ConfigMap — see `example/rdc-exporter-daemonset.yml`). Validated stable set:
10 telemetry + 6 profiling
(`RDC_FI_PROF_OCCUPANCY_PERCENT, RDC_FI_PROF_GPU_UTIL_PERCENT, RDC_FI_PROF_TENSOR_ACTIVE_PERCENT,
RDC_FI_PROF_ACTIVE_CYCLES, RDC_FI_PROF_ELAPSED_CYCLES, RDC_FI_PROF_SM_ACTIVE`).

## Suggested fixes (code)
1. **Multi-pass / batched PMC sampling:** split profiling counters across multiple packets
   (multiple passes) instead of one packet, so large field sets work on counter-limited GPUs.
2. **Cap counters per packet** based on a probe of the device's capacity, and surface a clear
   error for fields that don't fit instead of aborting.
3. **Don't let a worker-thread abort masquerade as healthy:** add a staleness/liveness signal
   (e.g. track "last successful collection time"; make `/metrics` or a `/healthz` fail when stale)
   so a Kubernetes liveness probe can restart a wedged exporter.
4. **Handle AQLProfile failure gracefully:** treat `Could not create PMC packets` as a recoverable
   per-field error (drop/skip the offending fields) rather than a process-fatal.

## References
- Deployment write-up: `tasks/4-deploy-rdc-exporter/DEPLOY_LOG.md`
- Knowledge note: `knowledge/ROCm/rdc-exporter/README.md`
