/*
Copyright 2024 The HAMi Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package monitor

// UsageInfo is the interface for reading HAMi-core shared memory cache data.
// It provides per-device memory and utilization information written in real-time
// by the in-container HAMi-core interceptor library (libvgpu.so).
type UsageInfo interface {
	DeviceMax() int
	DeviceNum() int
	DeviceMemoryContextSize(idx int) uint64
	DeviceMemoryModuleSize(idx int) uint64
	DeviceMemoryBufferSize(idx int) uint64
	DeviceMemoryOffset(idx int) uint64
	DeviceMemoryTotal(idx int) uint64
	DeviceMemoryMonitor(idx int) uint64 // NVML-reported real memory usage (monitorused)
	DeviceSmUtil(idx int) uint64
	SetDeviceSmLimit(l uint64)
	IsValidUUID(idx int) bool
	DeviceUUID(idx int) string
	DeviceMemoryLimit(idx int) uint64
	SetDeviceMemoryLimit(l uint64)
	LastKernelTime() int64
	GetPriority() int
	GetRecentKernel() int32
	SetRecentKernel(v int32)
	GetUtilizationSwitch() int32
	SetUtilizationSwitch(v int32)
}

// ContainerUsage holds the mmap-ed shared memory data for a single container.
type ContainerUsage struct {
	PodUID        string
	ContainerName string
	Data          []byte
	Info          UsageInfo
}
