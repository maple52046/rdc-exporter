# Issue 0002 — MEM_CLOCK 與 profiling counters 在多 GPU 監控時造成 RDC 越界存取與 SIGSEGV

- **Status:** Fixed in the validated patched TheRock artifact; upstream merge status not tracked here
- **Severity:** Critical（預設 16 fields 與 8-GPU 發布驗收受影響）
- **Component:** RDC telemetry path、AMD SMI clock-frequency API、RDC/rocprofiler 共存路徑
- **Affected fields:** `RDC_FI_MEM_CLOCK`（101）與任一已測試的 `RDC_FI_PROF_*` field
- **Affected hardware:** 8× AMD Instinct `gfx942`（tainan-ci）
- **Affected ROCm builds:** 已在 ROCm 7.14.0、7.14.1、10.0.0 與 AMD RC repository 的
  10.1.0rc0 TheRock artifact 重現
- **First investigated:** 2026-09-16
- **Release impact:** 原始 artifact 阻擋發布；patched ROCm 7.14.0、7.14.1 與 10.0.0 artifacts 已通過 release runtime gate

## Executive summary

rdc-exporter 的發布候選映像在 8 張 `gfx942` GPU 上監控預設 10 個 telemetry fields
與 6 個 profiling fields 時無法可靠更新 metrics。調查一開始需要區分四種可能：

1. rdc-exporter 的 Go/cgo RDC binding 已不相容。
2. minimal TheRock build 缺少 RDC 所需元件或編譯選項錯誤。
3. Distroless runtime 裁切掉必要 library、plugin、data file 或 symlink。
4. RDC/AMD SMI 本身在這個 field 與 GPU 組合下有問題。

使用官方 `rdci dmon` 與 `rdcd`、多個彼此獨立的 TheRock artifacts、完整 ROCm tree、
裁切後 runtime root、單 GPU/8 GPU、telemetry/profiling 子集合及 core dump 逐步隔離後，
目前已定位到 RDC 的未檢查陣列索引：

```cpp
amdsmi_frequencies_t f = {};
value->status = amdsmi_get_clk_freq(processor_handle, clk_type, &f);
value->type = INTEGER;
if (value->status == AMDSMI_STATUS_SUCCESS) {
  value->value.l_int = f.frequency[f.current];
}
```

發生問題時，AMD SMI 對 GPU 1 的 memory clock query 回傳：

```text
status        = AMDSMI_STATUS_SUCCESS (0)
num_supported = 3
current       = 0xffffffff (UINT32_MAX)
frequency[0]  = 900000000
frequency[1]  = 1100000000
```

`current == UINT32_MAX` 是 AMD SMI 已記錄的合法 sentinel，表示 clock domain power-gated、
sleep 或 kernel 沒有回報 current level。RDC 未檢查 `current < num_supported` 與
`current < AMDSMI_MAX_NUM_FREQUENCIES`，直接以 sentinel 索引 `frequency[]`，最後在
`RdcMetricFetcherImpl::fetch_gpu_field_()` 產生 SIGSEGV。

本問題不是單純「workload 存在但監控值為 0」。單 GPU control 能持續取得非零 profiling
數值；真正的發布阻擋是 8-GPU RDC server process 崩潰，之後 client 只會收到
`Connection refused`。

## 與 Issue 0001 的區別

本問題與 [Issue 0001](0001-profiling-fields-pmc-packet-overflow.md) 不同：

| 比較項目 | Issue 0001 | Issue 0002 |
| --- | --- | --- |
| GPU | gfx950 | gfx942 |
| 主要錯誤 | `Could not create PMC packets`, AQLProfile 4096 | `librdc.so.1.3` SIGSEGV |
| 觸發條件 | profiling counter 數量超過 PMC packet 容量 | field 101 與 profiling 同時存在於 8-GPU RDC context |
| Process 狀態 | worker fatal 後 exporter 可能仍存活但資料凍結 | `rdcd` process 直接死亡 |
| 本次 logs | 未出現 PMC packet/AQLProfile 4096 | kernel 明確記錄 `rdcd` segfault |

## 測試環境

### tainan-ci

- Host: `ubuntu@10.170.168.13`
- GPU: 8× AMD Instinct `gfx942`
- GPU access: 所有 runtime checks 以 root/privileged context 使用 `/dev/kfd`、`/dev/dri`
- Container platform: `linux/amd64`
- 測試期間使用獨立 `/tmp/rdc-exporter-*` 工作目錄
- 未修改主機既有 `/opt/rocm-7.14.0`
- 測試結束後已停止所有 `rdcd`、HIP workload 與 container

### HIP workload

為確認 profiling counters 不是因 GPU idle 而全部為 0，測試會在 GPU 0 執行約 20 秒的
HIP busy workload：

```text
/tmp/rdc-exporter-rocm714.IyFvbI/rdc-exporter-rocm714-load
```

執行方式：

```bash
sudo timeout 22 env HIP_VISIBLE_DEVICES=0 \
  /tmp/rdc-exporter-rocm714.IyFvbI/rdc-exporter-rocm714-load
```

該 workload 僅用於讓 GPU 0 產生可觀察的 GPU utilization 與 profiling activity；
RDC 測試所載入的 headers、libraries 與 plugins 仍全部來自被測 artifact。

## 被測 fields

### 預設 10 個 telemetry fields

| Field ID | RDC field | Prometheus name |
| ---: | --- | --- |
| 100 | `RDC_FI_GPU_CLOCK` | `gpu_clock` |
| 101 | `RDC_FI_MEM_CLOCK` | `mem_clock` |
| 200 | `RDC_FI_MEMORY_TEMP` | `memory_temp` |
| 201 | `RDC_FI_GPU_TEMP` | `gpu_temp` |
| 300 | `RDC_FI_POWER_USAGE` | `power_usage` |
| 500 | `RDC_FI_GPU_UTIL` | `gpu_util` |
| 501 | `RDC_FI_GPU_MEMORY_USAGE` | `gpu_memory_usage` |
| 502 | `RDC_FI_GPU_MEMORY_TOTAL` | `gpu_memory_total` |
| 600 | `RDC_FI_ECC_CORRECT_TOTAL` | `ecc_correct` |
| 601 | `RDC_FI_ECC_UNCORRECT_TOTAL` | `ecc_uncorrect` |

CLI representation：

```text
100,101,200,201,300,500,501,502,600,601
```

### 預設 6 個 profiling fields

| Field ID | RDC field | Prometheus name |
| ---: | --- | --- |
| 800 | `RDC_FI_PROF_OCCUPANCY_PERCENT` | `occupancy_percent` |
| 805 | `RDC_FI_PROF_GPU_UTIL_PERCENT` | `gpu_util_percent` |
| 804 | `RDC_FI_PROF_TENSOR_ACTIVE_PERCENT` | `tensor_percent` |
| 801 | `RDC_FI_PROF_ACTIVE_CYCLES` | `active_cycles` |
| 803 | `RDC_FI_PROF_ELAPSED_CYCLES` | `elapsed_cycles` |
| 812 | `RDC_FI_PROF_SM_ACTIVE` | `valubusy` |

CLI representation：

```text
800,805,804,801,803,812
```

### 完整 16-field set

```text
100,101,200,201,300,500,501,502,600,601,800,805,804,801,803,812
```

## Artifact provenance

所有下載的 tarball 均執行 `sha256sum` 與 `gzip -t`。每個解壓後的 RDC library 另外計算
SHA-256，以證明測試涵蓋不同 binary，而不是重複使用同一份 runtime root。

| ROCm | Artifact / source | Tarball SHA-256 | `librdc.so.1.3` SHA-256 |
| --- | --- | --- | --- |
| 7.14.0 | [all-GPU minimal RDC, 20260916](https://amd-afde.top/rocm/7.14.0/20260916/rocm7.14.0-all-gpu-minimal-rdc-glibc2.38.tar.gz) | `0a02fe40f4065ea9b24f2d9012a7534af0aef59874023c24bca65ae9a110fd6d` | `02ebd7fda5928e1c22813cfeb87e4e7c936e243a0e92751f35c5d79b622db3bf` |
| 7.14.0 | [gfx942/gfx950 build, 20260804](https://amd-afde.top/rocm/7.14.0/20260804/rocm7.14.0-gfx942-gfx950-glibc2.32.tar.gz) | `9a6e69f978cb61938766f9574cc5df3b4922ec348c5978834ac7bf713fcc4240` | `f1d0e98b9348b0e99fe88c8b75abba07e5aa4e78ff636230fdc38a3bd73dcf03` |
| 7.14.0 | tainan-ci `/opt/rocm-7.14.0` independent build | N/A | `04ce813d4198da2e87ae0d00f66b46f65a297e4c91182df539dc7eb200e9d89b` |
| 7.14.1 | [all-GPU minimal RDC, 20260916](https://amd-afde.top/rocm/7.14.1/20260916/rocm7.14.1-all-gpu-minimal-rdc-glibc2.38.tar.gz) | `5df9c1a8197ed0d70e05f608f7cad667e4523dc917355e3398d962d94b8c2adb` | `8c2a12e0c7699682f6346c6349f15ef5738aaa5ff00d637f8261c4bb6c6744d4` |
| 10.0.0 | [minimal build, 20260914](https://amd-afde.top/rocm/10.0.0/20260914/rocm10.0.0-minimal-glibc2.38.tar.gz) | `5fecb950071e3d9ed35ee456f6e6fcbcd1b87a3b05aca62720c611ce27bfbf36` | `c8d1bc0d866f290b74db253565bc08ebd8ffba9e0e1794787473e9a0b8f1b6c4` |
| 10.1.0rc0 | [AMD RC repository gfx94X dcgpu build](https://rc.repo.amd.com/rocm/core/tarball/therock-dist-linux-gfx94X-dcgpu-10.1.0rc0.tar.gz) | `831a9a25b3f26ced6a74a49afe4c87ade9fa2a9013b2549f2ffda18eb3ac9620` | `7efdfe088964e88144b278a4658cf129a793fcec46791d25169cf04d8775d2d0` |

原始 7.14.0 all-GPU build 的已知 provenance：

- TheRock commit: `418cd5f63abb7a604bad5874cd7b2e29334e640f`
- glibc upper bound: 2.38
- GPU targets: 27

AMD 10.1.0rc0 artifact 的 `.info/version` 內容是 `10.1.0`，RDC profiling log 顯示
rocprofiler-sdk `v1.4.1`。

## 基本測試程序

### 1. 準備 runtime library path

不同 artifact 的 root layout 略有差異。測試時設定：

```bash
ROCM_ROOT=/path/to/extracted/rocm
LD_LIBRARY_PATH="${ROCM_ROOT}/lib:${ROCM_ROOT}/lib/rocm_sysdeps/lib:${ROCM_ROOT}/lib/llvm/lib:${ROCM_ROOT}/lib/rdc/grpc/lib:${ROCM_ROOT}/lib/rdc/grpc/lib/rocm_sysdeps/lib"
```

先確認 `rdci` 與 `rdcd` 沒有 unresolved libraries：

```bash
LD_LIBRARY_PATH="${LD_LIBRARY_PATH}" ldd "${ROCM_ROOT}/bin/rdcd" | grep 'not found'
LD_LIBRARY_PATH="${LD_LIBRARY_PATH}" ldd "${ROCM_ROOT}/bin/rdci" | grep 'not found'
```

預期兩個命令均無輸出。

### 2. 啟動 isolated unauthenticated `rdcd`

每個 case 使用獨立 TCP port 與全新的 `rdcd` process，避免前一個 field group、watch 或
crash 狀態污染下一個 case：

```bash
sudo timeout 50 env LD_LIBRARY_PATH="${LD_LIBRARY_PATH}" \
  "${ROCM_ROOT}/bin/rdcd" -u -a 127.0.0.1 -p 55102 \
  >rdcd.log 2>&1 &
```

### 3. 使用官方 `rdci dmon`

單 GPU control：

```bash
LD_LIBRARY_PATH="${LD_LIBRARY_PATH}" \
  "${ROCM_ROOT}/bin/rdci" dmon \
  --host 127.0.0.1:55101 -u \
  -e 100,101,200,201,300,500,501,502,600,601,800,805,804,801,803,812 \
  -i 0 -d 1000 -c 8 -t
```

8-GPU reproduction：

```bash
LD_LIBRARY_PATH="${LD_LIBRARY_PATH}" \
  "${ROCM_ROOT}/bin/rdci" dmon \
  --host 127.0.0.1:55102 -u \
  -e 100,101,200,201,300,500,501,502,600,601,800,805,804,801,803,812 \
  -i 0,1,2,3,4,5,6,7 -d 1000 -c 5 -t
```

`-u` 代表不使用 mutual TLS，並不是 embedded RDC mode。

### 4. 判定標準

不能只看 `rdci` exit code。RDC server 崩潰後，`rdci` 仍可能回傳 0。每個 case 同時核對：

1. `rdcd` process 在 `rdci` 結束後是否仍存活。
2. output 是否包含每張 GPU 的有效 timestamp/value row，而非只有 `N/A`。
3. 是否出現 `Failed to connect` 或 `Connection refused`。
4. `rdcd.log` 是否包含 `dumped core`。
5. `dmesg` 是否新增 `rdcd ... segfault ... in librdc.so.1.3`。
6. workload 期間 GPU 0 的 telemetry 與 profiling 是否為非零且持續更新。

## 跨版本測試結果

### 單 GPU control

完整 16 fields 加上 GPU 0 HIP workload 在所有詳細測試版本都能工作：

| ROCm | Result | 代表性數值 |
| --- | --- | --- |
| 7.14.0 | Pass | `GPU_UTIL` 約 99–100，profiling counters 持續更新 |
| 7.14.1 | Pass | `GPU_UTIL=99`，`VALUBusy` 約 21–30% |
| 10.0.0 | Pass | `GPU_UTIL=99`，`VALUBusy` 約 22–30% |
| AMD 10.1.0rc0 | Pass | `GPU_UTIL=99`，`VALUBusy` 約 28.25–28.48% |

這證明：

- field IDs 在這些 artifacts 中仍有效。
- RDC profiling backend 能初始化。
- HIP workload 確實被 counters 觀察到。
- 不是所有 `MEM_CLOCK` query 都失敗。

### 8-GPU 完整 16 fields

| ROCm / build | `rdcd` after dmon | Client result | Kernel result |
| --- | --- | --- | --- |
| 7.14.0 all-GPU minimal | Dead | connection refused | `librdc.so.1.3` segfault |
| 7.14.0 gfx942/gfx950 | Dead | 可能先有 GPU 0 values，之後拒絕連線 | `librdc.so.1.3` segfault |
| 7.14.0 host independent build | Dead | connection refused | `librdc.so.1.3` segfault |
| 7.14.1 all-GPU minimal | Dead | connection refused | `librdc.so.1.3` segfault |
| 10.0.0 minimal | Dead | connection refused | `librdc.so.1.3` segfault |
| AMD 10.1.0rc0 gfx94X dcgpu | Dead | 0 valid GPU rows，515 connection errors | `librdc.so.1.3` segfault |

代表性 kernel log：

```text
rdcd[2707778]: segfault ... in librdc.so.1.3[...] likely on CPU ...
```

不同 tarball 的 `librdc.so.1.3` hashes 皆不同，但 failure mode 一致。

## AMD 10.1.0rc0 field-isolation matrix

10.1.0rc0 artifact 來自 AMD RC repository，且包含完整 `rdci`、`rdcd`、`librdc.so.1.3`、
RDC plugins 與 bundled gRPC。以下所有 case 都在相同 8× gfx942 host 上執行，且每個 case
使用新的 `rdcd` process。

### Category-level isolation

| Case | Fields | Result |
| --- | --- | --- |
| Telemetry only | `100,101,200,201,300,500,501,502,600,601` | Pass；8 張 GPU 各 5 個有效 samples，0 connection errors |
| Profiling only | `800,805,804,801,803,812` | Pass；8 張 GPU 都有 samples，`rdcd` 正常 shutdown |
| Full mixed set | 10 telemetry + 6 profiling | Fail；`rdcd` SIGSEGV |
| Full set without 101 | 9 telemetry + 6 profiling | Pass；8 張 GPU 各有 4 個有效 samples，0 connection errors |

Telemetry-only case 中 GPU 0 workload 的 `GPU_UTIL=100`；profiling-only case 也能正常載入
rocprofiler-sdk 1.4.1。這排除「telemetry 本身不支援 8 GPU」與「profiling 本身不支援
8 GPU」。

### 固定完整 telemetry set，每次加入一個 profiling field

| Added profiling field | Result |
| ---: | --- |
| 800 | SIGSEGV |
| 805 | SIGSEGV |
| 804 | SIGSEGV |
| 801 | SIGSEGV |
| 803 | SIGSEGV |
| 812 | SIGSEGV |

因此不是特定 profiling counter 壞掉。

### 固定 profiling field 800，每次只搭配一個 telemetry field

| Telemetry field | Mixed pair | Result |
| ---: | --- | --- |
| 100 | `100,800` | Pass；初始 `N/A` 後開始產生 8-GPU 有效值 |
| 101 | `101,800` | **SIGSEGV** |
| 200 | `200,800` | Pass |
| 201 | `201,800` | Pass |
| 300 | `300,800` | Pass |
| 500 | `500,800` | Pass |
| 501 | `501,800` | Pass |
| 502 | `502,800` | Pass |
| 600 | `600,800` | Pass |
| 601 | `601,800` | Pass |

只有 `RDC_FI_MEM_CLOCK`（101）是單一 telemetry trigger。

### 固定 field 101，每次搭配一個 profiling field

| Profiling field | Pair | Result |
| ---: | --- | --- |
| 800 | `101,800` | SIGSEGV |
| 805 | `101,805` | SIGSEGV |
| 804 | `101,804` | SIGSEGV |
| 801 | `101,801` | SIGSEGV |
| 803 | `101,803` | SIGSEGV |
| 812 | `101,812` | SIGSEGV |

所以必要 interaction 是 field 101 與 profiling subsystem 共存，而不是特定 profiling
field 的 event mapping。

### 拆成兩個 concurrent watches

為確認是否只是同一個 field group 混合兩種 backend 造成，另啟動兩個 `rdci dmon` client：

- Client A: 10 telemetry fields
- Client B: 6 profiling fields
- 兩者同時監控同一個 `rdcd` 的 8 張 GPU

結果：兩個 client 各取得少量初始資料後，`rdcd` 仍在 `librdc.so.1.3` SIGSEGV；telemetry
client 累計 323 connection errors，profiling client 累計 195 connection errors。

因此把 rdc-exporter 的 fields 拆成兩個 field groups/watches 並不足以修復。問題存在於同一個
RDC process/context 內 `MEM_CLOCK` 與 profiling 同時運作的狀態。

## Distroless 與 binding 排除測試

### Exact runtime-root closure

使用 `prepare-runtime-root.sh` 產生全新的 7.14 runtime root，並編譯一個 minimal official C
embedded RDC diagnostic。為避免 host library 意外介入，另建立無 RPATH variant，執行時只載入
裁切後 `runtime-root/opt/rocm`。

單 GPU + 完整 16 fields + workload 結果：

- `GPU_UTIL=100`
- `RDC_FI_PROF_SM_ACTIVE`/`VALUBusy` 約 24–30%
- timestamp 持續更新
- 沒有 missing shared library
- 沒有 broken symlink 造成的載入失敗

因此 Distroless runtime 裁切包含單 GPU 完整功能所需的 library/plugin/data closure。官方 AMD
RC build 與多個完整 ROCm trees 也能重現相同 8-GPU crash，故裁切不是根因。

### Official CLI/server path

主要重現完全使用 artifact 自帶的 `rdci`、`rdcd` 與 `librdc`，不經過：

- rdc-exporter Go code
- cgo wrapper
- Prometheus collector
- HTTP `/metrics`
- Distroless image

所以本次 `rdcd` SIGSEGV 不可能由 Go binding signature、goroutine scheduling 或 Prometheus
scrape path 單獨造成。Go exporter 仍可能有其他 integration 問題，但不是這個 multi-GPU
crash 的必要條件。

## Core dump analysis

### Core source

tainan-ci 的 Apport 保留一份獨立 7.14.0 host-build crash：

```text
/var/crash/_opt_rocm-7.14.0_bin_rdcd.0.crash
```

metadata：

```text
Date: Wed Sep 16 08:34:48 2026
ExecutablePath: /opt/rocm-7.14.0/bin/rdcd
ProcCmdline: /opt/rocm-7.14.0/bin/rdcd -u -a 127.0.0.1 -p 55060
Signal: 11
```

解包後的 652 MiB core：

```text
/tmp/rdcd-apport.0qCg9w/CoreDump
```

使用 AMD 10.1.0rc0 artifact 自帶的 `rocgdb 16.3` 分析，不需修改主機套件。

### Crashing frame

```text
#0 amd::rdc::RdcMetricFetcherImpl::fetch_gpu_field_(
     gpu_index=1,
     field_id=RDC_FI_MEM_CLOCK,
     value=...,
     processor_handle=...)
   at RdcMetricFetcherImpl.cc:897
#1 amd::rdc::RdcMetricFetcherImpl::fetch_smi_field(...)
#2 amd::rdc::RdcSmiLib::rdc_telemetry_fields_value_get(
     fields_count=80, ...)
#3 amd::rdc::RdcTelemetryModule::rdc_telemetry_fields_value_get(...)
#4 amd::rdc::RdcWatchTableImpl::rdc_field_update_all(...)
#5 amd::rdc::RdcMetricsUpdaterImpl::start() worker
```

`fields_count=80` 對應 8 GPU × 10 telemetry fields。Profiling subsystem 已在同一 process
初始化，但 crash 發生在 telemetry updater 讀取 GPU 1 的 `MEM_CLOCK` 時。

### Values at crash

```text
field_id           = RDC_FI_MEM_CLOCK
gpu_index          = 1
f.has_deep_sleep   = false
f.num_supported    = 3
f.current          = 0xffffffff
value->status      = 0 (AMDSMI_STATUS_SUCCESS)
f.frequency[0]     = 900000000
f.frequency[1]     = 1100000000
```

Disassembly：

```text
call amdsmi_get_clk_freq@plt
test %eax,%eax
jne  error_path
mov  0x268(%rsp),%eax              # f.current = 0xffffffff
mov  0x270(%rsp,%rax,8),%rax       # f.frequency[f.current] -> SIGSEGV
```

這證明 AMD SMI call 回傳 success 後 RDC 直接以 `UINT32_MAX` 索引 array。

## Root cause

### AMD SMI contract

AMD SMI 的 `amdsmi_frequencies_t` 定義明確說明：

- `current` 是 current frequency index。
- `current` 可以是 `(uint32_t)-1`。
- sentinel 表示 clock domain power-gated/sleep 或 kernel 沒有回報 current level。
- `frequency[]` 只有前 `num_supported` 個元素有效。

來源：
[ROCm/rocm-systems — amdsmi.h](https://github.com/ROCm/rocm-systems/blob/c383f8dacf2303e2bb1dab4b923d4937b77cab66/projects/amdsmi/include/amd_smi/amdsmi.h#L2019-L2029)

### Unsafe RDC consumer

RDC 只檢查 `amdsmi_get_clk_freq()` status，沒有檢查 index：

```cpp
if (value->status == AMDSMI_STATUS_SUCCESS) {
  value->value.l_int = f.frequency[f.current];
}
```

來源：
[ROCm/rocm-systems — RdcMetricFetcherImpl.cc](https://github.com/ROCm/rocm-systems/blob/c383f8dacf2303e2bb1dab4b923d4937b77cab66/projects/rdc/rdc_libs/rdc/src/RdcMetricFetcherImpl.cc#L982-L993)

### Trigger interpretation

實測顯示 profiling subsystem 與 `MEM_CLOCK` 在 8-GPU context 共存時，至少 GPU 1 會在某次
update 回報 `current == UINT32_MAX`。合理推論是 profiling 初始化或 multi-GPU clock/power 狀態
改變暴露了一個 clock domain 暫時沒有 current DPM level 的時間窗；AMD SMI 以其契約允許的
sentinel 表示，RDC 卻將 sentinel 當成有效 index。

「profiling 為何增加 sentinel 出現機率」尚未完全定位，但這不影響 crash 的直接根因：
RDC 必須防守 AMD SMI 已允許的 `current == UINT32_MAX` 與任何 out-of-range index。

## Ruled out

### 不是 rdc-exporter binding 才造成

官方 `rdci`/`rdcd` 可獨立重現，沒有使用任何 Go/cgo code。

### 不是單一 custom TheRock binary

至少五份不同 `librdc.so.1.3` hashes、四個 ROCm version/build line 均可重現，其中包括 AMD
RC repository 的 gfx94X dcgpu artifact。

### 不是 minimal build 缺少元件

- `ldd` 沒有 missing library。
- `rdci`、`rdcd`、RDC plugins、rocprofiler data 均存在。
- 單 GPU 的完整 telemetry/profiling 能正常工作。
- 非 minimal 的 gfx942/gfx950 artifact 與 AMD RC artifact 也會重現。

### 不是 Distroless runtime 裁切

完整未裁切 artifact、主機 `/opt/rocm-7.14.0` 與 AMD RC artifact 都會重現。裁切 runtime 的
official C diagnostic 在單 GPU 下也能正常載入所有功能。

### 不是特定 profiling field

field 101 分別搭配六個 profiling fields 全部重現。

### 不是 field count 單純過多

最小 `101 + one profiling field` 即可重現；`100 + 800` 與其他九個 telemetry pair 都正常。

### 不是同一 field group 才會發生

拆成兩個 concurrent watches 仍會 crash。

## Secondary observations

### POWER_USAGE sentinel/scaling anomaly

多個版本的 `rdci` output 都出現：

```text
POWER_USAGE = 4294967295000000
```

此值等於 `UINT32_MAX × 1,000,000`，很像另一個未過濾的 unavailable/sentinel value。它不是本次
SIGSEGV 的 crash frame，也不是本文件已證明的直接 root cause；應另行追蹤，避免與 field 101
越界問題混淆。

### `rdci` exit status 不可靠

即使 `rdcd` 已 segfault 且 output 大量出現 connection refused，`rdci dmon` 仍可能 exit 0。
CI 不可只以 client process exit code 判定成功。

### Initial `N/A` samples

部分正常 mixed-pair cases 在 profiling 初始化期間先輸出一至兩輪 `N/A`，之後才出現有效值。
驗收應允許短暫 warm-up，但必須確認後續 timestamp/value 持續更新。

## Proposed upstream fix

The current exact-base `therock-10.0` backport used by this project is stored at
[`patches/rdc/rdc-clock-index-bounds-therock-10.0-6b0e43f3.patch`](../../patches/rdc/rdc-clock-index-bounds-therock-10.0-6b0e43f3.patch)
with application and validation instructions in the adjacent
[`README.md`](../../patches/rdc/README.md).

RDC 在使用 `f.current` 前必須檢查兩個上限：

```cpp
if (value->status == AMDSMI_STATUS_SUCCESS) {
  if (f.num_supported > 0 &&
      f.num_supported <= AMDSMI_MAX_NUM_FREQUENCIES &&
      f.current < f.num_supported &&
      f.current < AMDSMI_MAX_NUM_FREQUENCIES) {
    value->value.l_int = f.frequency[f.current];
  } else {
    value->status = AMDSMI_STATUS_NO_DATA;
  }
}
```

實際 upstream patch 應再確認 RDC 對 unavailable telemetry 的既有 status convention；
`AMDSMI_STATUS_NO_DATA` 是合理候選，但不可在沒有相容性檢查下直接假設為最終 API 行為。

建議同步增加：

1. `current == UINT32_MAX` unit test。
2. `current >= num_supported` unit test。
3. `num_supported > AMDSMI_MAX_NUM_FREQUENCIES` defensive test。
4. Multi-GPU integration test：`MEM_CLOCK + one profiling field`，至少包含一張 idle GPU。
5. CI 判定 `rdcd` 存活與連續 samples，而非只看 `rdci` exit 0。

## Workarounds and limitations

### Remove field 101

完整預設組合只移除 `RDC_FI_MEM_CLOCK` 後，8 GPU telemetry 與 profiling 均通過。這是已驗證
workaround，但會刪減既有預設 metric，違反本次 release requirement，因此不能直接用於發布。

### Disable profiling

10 telemetry fields（包含 101）在 8 GPU 下正常；但停用 profiling 同樣改變既有 16-field
contract，不符合本次發布要求。

### Split field groups/watches

已驗證無效。同一 `rdcd` 中兩個 concurrent watches 仍會 crash。

### Patch and rebuild RDC

這是保留原有 metrics contract 的解法。2026-09-17 已以修補
`RdcMetricFetcherImpl` bounds check 的 TheRock/RDC artifact 重跑原始完整 16 fields，結果通過。

## Release decision

原始 unpatched artifact 在下列條件完成前不得建立或推送對應 release tag/image：

1. 使用 patched/fixed RDC，保留原始 10 telemetry + 6 profiling fields。
2. 8 張 gfx942 都能持續輸出 samples。
3. GPU 0 workload 期間 `RDC_FI_PROF_SM_ACTIVE`/`VALUBusy` 為非零。
4. 連續 scrape 數值與 timestamp 持續更新。
5. `rdcd`/embedded exporter process 保持存活。
6. logs/dmesg 無 `librdc` segfault、PMC packet fatal 或 AQLProfile 4096。
7. 使用同一 fixed artifact 重建 exporter binary 與 final runtime image。

### Patched artifact validation（2026-09-17）

上述 gate 已由 patched TheRock artifact 通過：

- Artifact：`rocm7.14.0-all-gpu-minimal-rdc-clock-index-fix-glibc2.38.tar.gz`
- Artifact SHA-256：`1ba6d19d0928b384ef30bbb993efbb98a6738bcbe80842fd9508daf181d48528`
- TheRock commit：`418cd5f63abb7a604bad5874cd7b2e29334e640f`
- rocm-systems commit：`2b22ab0195cc1461cd9abf3b969e9dd7c10af350`
- Patch SHA-256：`213ef61a7a564e4c9f664d48c42e6e6d4629602f926cae50a6591fba5875c299`
- Patched `librdc.so.1.3` SHA-256：`ca86028c7c005ce34f9c01029b8a8786c244ec3fece8ea248dc690632a047852`

原生 `rdci`/`rdcd` 在 8 張 GPU、完整 16 fields、GPU 0 HIP workload 下連續執行 20 輪：

- `rdcd` 保持存活，client status 0。
- `GPU_UTIL=100`。
- 六個 profiling fields 皆有數值，`VALUBusy` 持續更新。
- 無 connection refusal、core dump、PMC packet、AQLProfile 4096 或 segfault。

使用相同 artifact headers、libraries 與全新 runtime root 建置的 exporter candidate 也通過：

- `/metrics` 恰有 128 個預設 GPU samples（16 fields × 8 GPUs）。
- 兩次 workload scrape 的 GPU 0 `valubusy` 分別為
  `181.4867570563982` 與 `181.93163358492765`，`gpu_util=100`。
- image-local `rocm-smi` 找到全部 8 張 MI308X/gfx942 GPU。
- exporter process 保持存活，logs 無 missing library、PMC/AQLProfile fatal、panic 或 segfault。

因此不需修改 rdc-exporter 的 Go/cgo binding。idle GPU 的 memory-clock current index 無效時，
patched RDC 會回報 no data（log 中可見 status 13 / AMD SMI code 40）而不再越界。

### Patched ROCm 7.14.1 artifact validation（2026-09-17）

ROCm 7.14.1 使用同一個 bounds fix、但以新的 exact source baseline 重新建置並獨立驗證：

- Artifact：`rocm7.14.1-all-gpu-minimal-rdc-clock-index-fix.tar.gz`
- Artifact SHA-256：`0be3665633164f78c2d82fc13e9d105d2bdb87dd323f4be295946426260a3fef`
- TheRock commit：`f51dc6c91e0d3214f22853fd5cb3f96dbc7d2c4b`
- rocm-systems commit：`ca887ee80abfb82671fe1d6d8da708a713438e05`
- Patch SHA-256：`213ef61a7a564e4c9f664d48c42e6e6d4629602f926cae50a6591fba5875c299`
- Patched `librdc.so.1.3` SHA-256：`1096eafa7df169aa60c700954a68bd4a7f6c6aac4e99dcbc4f4a72a78c2abe74`
- `share/therock/dist_info.json`：28 個 GPU targets
- runtime closure 最大 glibc requirement：`GLIBC_2.38`

原生 RDC 的 `101,800` regression case 在 8 張 `gfx942` GPU 上完成 12 輪，daemon 保持存活。
完整 16-field case 在 GPU 0 持續 FP32 workload 下完成 20 輪，`GPU_UTIL=100`，20 個 GPU 0
samples 的 `VALUBusy` 全部非零，最高約 `98.944`。idle GPU 的 field 101 可顯示 `N/A`，並留下
status 13 / AMD SMI code 40，但沒有 connection refusal、segfault、PMC packet 或 AQLProfile
4096 錯誤。

以相同 headers/libraries 編譯的 exporter 通過 `go test ./...`，全新 runtime root 無 broken
symlink，candidate image 的 Docker local size 是 204,845,060 bytes（195.35 MiB）。image-local
`rocm-smi` 找到 8 張 MI308X/gfx942 GPU；兩次 qualified scrape 都恰有 128 個預設 GPU
samples，GPU 0 `gpu_util=100`，`valubusy` 從 `65.54684016745742` 更新為
`69.32468342459957`，`active_cycles` 也持續前進。exporter logs 無 loader、PMC、AQLProfile、
panic 或 segfault 錯誤。

7.14.1 的結果再次確認問題與修復都位於 RDC/TheRock；rdc-exporter 不需要 Go/cgo binding
調整，也不需要刪減既有 10 telemetry + 6 profiling fields。

### Patched ROCm 10.0.0 artifact validation（2026-09-17）

ROCm 10.0.0 以新的 exact source baseline 套用相同 bounds fix，並明確停用 GPU emulation：

- Artifact：`rocm10.0.0-all-gpu-minimal-rdc-clock-index-fix-no-emulation-glibc2.38.tar.gz`
- Artifact SHA-256：`a1e075511bb479e9baae21abc6786d52a60bf7dc0c7a2946c64f891a43179788`
- TheRock commit：`16adc4d875fd4f65ea23c7c84e1c66706fde3047`
- rocm-systems commit：`6b0e43f341195e203754e08f850e437ff2fc09f9`
- Patch SHA-256：`bd6f313b66a8e8ccf276e1a33c0d12dcd04d8c9e6d1866239b8712511619519a`
- Patched `librdc.so.1.3` SHA-256：`7c764ab6c23de64ff45e88c9b65be5073502e889192626c4e9679496a065402f`
- `share/therock/dist_info.json`：28 個 GPU targets
- 完整 distribution 最大 glibc requirement：`GLIBC_2.38`

artifact 內 `rdci`、`rdcd`、`librdc.so.1.3` 沒有 unresolved dynamic libraries，artifact
內的 `rocm-smi` 找到 8 張 MI308X/gfx942 GPU。原生 RDC 驗證結果如下：

- `101,800` regression case 完成 12 輪，8 張 GPU 各有 12 rows，daemon 在 client
  結束後仍存活。
- 完整 16-field case 在 GPU 0 持續 HIP workload 下完成 20 輪，8 張 GPU 各有 20
  rows；workload 期間 `GPU_UTIL=100`，`VALUBusy` 為 21.331–29.652%。
- idle GPU 的 field 101 可顯示 `N/A` 並留下 status 13 / AMD SMI code 40，但 daemon
  保持存活，其他 fields 持續更新。
- client、server 與 kernel logs 無 connection refusal、segfault、PMC packet failure 或
  AQLProfile return code 4096。

未修改的既有 exporter source 以 Go 1.26.3 對同一份 ROCm 10.0.0 headers/libraries 完成
`go test ./...` 與 CGO build，證明這次升級不需要修改 Go/cgo binding。全新 runtime root
為 572,271,314 bytes、沒有 broken symlink；final distroless candidate 為 211,029,557 bytes。
image-local `rocm-smi` 找到全部 8 張 GPU，每次 qualified scrape 都恰有 128 個 samples。

exporter 的 RDC watch period 是 10 秒；兩個只相隔 4 秒的初始 workload scrapes 因讀到同一份
cache 而完全相同，這不是凍結。讓 workload 跨越兩個 update windows 後，GPU 0
`valubusy` 從 `29.28883367251129` 更新為 `29.50381430223124`，`active_cycles` 從
`57,122,976` 更新為 `57,172,185`，兩次的 `gpu_util` 都是 100。exporter logs 無 loader、
PMC、AQLProfile、panic 或 segfault 錯誤，因此 10.0.0 同樣不需修改 Go/cgo binding 或刪減
預設 fields。

## Evidence locations on tainan-ci

這些是診斷暫存資料，不應視為永久 artifact storage：

| Evidence | Location |
| --- | --- |
| ROCm 7.14.0 all-GPU tests、candidate image、C diagnostic | `/tmp/rdc-exporter-rocm714.IyFvbI` |
| ROCm 7.14.0 gfx942/gfx950 tests | `/tmp/rocm714-gfx942-gfx950-ab.HgZ8kf` |
| ROCm 7.14.1 tests | `/tmp/rdc-exporter-rocm7141.AxP3yL` |
| ROCm 10.0.0 tests | `/tmp/rdc-exporter-rocm1000.dYDr2M` |
| AMD 10.1.0rc0 full isolation matrix | `/tmp/rdc-exporter-rocm101rc0.gyu22U` |
| Patched ROCm 7.14 native RDC、runtime root、candidate image 與 exporter logs | `/dockerdata/tasks/150-rdc-exporter-rocm714-clock-fix` |
| 7.14.0 Apport report | `/var/crash/_opt_rocm-7.14.0_bin_rdcd.0.crash` |
| Unpacked core dump | `/tmp/rdcd-apport.0qCg9w/CoreDump` |

10.1.0rc0 workspace 的重要 logs：

```text
rdci-single.log
rdcd-single.log
rdci-all8.log
rdcd-all8.log
rdci-all8-telemetry.log
rdcd-all8-telemetry.log
rdci-all8-profiling.log
rdcd-all8-profiling.log
rdci-all8-without-101.log
rdcd-all8-without-101.log
rdci-all8-split-telemetry.log
rdci-all8-split-profiling.log
rdcd-all8-split-watch.log
rdci-pair-<telemetry>-800.log
rdcd-pair-<telemetry>-800.log
rdci-pair-101-<profiling>.log
rdcd-pair-101-<profiling>.log
```

調查結束時 tainan-ci 沒有殘留測試 process 或 running container。證據目錄仍保留，當時 root
filesystem 約剩 18 GiB，後續若需長期保存應先搬移 logs/core，再清理大型解壓目錄。

## References

- [AMD SMI `amdsmi_frequencies_t` contract](https://github.com/ROCm/rocm-systems/blob/c383f8dacf2303e2bb1dab4b923d4937b77cab66/projects/amdsmi/include/amd_smi/amdsmi.h#L2019-L2029)
- [RDC unchecked `f.frequency[f.current]` access](https://github.com/ROCm/rocm-systems/blob/c383f8dacf2303e2bb1dab4b923d4937b77cab66/projects/rdc/rdc_libs/rdc/src/RdcMetricFetcherImpl.cc#L982-L993)
- [Related but not identical MEM_CLOCK issue in ROCm/rocm-systems](https://github.com/ROCm/rocm-systems/issues/3937)
- [Project metric catalog](../metrics.md)
- [Issue 0001 — profiling PMC packet overflow](0001-profiling-fields-pmc-packet-overflow.md)
