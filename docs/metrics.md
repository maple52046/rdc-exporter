# Metric List

## Common Labels

Every exported metric carries `gpu_index` and `UUID`. The UUID is read once at
startup from `rocm-smi --json --showuniqueid` and normalized to
`GPU-` plus 16 lowercase hexadecimal digits; discovery failure leaves the value
empty without stopping metric collection. When kubelet integration is enabled,
the metrics also carry the workload labels `pod`, `namespace`, and `container`.

## RDC Metrics

| Metric | Prom Name | Field ID | Enable | Usage/Help | DCGM Metric |
|--------|-----------|----------|:------:|------------|-------------|
| RDC_FI_GPU_CLOCK | gpu_clock | 100 | Y | Current GPU clock frequencies | DCGM_FI_DEV_SM_CLOCK |
| RDC_FI_MEM_CLOCK | mem_clock | 101 | Y | Current Memory clock frequencies | DCGM_FI_DEV_MEM_CLOCK |
| RDC_FI_MEMORY_TEMP | memory_temp | 200 | Y | Memory temperature in millidegrees Celsius | DCGM_FI_DEV_MEMORY_TEMP |
| RDC_FI_GPU_TEMP | gpu_temp | 201 | Y | GPU temperature in millidegrees Celsius | DCGM_FI_DEV_GPU_TEMP |
| RDC_FI_POWER_USAGE | power_usage | 300 | Y (RDC 6.4+) | Power usage in microwatts | DCGM_FI_DEV_POWER_USAGE |
| RDC_FI_PCIE_TX | pcie_tx | 400 | N | PCIe Tx utilization in bytes/second | DCGM_FI_DEV_PCIE_TX_THROUGHPUT |
| RDC_FI_PCIE_RX | pcie_rx | 401 | N | PCIe Rx utilization in bytes/second | DCGM_FI_DEV_PCIE_RX_THROUGHPUT |
| RDC_FI_PCIE_BANDWIDTH | pcie_bandwidth | 403 | N | PCIe bandwidth in Mbps | - |
| RDC_FI_GPU_UTIL | gpu_util | 500 | Y (RDC 6.4+) | GPU busy percentage | DCGM_FI_DEV_GPU_UTIL |
| RDC_FI_GPU_MEMORY_USAGE | gpu_memory_usage | 501 | Y | Memory usage of the GPU instance in bytes | - |
| RDC_FI_GPU_MEMORY_TOTAL | gpu_memory_total | 502 | Y | Total memory of the GPU instance | - |
| RDC_FI_GPU_MM_ENC_UTIL | gpu_mm_enc_util | 503 | N | Mutilmedia encoder busy percentage | DCGM_FI_DEV_ENC_UTIL |
| RDC_FI_GPU_MM_DEC_UTIL | gpu_mm_dec_util | 504 | N | Mutilmedia decoder busy percentage | DCGM_FI_DEV_DEC_UTIL |
| RDC_FI_GPU_MEMORY_ACTIVITY | gpu_mem_util | 505 | N | Memory busy percentage | DCGM_FI_DEV_MEM_COPY_UTIL |
| RDC_FI_GPU_MEMORY_MAX_BANDWIDTH | gpu_mem_max_bandwidth | 506 | N | Memory max bandwidth | - |
| RDC_FI_GPU_MEMORY_CUR_BANDWIDTH | gpu_mem_cur_bandwidth | 507 | N | Memory current bandwidth | - |
| RDC_FI_GPU_BUSY_PERCENT | gpu_busy_percent | 508 | N | GPU busy percentage | - |
| RDC_FI_GPU_PAGE_RETRIED | gpu_page_retried | 550 | N | Retried page of the GPU instance | - |
| RDC_FI_ECC_CORRECT_TOTAL | ecc_correct | 600 | Y | Accumulated Single Error Correction | - |
| RDC_FI_ECC_UNCORRECT_TOTAL | ecc_uncorrect | 601 | Y | Accumulated Double Error Detection | DCGM_FI_DEV_ECC_DBE_VOL_TOTAL |
| RDC_FI_ECC_SDMA_UE | ecc_sdma_ue | 604 | N | SDMA Uncorrectable Error | - |
| RDC_FI_ECC_GFX_CE | ecc_gfx_ce | 605 | N | GFX Correctable Error | - |
| RDC_FI_ECC_GFX_UE | ecc_gfx_ue | 606 | N | GFX Uncorrectable Error | - |
| RDC_FI_ECC_MMHUB_CE | ecc_mmhub_ce | 607 | N | MMHUB Correctable Error | - |
| RDC_FI_ECC_MMHUB_UE | ecc_mmhub_ue | 608 | N | MMHUB Uncorrectable Error | - |
| RDC_FI_ECC_ATHUB_CE | ecc_athub_ce | 609 | N | ATHUB Correctable Error | - |
| RDC_FI_ECC_ATHUB_UE | ecc_athub_ue | 610 | N | ATHUB Uncorrectable Error | - |
| RDC_FI_ECC_PCIE_BIF_CE | ecc_pcie_bif_ce | 611 | N | PCIE_BIF Correctable Error | - |
| RDC_FI_ECC_PCIE_BIF_UE | ecc_pcie_bif_ue | 612 | N | PCIE_BIF Uncorrectable Error | - |
| RDC_FI_ECC_HDP_CE | ecc_hdp_ce | 613 | N | HDP Correctable Error | - |
| RDC_FI_ECC_HDP_UE | ecc_hdp_ue | 614 | N | HDP Uncorrectable Error | - |
| RDC_FI_ECC_XGMI_WAFL_CE | ecc_xgmi_wafl_ce | 615 | N | XGMI_WAFL Correctable Error | DCGM_FI_DEV_NVSWITCH_NON_FATAL_ERRORS |
| RDC_FI_ECC_XGMI_WAFL_UE | ecc_xgmi_wafl_ue | 616 | N | XGMI_WAFL Uncorrectable Error | DCGM_FI_DEV_NVSWITCH_FATAL_ERRORS |
| RDC_FI_ECC_DF_CE | ecc_df_ce | 617 | N | DF Correctable Error | - |
| RDC_FI_ECC_DF_UE | ecc_df_ue | 618 | N | DF Uncorrectable Error | - |
| RDC_FI_ECC_SMN_CE | ecc_smn_ce | 619 | N | SMN Correctable Error | - |
| RDC_FI_ECC_SMN_UE | ecc_smn_ue | 620 | N | SMN Uncorrectable Error | - |
| RDC_FI_ECC_SEM_CE | ecc_sem_ce | 621 | N | SEM Correctable Error | - |
| RDC_FI_ECC_SEM_UE | ecc_sem_ue | 622 | N | SEM Uncorrectable Error | - |
| RDC_FI_ECC_MP0_CE | ecc_mp0_ce | 623 | N | MP0 Correctable Error | - |
| RDC_FI_ECC_MP0_UE | ecc_mp0_ue | 624 | N | MP0 Uncorrectable Error | - |
| RDC_FI_ECC_MP1_CE | ecc_mp1_ce | 625 | N | MP1 Correctable Error | - |
| RDC_FI_ECC_MP1_UE | ecc_mp1_ue | 626 | N | MP1 Uncorrectable Error | - |
| RDC_FI_ECC_FUSE_CE | ecc_fuse_ce | 627 | N | FUSE Correctable Error | - |
| RDC_FI_ECC_FUSE_UE | ecc_fuse_ue | 628 | N | FUSE Uncorrectable Error | - |
| RDC_FI_ECC_UMC_CE | ecc_umc_ce | 629 | N | UMC Correctable Error | - |
| RDC_FI_ECC_UMC_UE | ecc_umc_ue | 630 | N | UMC Uncorrectable Error | - |
| RDC_FI_ECC_MCA_CE | ecc_mca_ce | 631 | N | MCA Correctable Error | - |
| RDC_FI_ECC_MCA_UE | ecc_mca_ue | 632 | N | MCA Uncorrectable Error | - |
| RDC_FI_ECC_VCN_CE | ecc_vcn_ce | 633 | N | VCN Correctable Error | - |
| RDC_FI_ECC_VCN_UE | ecc_vcn_ue | 634 | N | VCN Uncorrectable Error | - |
| RDC_FI_ECC_JPEG_CE | ecc_jpeg_ce | 635 | N | JPEG Correctable Error | - |
| RDC_FI_ECC_JPEG_UE | ecc_jpeg_ue | 636 | N | JPEG Uncorrectable Error | - |
| RDC_FI_ECC_IH_CE | ecc_ih_ce | 637 | N | IH Correctable Error | - |
| RDC_FI_ECC_IH_UE | ecc_ih_ue | 638 | N | IH Uncorrectable Error | - |
| RDC_FI_ECC_MPIO_CE | ecc_mpio_ce | 639 | N | MPIO Correctable Error | - |
| RDC_FI_ECC_MPIO_UE | ecc_mpio_ue | 640 | N | MPIO Uncorrectable Error | - |
| RDC_FI_XGMI_0_READ_KB | xgmi_0_read | 700 | N | XGMI0 accumulated data read size (KB) | - |
| RDC_FI_XGMI_1_READ_KB | xgmi_1_read | 701 | N | XGMI1 accumulated data read size (KB) | - |
| RDC_FI_XGMI_2_READ_KB | xgmi_2_read | 702 | N | XGMI2 accumulated data read size (KB) | - |
| RDC_FI_XGMI_3_READ_KB | xgmi_3_read | 703 | N | XGMI3 accumulated data read size (KB) | - |
| RDC_FI_XGMI_4_READ_KB | xgmi_4_read | 704 | N | XGMI4 accumulated data read size (KB) | - |
| RDC_FI_XGMI_5_READ_KB | xgmi_5_read | 705 | N | XGMI5 accumulated data read size (KB) | - |
| RDC_FI_XGMI_6_READ_KB | xgmi_6_read | 706 | N | XGMI6 accumulated data read size (KB) | - |
| RDC_FI_XGMI_7_READ_KB | xgmi_7_read | 707 | N | XGMI7 accumulated data read size (KB) | - |
| RDC_FI_XGMI_0_WRITE_KB | xgmi_0_write | 708 | N | XGMI0 accumulated data write size (KB) | - |
| RDC_FI_XGMI_1_WRITE_KB | xgmi_1_write | 709 | N | XGMI1 accumulated data write size (KB) | - |
| RDC_FI_XGMI_2_WRITE_KB | xgmi_2_write | 710 | N | XGMI2 accumulated data write size (KB) | - |
| RDC_FI_XGMI_3_WRITE_KB | xgmi_3_write | 711 | N | XGMI3 accumulated data write size (KB) | - |
| RDC_FI_XGMI_4_WRITE_KB | xgmi_4_write | 712 | N | XGMI4 accumulated data write size (KB) | - |
| RDC_FI_XGMI_5_WRITE_KB | xgmi_5_write | 713 | N | XGMI5 accumulated data write size (KB) | - |
| RDC_FI_XGMI_6_WRITE_KB | xgmi_6_write | 714 | N | XGMI6 accumulated data write size (KB) | - |
| RDC_FI_XGMI_7_WRITE_KB | xgmi_7_write | 715 | N | XGMI7 accumulated data write size (KB) | - |
| RDC_FI_XGMI_TOTAL_READ_KB | xgmi_total_read | 716 | N | XGMI accumlated data read size across all lanes (KB) | - |
| RDC_FI_XGMI_TOTAL_WRITE_KB | xgmi_total_write | 717 | N | XGMI accumlated data write size across all lanes (KB) | - |
| RDC_FI_PROF_OCCUPANCY_PERCENT | occupancy_percent | 800 | Y | Percent of GPU occupancy | - |
| RDC_FI_PROF_ACTIVE_CYCLES | active_cycles | 801 | Y | Number of Active Cycles | - |
| RDC_FI_PROF_ACTIVE_WAVES | active_waves | 802 | Y | Number of Active Waves | - |
| RDC_FI_PROF_ELAPSED_CYCLES | elapsed_cycles | 803 | Y | Number of Elapsed Cycles over all SMs | - |
| RDC_FI_PROF_TENSOR_ACTIVE_PERCENT | tensor_percent | 804 | Y | Percent of Active Pipe Tensors | DCGM_FI_PROF_PIPE_TENSOR_ACTIVE |
| RDC_FI_PROF_GPU_UTIL_PERCENT | gpu_util_percent | 805 | Y | Percent of GPU Utilization | - |
| RDC_FI_PROF_EVAL_MEM_R_BW | mem_r_bw | 806 | Y | Fetched from video memory kb / ms | - |
| RDC_FI_PROF_EVAL_MEM_W_BW | mem_w_bw | 807 | Y | Written to video memory kb / ms | - |
| RDC_FI_PROF_EVAL_FLOPS_16 | flops_16 | 808 | Y | Number of fp16 OPS / ms | AMPF_FI_FROF_FP16_TFPS_USED |
| RDC_FI_PROF_EVAL_FLOPS_32 | flops_32 | 809 | Y | Number of fp32 OPS / ms | AMPF_FI_FROF_FP32_TFPS_USED |
| RDC_FI_PROF_EVAL_FLOPS_64 | flops_64 | 810 | Y | Number of fp64 OPS / ms | AMPF_FI_FROF_FP64_TFPS_USED |
| RDC_FI_PROF_VALU_PIPE_ISSUE_UTIL | valu_utilization | 811 | Y | Percent of Active Pipe VALU | - |
| RDC_FI_PROF_SM_ACTIVE | valubusy | 812 | Y | Ratio of Cycles with active warp on SM | DCGM_FI_PROF_SM_ACTIVE |
| RDC_FI_PROF_OCC_PER_ACTIVE_CU | occ_cu | 813 | Y | Mean occ per active compute unit | DCGM_FI_PROF_SM_OCCUPANCY |
| RDC_FI_PROF_OCC_ELAPSED | occ_cu_elapsed | 814 | Y | Mean occ per active cu over elapsed | - |
| RDC_FI_PROF_EVAL_FLOPS_16_PERCENT | flops_16_percent | 815 | Y | Number of fp16 OPS percent of max | DCGM_FI_PROF_PIPE_FP16_ACTIVE |
| RDC_FI_PROF_EVAL_FLOPS_32_PERCENT | flops_32_percent | 816 | Y | Number of fp32 OPS percent of max | DCGM_FI_PROF_PIPE_FP32_ACTIVE |
| RDC_FI_PROF_EVAL_FLOPS_64_PERCENT | flops_64_percent | 817 | Y | Number of fp64 OPS percent of max | DCGM_FI_PROF_PIPE_FP64_ACTIVE |
| RDC_FI_PROF_CPC_CPC_STAT_BUSY | cpc_cpc_stat_busy | 818 | N | - | - |
| RDC_FI_PROF_CPC_CPC_STAT_IDLE | cpc_cpc_stat_idle | 819 | N | - | - |
| RDC_FI_PROF_CPC_CPC_STAT_STALL | cpc_cpc_stat_stall | 820 | N | - | - |
| RDC_FI_PROF_CPC_CPC_TCIU_BUSY | cpc_cpc_tciu_busy | 821 | N | - | - |
| RDC_FI_PROF_CPC_CPC_TCIU_IDLE | cpc_cpc_tciu_idle | 822 | N | - | - |
| RDC_FI_PROF_CPC_CPC_UTCL2IU_BUSY | cpc_cpc_utcl2iu_busy | 823 | N | - | - |
| RDC_FI_PROF_CPC_CPC_UTCL2IU_IDLE | cpc_cpc_utcl2iu_idle | 824 | N | - | - |
| RDC_FI_PROF_CPC_CPC_UTCL2IU_STALL | cpc_cpc_utcl2iu_stall | 825 | N | - | - |
| RDC_FI_PROF_CPC_ME1_BUSY_FOR_PACKET_DECODE | cpc_me1_busy_for_packet_decode | 826 | N | - | - |
| RDC_FI_PROF_CPC_ME1_DC0_SPI_BUSY | cpc_me1_dc0_spi_busy | 827 | N | - | - |
| RDC_FI_PROF_CPC_UTCL1_STALL_ON_TRANSLATION | cpc_utcl1_stall_on_translation | 828 | N | - | - |
| RDC_FI_PROF_CPC_ALWAYS_COUNT | cpc_always_count | 829 | N | - | - |
| RDC_FI_PROF_CPC_ADC_VALID_CHUNK_NOT_AVAIL | cpc_adc_valid_chunk_not_avail | 830 | N | - | - |
| RDC_FI_PROF_CPC_ADC_DISPATCH_ALLOC_DONE | cpc_adc_dispatch_alloc_done | 831 | N | - | - |
| RDC_FI_PROF_CPC_ADC_VALID_CHUNK_END | cpc_adc_valid_chunk_end | 832 | N | - | - |
| RDC_FI_PROF_CPC_SYNC_FIFO_FULL_LEVEL | cpc_sync_fifo_full_level | 833 | N | - | - |
| RDC_FI_PROF_CPC_SYNC_FIFO_FULL | cpc_sync_fifo_full | 834 | N | - | - |
| RDC_FI_PROF_CPC_GD_BUSY | cpc_gd_busy | 835 | N | - | - |
| RDC_FI_PROF_CPC_TG_SEND | cpc_tg_send | 836 | N | - | - |
| RDC_FI_PROF_CPC_WALK_NEXT_CHUNK | cpc_walk_next_chunk | 837 | N | - | - |
| RDC_FI_PROF_CPC_STALLED_BY_SE0_SPI | cpc_stalled_by_se0_spi | 838 | N | - | - |
| RDC_FI_PROF_CPC_STALLED_BY_SE1_SPI | cpc_stalled_by_se1_spi | 839 | N | - | - |
| RDC_FI_PROF_CPC_STALLED_BY_SE2_SPI | cpc_stalled_by_se2_spi | 840 | N | - | - |
| RDC_FI_PROF_CPC_STALLED_BY_SE3_SPI | cpc_stalled_by_se3_spi | 841 | N | - | - |
| RDC_FI_PROF_CPC_LTE_ALL | cpc_lte_all | 842 | N | - | - |
| RDC_FI_PROF_CPC_SYNC_WRREQ_FIFO_BUSY | cpc_sync_wrreq_fifo_busy | 843 | N | - | - |
| RDC_FI_PROF_CPC_CANE_BUSY | cpc_cane_busy | 844 | N | - | - |
| RDC_FI_PROF_CPC_CANE_STALL | cpc_cane_stall | 845 | N | - | - |
| RDC_FI_PROF_CPF_CMP_UTCL1_STALL_ON_TRANSLATION | cpf_cmp_utcl1_stall_on_translation | 846 | N | - | - |
| RDC_FI_PROF_CPF_CPF_STAT_BUSY | cpf_cpf_stat_busy | 847 | N | - | - |
| RDC_FI_PROF_CPF_CPF_STAT_IDLE | cpf_cpf_stat_idle | 848 | N | - | - |
| RDC_FI_PROF_CPF_CPF_STAT_STALL | cpf_cpf_stat_stall | 849 | N | - | - |
| RDC_FI_PROF_CPF_CPF_TCIU_BUSY | cpf_cpf_tciu_busy | 850 | N | - | - |
| RDC_FI_PROF_CPF_CPF_TCIU_IDLE | cpf_cpf_tciu_idle | 851 | N | - | - |
| RDC_FI_PROF_CPF_CPF_TCIU_STALL | cpf_cpf_tciu_stall | 852 | N | - | - |
| RDC_FI_PROF_SIMD_UTILIZATION | simd_utilization | 853 | N | Fraction of time the SIMDs are being utilized | - |
| RDC_FI_PROF_KFD_ID | prof_kfd_id | 854 | N | GPU_ID from rocprofiler, same as KFD_ID | - |
| RDC_EVNT_XGMI_0_NOP_TX | xgmi_nop_0 | 1000 | N | NOPs sent to neighbor 0 | - |
| RDC_EVNT_XGMI_0_REQ_TX | xgmi_req_0 | 1001 | N | Outgoing requests to neighbor 0 | - |
| RDC_EVNT_XGMI_0_RESP_TX | xgmi_res_0 | 1002 | N | Outgoing responses to neighbor 0 | - |
| RDC_EVNT_XGMI_0_BEATS_TX | xgmi_bts_0 | 1003 | N | Data sent to neighbor 0 (32 byte pkts) | - |
| RDC_EVNT_XGMI_1_NOP_TX | xgmi_nop_1 | 1004 | N | NOPs sent to neighbor 1 | - |
| RDC_EVNT_XGMI_1_REQ_TX | xgmi_req_1 | 1005 | N | Outgoing requests to neighbor 1 | - |
| RDC_EVNT_XGMI_1_RESP_TX | xgmi_res_1 | 1006 | N | Outgoing responses to neighbor 1 | - |
| RDC_EVNT_XGMI_1_BEATS_TX | xgmi_bts_1 | 1007 | N | Data sent to neighbor 1 (32 byte pkts) | DCGM_FI_PROF_NVLINK_TX_BYTES |
| RDC_EVNT_XGMI_0_THRPUT | xgmi_0_t | 1500 | N | Tx throughput to XGMI neighbor 0 in b/s | - |
| RDC_EVNT_XGMI_1_THRPUT | xgmi_1_t | 1501 | N | Tx throughput to XGMI neighbor 1 in b/s | - |
| RDC_EVNT_XGMI_2_THRPUT | xgmi_2_t | 1502 | N | Tx throughput to XGMI neighbor 2 in b/s | - |
| RDC_EVNT_XGMI_3_THRPUT | xgmi_3_t | 1503 | N | Tx throughput to XGMI neighbor 3 in b/s | - |
| RDC_EVNT_XGMI_4_THRPUT | xgmi_4_t | 1504 | N | Tx throughput to XGMI neighbor 4 in b/s | - |
| RDC_EVNT_XGMI_5_THRPUT | xgmi_5_t | 1505 | N | Tx throughput to XGMI neighbor 5 in b/s | - |
| RDC_EVNT_NOTIF_VMFAULT | vm_page_fault | 2000 | N | VM page fault | - |
| RDC_EVNT_NOTIF_THERMAL_THROTTLE | thermal_throt | 2002 | N | Clk freq decrease due to temp | - |
| RDC_EVNT_NOTIF_PRE_RESET | gpu_pre_reset | 2003 | N | GPU reset is about to occur | - |
| RDC_EVNT_NOTIF_POST_RESET | gpu_post_reset | 2004 | N | GPU reset just occurred | - |
| RDC_EVNT_NOTIF_MIGRATE_START | migrate_start | 2005 | N | GPU migrate has started | - |
| RDC_EVNT_NOTIF_MIGRATE_END | migrate_end | 2006 | N | GPU migrate has ended | - |
| RDC_EVNT_NOTIF_PAGE_FAULT_START | page_fault_start | 2007 | N | GPU page fault started | - |
| RDC_EVNT_NOTIF_PAGE_FAULT_END | page_fault_end | 2008 | N | GPU page fault ended | - |
| RDC_EVNT_NOTIF_QUEUE_EVICTION | queue_evicition | 2009 | N | GPU queue eviction occured | - |
| RDC_EVNT_NOTIF_QUEUE_RESTORE | queue_restore | 2010 | N | GPU queue restore occured | - |
| RDC_EVNT_NOTIF_UNMAP_FROM_GPU | unmap_from_gpu | 2011 | N | GPU unmap occured | - |
| RDC_EVNT_NOTIF_PROCESS_START | process_start | 2012 | N | GPU process started | - |
| RDC_EVNT_NOTIF_PROCESS_END | process_end | 2013 | N | GPU process ended | - |
| RDC_HEALTH_XGMI_ERROR | xgmi_error | 3000 | N | XGMI one or more errors detected | - |
| RDC_HEALTH_PCIE_REPLAY_COUNT | pcie_replay_count | 3001 | N | Total PCIE replay count | DCGM_FI_DEV_PCIE_REPLAY_COUNTER |
| RDC_HEALTH_RETIRED_PAGE_NUM | retired_page_num | 3002 | Y | Retired page number | DCGM_FI_DEV_RETIRED_DBE |
| RDC_HEALTH_PENDING_PAGE_NUM | pending_page_num | 3003 | N | Pending page number | DCGM_FI_DEV_RETIRED_PENDING |
| RDC_HEALTH_RETIRED_PAGE_LIMIT | retired_page_limit | 3004 | N | Retired page limit | - |
| RDC_HEALTH_EEPROM_CONFIG_VALID | eeprom_config_valid | 3005 | N | Verify checksum of EEPROM | - |
| RDC_HEALTH_POWER_THROTTLE_TIME | power_throttle_time | 3006 | N | Power throttle status counter | DCGM_FI_DEV_POWER_VIOLATION |
| RDC_HEALTH_THERMAL_THROTTLE_TIME | thermal_throttle_time | 3007 | N | Total time(ms) in thermal throttle status | DCGM_FI_DEV_THERMAL_VIOLATION |
