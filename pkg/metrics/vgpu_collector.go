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
	"fmt"
	"time"

	"github.com/Project-HAMi/HAMi-DRA/pkg/cache"
	"github.com/Project-HAMi/HAMi-DRA/pkg/monitor"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

// VGPUCollector collects real-time per-container GPU metrics by reading
// HAMi-core shared memory cache files via a ContainerLister.
// It implements prometheus.Collector.
type VGPUCollector struct {
	containerLister *monitor.ContainerLister
	podLister       listerscorev1.PodLister
	nodeName        string
	nodeDevices     *cache.NodeDevices
}

// NewVGPUCollector creates a VGPUCollector. It sets up a pod informer to
// resolve podUID -> (namespace, name) for metric labels.
func NewVGPUCollector(containerLister *monitor.ContainerLister, clientset kubernetes.Interface, nodeName string, nodeDevices *cache.NodeDevices) *VGPUCollector {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, time.Hour*1)
	podLister := informerFactory.Core().V1().Pods().Lister()
	stopCh := make(chan struct{})
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	return &VGPUCollector{
		containerLister: containerLister,
		podLister:       podLister,
		nodeName:        nodeName,
		nodeDevices:     nodeDevices,
	}
}

// Describe implements prometheus.Collector — sends all vGPU metric descriptors.
func (vc *VGPUCollector) Describe(ch chan<- *prometheus.Desc) {
	// Bytes metrics are commented out — kept for future use.
	// ch <- ctrvGPUMemoryUsageDesc
	// ch <- ctrvGPUMemoryUsageRealDesc
	// ch <- ctrvGPUMemoryLimitDesc
	// ch <- ctrDeviceMemoryContextDesc
	// ch <- ctrDeviceMemoryModuleDesc
	// ch <- ctrDeviceMemoryBufferDesc

	ch <- ctrDeviceMemoryDesc
	ch <- ctrDeviceUtilizationDesc
	ch <- ctrDeviceLastKernelDesc
	ch <- ctrvGPUMemoryUsageMiBDesc
	ch <- ctrvGPUMemoryUsageRealMiBDesc
	ch <- ctrvGPUMemoryLimitMiBDesc
	ch <- ctrDeviceMemoryContextMiBDesc
	ch <- ctrDeviceMemoryModuleMiBDesc
	ch <- ctrDeviceMemoryBufferMiBDesc
}

// Collect implements prometheus.Collector — gathers real-time container GPU metrics.
func (vc *VGPUCollector) Collect(ch chan<- prometheus.Metric) {
	if vc == nil || vc.containerLister == nil {
		return
	}

	containers := vc.containerLister.ListContainers()
	if len(containers) == 0 {
		klog.V(5).Info("No containers with cache files found")
		return
	}

	// Build podUID -> (namespace, name) map
	pods, err := vc.podLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("VGPUCollector: failed to list pods: %v", err)
		return
	}
	// podMeta holds resolved pod metadata for metric labels.
	type podMeta struct {
		namespace   string
		name        string
		ctrImages   map[string]string // containerName -> image (with tag)
		ctrImageIDs map[string]string // containerName -> imageID (digest)
	}
	podInfoMap := make(map[string]*podMeta) // podUID -> metadata
	for _, pod := range pods {
		imgMap := make(map[string]string)
		imgIDMap := make(map[string]string)
		for _, cs := range pod.Status.ContainerStatuses {
			imgMap[cs.Name] = cs.Image
			imgIDMap[cs.Name] = cs.ImageID
		}
		podInfoMap[string(pod.UID)] = &podMeta{
			namespace:  pod.Namespace,
			name:       pod.Name,
			ctrImages:  imgMap,
			ctrImageIDs: imgIDMap,
		}
	}

	// Build UUID -> productName map from DRA ResourceSlice cache.
	uuidToProduct := make(map[string]string)
	if vc.nodeDevices != nil {
		devices := vc.nodeDevices.GetDevices(vc.nodeName)
		for _, d := range devices {
			if d.UUID != "" {
				uuidToProduct[d.UUID] = d.ProductName
			}
		}
	}

	nowSec := time.Now().Unix()

	for _, c := range containers {
		if c.Info == nil {
			continue
		}

		podInfo, found := podInfoMap[c.PodUID]
		if !found {
			klog.V(5).Infof("VGPUCollector: pod UID %s not found in informer cache, skipping", c.PodUID)
			continue
		}
		podNamespace := podInfo.namespace
		podName := podInfo.name
		ctrName := c.ContainerName
		ctrImage := podInfo.ctrImages[ctrName]
		ctrImageID := podInfo.ctrImageIDs[ctrName]

		for i := range c.Info.DeviceNum() {
			uuid := c.Info.DeviceUUID(i)
			if len(uuid) < 40 {
				continue
			}
			uuid = uuid[0:40]

			lbls := []string{podNamespace, podName, ctrName, fmt.Sprint(i), uuid}

			// Extended labels for usage_real and limit metrics:
			// pod_uid, image (with tag), device_type (GPU product name)
			deviceType := uuidToProduct[uuid]
			if deviceType == "" {
				deviceType = "unknown"
			}
			extLbls := append(lbls, c.PodUID, ctrImage, ctrImageID, deviceType)

			memoryTotal := c.Info.DeviceMemoryTotal(i)
			memoryLimit := c.Info.DeviceMemoryLimit(i)
			memoryMonitor := c.Info.DeviceMemoryMonitor(i)
			memoryContextSize := c.Info.DeviceMemoryContextSize(i)
			memoryModuleSize := c.Info.DeviceMemoryModuleSize(i)
			memoryBufferSize := c.Info.DeviceMemoryBufferSize(i)
			smUtil := c.Info.DeviceSmUtil(i)
			lastKernelTime := c.Info.LastKernelTime()

			// Bytes metrics are commented out — kept for future use.
			// sendMetricSafe(ch, ctrvGPUMemoryUsageDesc, prometheus.GaugeValue, float64(memoryTotal), lbls...)
			// sendMetricSafe(ch, ctrvGPUMemoryUsageRealDesc, prometheus.GaugeValue, float64(memoryMonitor), lbls...)
			// sendMetricSafe(ch, ctrvGPUMemoryLimitDesc, prometheus.GaugeValue, float64(memoryLimit), lbls...)
			// sendMetricSafe(ch, ctrDeviceMemoryContextDesc, prometheus.GaugeValue, float64(memoryContextSize), lbls...)
			// sendMetricSafe(ch, ctrDeviceMemoryModuleDesc, prometheus.GaugeValue, float64(memoryModuleSize), lbls...)
			// sendMetricSafe(ch, ctrDeviceMemoryBufferDesc, prometheus.GaugeValue, float64(memoryBufferSize), lbls...)

			// Active metrics
			sendMetricSafe(ch, ctrDeviceMemoryDesc, prometheus.GaugeValue, float64(memoryTotal), lbls...)
			sendMetricSafe(ch, ctrDeviceUtilizationDesc, prometheus.GaugeValue, float64(smUtil), lbls...)

			// MiB metrics (rounded)
			sendMetricSafe(ch, ctrvGPUMemoryUsageMiBDesc, prometheus.GaugeValue, bytesToMiB(memoryTotal), lbls...)
			sendMetricSafe(ch, ctrvGPUMemoryUsageRealMiBDesc, prometheus.GaugeValue, bytesToMiB(memoryMonitor), extLbls...)
			sendMetricSafe(ch, ctrvGPUMemoryLimitMiBDesc, prometheus.GaugeValue, bytesToMiB(memoryLimit), extLbls...)
			sendMetricSafe(ch, ctrDeviceMemoryContextMiBDesc, prometheus.GaugeValue, bytesToMiB(memoryContextSize), lbls...)
			sendMetricSafe(ch, ctrDeviceMemoryModuleMiBDesc, prometheus.GaugeValue, bytesToMiB(memoryModuleSize), lbls...)
			sendMetricSafe(ch, ctrDeviceMemoryBufferMiBDesc, prometheus.GaugeValue, bytesToMiB(memoryBufferSize), lbls...)

			if lastKernelTime > 0 {
				lastSec := max(nowSec-lastKernelTime, 0)
				sendMetricSafe(ch, ctrDeviceLastKernelDesc, prometheus.GaugeValue, float64(lastSec), lbls...)
			}
		}

		klog.V(5).Infof("VGPUCollector: collected metrics for pod %s/%s container %s", podNamespace, podName, ctrName)
	}
}

func sendMetricSafe(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, value float64, labelValues ...string) {
	metric, err := prometheus.NewConstMetric(desc, valueType, value, labelValues...)
	if err != nil {
		klog.Errorf("Failed to create metric %s: %v", desc.String(), err)
		return
	}
	ch <- metric
}
