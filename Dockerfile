# syntax=docker/dockerfile:1

# rdc-exporter needs glibc and dynamically loaded ROCm libraries. rocm-smi is
# a Python program, so its direct-command requirement makes python3-debian13
# the smallest suitable distroless family; static-debian13 cannot run either.
ARG DISTROLESS_IMAGE=gcr.io/distroless/python3-debian13@sha256:b340f07acd3692d739cbc28450b8876b4770ee01967aa9e4193c3bcec7bd235e

FROM debian:13-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258 AS native-runtime
RUN apt-get update && \
    apt-get install -y --no-install-recommends libatomic1 libstdc++6 && \
    mkdir -p /runtime && \
    cp -aL /usr/lib/x86_64-linux-gnu/libatomic.so.1 /runtime/libatomic.so.1 && \
    cp -aL /usr/lib/x86_64-linux-gnu/libstdc++.so.6 /runtime/libstdc++.so.6 && \
    cp -aL /lib/x86_64-linux-gnu/libgcc_s.so.1 /runtime/libgcc_s.so.1 && \
    rm -rf /var/lib/apt/lists/*

FROM ${DISTROLESS_IMAGE}

ARG BUILD_DATE
ARG ROCM_VERSION
ARG ROCM_ARCHS
ARG THEROCK_COMMIT
ARG RDC_EXPORTER_COMMIT

LABEL org.opencontainers.image.title="rdc-exporter" \
      org.opencontainers.image.description="AMD RDC Prometheus exporter on TheRock and distroless Debian 13" \
      org.opencontainers.image.source="https://github.com/maple52046/rdc-exporter" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      com.amd.rocm.version="${ROCM_VERSION}" \
      com.amd.rocm.archs="${ROCM_ARCHS}" \
      com.amd.rocm.source="TheRock" \
      com.amd.rocm.therock.commit="${THEROCK_COMMIT}" \
      com.amd.rdc-exporter.commit="${RDC_EXPORTER_COMMIT}"

COPY runtime-root/ /
COPY --from=native-runtime /runtime/libatomic.so.1 /usr/lib/x86_64-linux-gnu/libatomic.so.1
COPY --from=native-runtime /runtime/libstdc++.so.6 /usr/lib/x86_64-linux-gnu/libstdc++.so.6
COPY --from=native-runtime /runtime/libgcc_s.so.1 /usr/lib/x86_64-linux-gnu/libgcc_s.so.1
COPY bin/rdc-exporter /opt/rdc-exporter/bin/rdc-exporter

ENV ROCM_HOME=/opt/rocm \
    LD_LIBRARY_PATH=/opt/rocm/lib:/opt/rocm/lib/rdc:/opt/rocm/lib/llvm/lib:/opt/rocm/lib/rocm_sysdeps/lib \
    PATH=/opt/rdc-exporter/bin:/opt/rocm/bin:/usr/bin

WORKDIR /opt/rdc-exporter
EXPOSE 5000

# The upstream DaemonSet is privileged and expects the image's default root
# user. Moving to nonroot requires an explicit policy for /dev/kfd, /dev/dri,
# and the kubelet pod-resources socket, so this image keeps root compatibility.
USER 0
ENTRYPOINT ["/opt/rdc-exporter/bin/rdc-exporter"]

# Profiling is supported, but PMC packet capacity is hardware-dependent. This
# conservative MI355X-validated set keeps six profiling counters and includes
# SM_ACTIVE. Users may replace CMD to select other fields, but should add
# profiling fields gradually and watch for AQLProfile return code 4096.
CMD ["--fields", "RDC_FI_GPU_CLOCK,RDC_FI_MEM_CLOCK,RDC_FI_MEMORY_TEMP,RDC_FI_GPU_TEMP,RDC_FI_POWER_USAGE,RDC_FI_GPU_UTIL,RDC_FI_GPU_MEMORY_USAGE,RDC_FI_GPU_MEMORY_TOTAL,RDC_FI_ECC_CORRECT_TOTAL,RDC_FI_ECC_UNCORRECT_TOTAL,RDC_FI_PROF_OCCUPANCY_PERCENT,RDC_FI_PROF_GPU_UTIL_PERCENT,RDC_FI_PROF_TENSOR_ACTIVE_PERCENT,RDC_FI_PROF_ACTIVE_CYCLES,RDC_FI_PROF_ELAPSED_CYCLES,RDC_FI_PROF_SM_ACTIVE"]
