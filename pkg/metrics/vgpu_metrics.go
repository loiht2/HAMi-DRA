/*
Copyright 2025 The HAMi Authors.

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

package metrics

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

// bytesToMiB converts bytes to MiB (mebibytes) with rounding to the nearest integer.
func bytesToMiB(bytes uint64) float64 {
	return math.Round(float64(bytes) / 1048576)
}

// Real-time vGPU container metrics read from HAMi-core shared memory cache files.
// These metrics are only available when the monitor runs in node-level mode
// (--node-name and --hook-path are set) and has access to the host vgpu directory.
var (
	// ----- Bytes metrics (commented out — kept for future use) -----
	// ctrvGPUMemoryUsageDesc = prometheus.NewDesc(
	// 	"vGPU_device_memory_usage_in_bytes",
	// 	"Real-time vGPU device memory usage read from HAMi-core shared memory",
	// 	[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	// )
	//
	// ctrvGPUMemoryUsageRealDesc = prometheus.NewDesc(
	// 	"vGPU_device_memory_usage_real_in_bytes",
	// 	"Real GPU memory usage from NVML (matches nvidia-smi) via HAMi-core monitorused",
	// 	[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	// )
	//
	// ctrvGPUMemoryLimitDesc = prometheus.NewDesc(
	// 	"vGPU_device_memory_limit_in_bytes",
	// 	"vGPU device memory limit configured for the container",
	// 	[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	// )
	//
	// ctrDeviceMemoryContextDesc = prometheus.NewDesc(
	// 	"vGPU_device_memory_context_size_bytes",
	// 	"Container device memory context size from HAMi-core shared memory",
	// 	[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	// )
	//
	// ctrDeviceMemoryModuleDesc = prometheus.NewDesc(
	// 	"vGPU_device_memory_module_size_bytes",
	// 	"Container device memory module size from HAMi-core shared memory",
	// 	[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	// )
	//
	// ctrDeviceMemoryBufferDesc = prometheus.NewDesc(
	// 	"vGPU_device_memory_buffer_size_bytes",
	// 	"Container device memory buffer size from HAMi-core shared memory",
	// 	[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	// )

	// ----- Active metrics -----

	ctrDeviceMemoryDesc = prometheus.NewDesc(
		"Device_memory_desc_of_container",
		"Container device memory description from HAMi-core shared memory",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrDeviceUtilizationDesc = prometheus.NewDesc(
		"Device_utilization_desc_of_container",
		"Container device SM utilization from HAMi-core shared memory",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrDeviceLastKernelDesc = prometheus.NewDesc(
		"Device_last_kernel_of_container",
		"Seconds since the container last ran a GPU kernel",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	// ----- MiB metrics (rounded) -----

	ctrvGPUMemoryUsageMiBDesc = prometheus.NewDesc(
		"vGPU_device_memory_usage_in_MiB",
		"Real-time vGPU device memory usage in MiB (cudaMalloc tracked, rounded)",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrvGPUMemoryUsageRealMiBDesc = prometheus.NewDesc(
		"vGPU_device_memory_usage_real_in_MiB",
		"Real GPU memory usage in MiB from NVML (matches nvidia-smi, rounded)",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrvGPUMemoryLimitMiBDesc = prometheus.NewDesc(
		"vGPU_device_memory_limit_in_MiB",
		"vGPU device memory limit in MiB (rounded)",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrDeviceMemoryContextMiBDesc = prometheus.NewDesc(
		"vGPU_device_memory_context_size_MiB",
		"Container device memory context size in MiB (rounded)",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrDeviceMemoryModuleMiBDesc = prometheus.NewDesc(
		"vGPU_device_memory_module_size_MiB",
		"Container device memory module size in MiB (rounded)",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)

	ctrDeviceMemoryBufferMiBDesc = prometheus.NewDesc(
		"vGPU_device_memory_buffer_size_MiB",
		"Container device memory buffer size in MiB (rounded)",
		[]string{"podnamespace", "podname", "ctrname", "vdeviceid", "deviceuuid"}, nil,
	)
)
