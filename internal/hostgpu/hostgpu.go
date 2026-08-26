// Package hostgpu discovers AMD GPUs and their stable identities through host
// interfaces such as the kernel sysfs amdgpu tree and rocm-smi.
//
// This is a Frameworks and Drivers layer package: it performs host I/O against
// /sys or a host command and returns plain device descriptors or identity maps.
// It exists to align PCI addresses, DRM render devices, and UUIDs with the same
// zero-based index the exporter uses elsewhere. It must not depend on RDC,
// Prometheus, or the use-case layer.
package hostgpu

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
)

// amdgpuDriverPath is the sysfs directory the amdgpu driver populates with one
// entry per bound PCI device. Discovery order is derived from sorting these
// entries so the assigned index is stable across runs on the same host.
const amdgpuDriverPath = "/sys/module/amdgpu/drivers/pci:amdgpu/"

// GpuDevice describes one discovered AMD GPU and the identifiers used to match
// it to container device allocations.
//
// Index is assigned by discovery order and is intended to line up with the GPU
// index used by the rest of the exporter. PCIAddress and RenderDevice are
// pointers because either may be unavailable; a nil field means that identifier
// could not be resolved for the device.
type GpuDevice struct {
	// Index is the zero-based discovery index of the GPU.
	Index int `json:"index"`
	// PCIAddress is the PCI bus address, for example 0000:1b:00.0, if available.
	PCIAddress *string `json:"pci,omitempty"`
	// RenderDevice is the DRM render node path, for example /dev/dri/renderD128,
	// if available.
	RenderDevice *string `json:"render,omitempty"`
}

// GetAMDGpuDevices enumerates the AMD GPUs bound to the amdgpu driver.
//
// Entries are sorted by PCI address so indexes are assigned deterministically.
// A directory entry that is not a PCI address, or whose render device cannot be
// resolved, is skipped so one unusable device does not hide the others. It
// returns an error only when the amdgpu driver directory itself cannot be read,
// which usually means the driver is not loaded.
func GetAMDGpuDevices() ([]*GpuDevice, error) {
	entries, err := os.ReadDir(amdgpuDriverPath)
	if err != nil {
		slog.Error("Failed to read AMD GPU directory", "error", err)
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	// Match a sysfs entry that is a PCI address such as 0000:1b:00.0; the
	// hex bus segment is case-insensitive across kernels.
	pciPattern := regexp.MustCompile(`^(0000:[0-9a-fA-F]{2}:00\.0\s*)+$`)

	devices := make([]*GpuDevice, 0, len(entries))
	for _, entry := range entries {
		pciAddr := entry.Name()
		if !pciPattern.MatchString(pciAddr) {
			slog.Debug("Skipping non-PCI address entry", "entry", pciAddr)
			continue
		}

		renderDev, err := RenderDeviceByPCI(pciAddr)
		if err != nil {
			slog.Error("Skipping device due to render device error", "pciAddress", pciAddr, "error", err)
			continue
		}

		devices = append(devices, &GpuDevice{
			Index:        len(devices),
			PCIAddress:   &pciAddr,
			RenderDevice: &renderDev,
		})
	}

	return devices, nil
}

// RenderDeviceByPCI resolves the DRM render node path (for example
// /dev/dri/renderD128) for a GPU identified by its PCI address. It returns an
// error if the device's DRM directory cannot be read or contains no render node,
// which lets the caller skip a device that cannot be attributed to containers.
func RenderDeviceByPCI(pciAddr string) (string, error) {
	drmBasePath := amdgpuDriverPath + pciAddr + "/drm"

	entries, err := os.ReadDir(drmBasePath)
	if err != nil {
		slog.Error("Failed to read DRM base path", "path", drmBasePath, "error", err)
		return "", err
	}

	renderPattern := regexp.MustCompile(`^(renderD[0-9]*)$`)
	for _, entry := range entries {
		renderDev := entry.Name()
		if !renderPattern.MatchString(renderDev) {
			slog.Debug("Skipping non-render device entry", "entry", renderDev)
			continue
		}
		return "/dev/dri/" + renderDev, nil
	}

	return "", fmt.Errorf("no render device found for PCI address %s", pciAddr)
}
