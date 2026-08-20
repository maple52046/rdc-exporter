---
name: build-docker-image
description: Build and verify the TheRock-based Debian 13 distroless rdc-exporter image through the Makefile. Requires a matching TheRock ROCm root and commit, preserves rocm-smi and RDC profiling support, and supports --date, --no-verify, --push, and an explicit --tag override. Use when the user runs /build-docker-image or asks to build, verify, tag, or push the container image.
disable-model-invocation: true
---

# Build Docker Image — Cursor Entry

This is the Cursor-specific entry point for the `build-docker-image` skill. Its
only job is to wire the skill into Cursor's `/`-command discovery.

The full, IDE-neutral instructions are defined once in:

`skills/build-docker-image/SKILL.md` (relative to the repository root).

When this skill is invoked, read `skills/build-docker-image/SKILL.md` and follow
it exactly. Do not duplicate or fork the steps here — keep this file as a thin
reference so there is a single source of truth.
