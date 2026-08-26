# syntax=docker/dockerfile:1
FROM ubuntu:24.04 AS base

## Basic arguments and environment parameters
ENV DEBIAN_FRONTEND=noninteractive
ENV _GLIBCXX_USE_CXX11_ABI=1

## Setup ROCm repo
## ROCM_DEB / ROCM_VERSION / GO_VERSION are provided by the Makefile via --build-arg.
ARG ROCM_DEB
RUN apt update && \
    apt install -y wget && \
    wget -O /tmp/amdgpu-install.deb ${ROCM_DEB} && \
    apt install -y /tmp/amdgpu-install.deb && \
    sed -i '/graphics/d' /etc/apt/sources.list.d/rocm.list && \
    rm -f /tmp/amdgpu-install.deb && \
    apt clean && \
    apt update

## Install ROCm packages for building
RUN apt update && \
    apt install -y rdc rocprofiler-sdk && \
    apt clean

## Setup ROCm environment
ENV ROCM_HOME="/opt/rocm"
ENV C_INCLUDE_PATH="$ROCM_HOME/include:$C_INCLUDE_PATH"
ENV CMAKE_PREFIX_PATH="$ROCM_HOME/lib/cmake:${CMAKE_PREFIX_PATH:-/usr/local/lib/cmake}"
ENV CPLUS_INCLUDE_PATH="$ROCM_HOME/include:${CPLUS_INCLUDE_PATH:-/usr/local/include}"
ENV LD_LIBRARY_PATH="$ROCM_HOME/lib:${LD_LIBRARY_PATH:-/usr/local/lib}"
ENV PATH="$ROCM_HOME/bin:$PATH"

WORKDIR /opt

FROM base AS builder

## Install basic packages
RUN apt update && \
    apt install -y make && \
    apt clean

## Download and install Go
ARG GO_VERSION
RUN wget -qO- https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH="/usr/local/go/bin:$PATH"

## Build rdc-exporter
WORKDIR /opt/rdc-exporter
ADD . /opt/rdc-exporter
RUN go mod tidy
RUN make

## Final output
FROM base

ARG BUILD_DATE
ARG ROCM_VERSION
LABEL org.opencontainers.image.title="rdc-exporter" \
      org.opencontainers.image.authors="Chen-Hao Ku <Bill.Ku@amd.com>" \
      org.opencontainers.image.description="RDC Exporter for AMD GPU" \
      org.opencontainers.image.source="https://github.com/maple52046/rdc-exporter" \
      org.opencontainers.image.vendor="DCGPU System Eng TWN, AMD Inc." \
      org.opencontainers.image.version="${BUILD_DATE:-20251210}" \
      com.amd.rocm.version="${ROCM_VERSION}"

# Install basic ROCm packages
RUN apt update && \
    apt install -y amd-smi-lib comgr hip-runtime-amd hsa-amd-aqlprofile libhsa-runtime64-1 libdw1 rocm-smi-lib && \
    apt clean

# Install RDC exporter
WORKDIR /opt/rdc-exporter
RUN --mount=type=bind,from=builder,source=/opt/rdc-exporter,target=/builder,ro \
    mkdir -p bin && \
    cp /builder/bin/rdc-exporter bin/rdc-exporter && \
    cp /builder/internal/config/catalog/catalog.yaml catalog-example.yaml && \
    chmod +x bin/rdc-exporter
ENV PATH="/opt/rdc-exporter/bin:$PATH"

WORKDIR /opt/rdc-exporter
EXPOSE 5000
ENTRYPOINT ["/opt/rdc-exporter/bin/rdc-exporter"]
