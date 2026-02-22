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

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	v0 "github.com/Project-HAMi/HAMi-DRA/pkg/monitor/v0"
	v1 "github.com/Project-HAMi/HAMi-DRA/pkg/monitor/v1"
	"github.com/Project-HAMi/HAMi-DRA/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const SharedRegionMagicFlag = 19920718

type headerT struct {
	initializedFlag int32
	majorVersion    int32
	minorVersion    int32
}

// ContainerLister scans the host file system for HAMi-core .cache files
// created under containers/{podUID}_{containerName}/ and mmap-s them
// to provide real-time GPU memory/utilization data.
type ContainerLister struct {
	containerPath string
	containers    map[string]*ContainerUsage
	mutex         sync.Mutex
	clientset     kubernetes.Interface
	nodeName      string

	informerFactory informers.SharedInformerFactory
	podLister       corelisters.PodLister
	podListerSynced cache.InformerSynced
	stopCh          chan struct{}
}

var resyncInterval = 5 * time.Minute

// NewContainerLister creates a ContainerLister that monitors cache files
// under hookPath/containers/ for the given node.
func NewContainerLister(hookPath, nodeName string) (*ContainerLister, error) {
	if hookPath == "" {
		return nil, fmt.Errorf("hookPath must not be empty")
	}
	if nodeName == "" {
		return nil, fmt.Errorf("nodeName must not be empty")
	}

	client, err := utils.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	lister := &ContainerLister{
		containerPath: filepath.Join(hookPath, "containers"),
		containers:    make(map[string]*ContainerUsage),
		clientset:     client.Interface,
		nodeName:      nodeName,
		stopCh:        make(chan struct{}),
	}

	if err := lister.initInformer(); err != nil {
		return nil, err
	}

	return lister, nil
}

// ListContainers returns the current map of container usages (key = dirName).
func (l *ContainerLister) ListContainers() map[string]*ContainerUsage {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Return a shallow copy to avoid holding the lock
	result := make(map[string]*ContainerUsage, len(l.containers))
	for k, v := range l.containers {
		result[k] = v
	}
	return result
}

// Update scans the containers directory, loads new cache files and removes stale ones.
func (l *ContainerLister) Update() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entries, err := os.ReadDir(l.containerPath)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(5).Infof("Container path %s does not exist yet, skipping", l.containerPath)
			return nil
		}
		return err
	}

	pods, err := l.podLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	podUIDs := make(map[string]bool, len(pods))
	for _, pod := range pods {
		podUIDs[string(pod.UID)] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := filepath.Join(l.containerPath, entry.Name())
		podUID := strings.Split(entry.Name(), "_")[0]
		if !podUIDs[podUID] {
			dirInfo, err := os.Stat(dirName)
			if err == nil && dirInfo.ModTime().Add(resyncInterval).After(time.Now()) {
				continue
			}
			klog.Infof("Removing dirname %s in monitorpath", dirName)
			if c, ok := l.containers[entry.Name()]; ok {
				syscall.Munmap(c.Data)
				delete(l.containers, entry.Name())
			}
			_ = os.RemoveAll(dirName)
			continue
		}
		if _, ok := l.containers[entry.Name()]; ok {
			continue
		}
		usage, err := loadCache(dirName)
		if err != nil {
			klog.Errorf("Failed to load cache: %s, error: %v", dirName, err)
			continue
		}
		if usage == nil {
			// no cuInit in container
			continue
		}
		usage.PodUID = podUID
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) == 2 {
			usage.ContainerName = parts[1]
		}
		l.containers[entry.Name()] = usage
		klog.Infof("Adding ctr dirname %s in monitorpath", dirName)
	}
	return nil
}

// Stop shuts down the informer.
func (l *ContainerLister) Stop() {
	close(l.stopCh)
}

func loadCache(fpath string) (*ContainerUsage, error) {
	klog.V(5).Infof("Checking path %s", fpath)
	files, err := os.ReadDir(fpath)
	if err != nil {
		return nil, err
	}
	if len(files) > 2 {
		return nil, errors.New("cache num not matched")
	}
	if len(files) == 0 {
		return nil, nil
	}
	cacheFile := ""
	for _, val := range files {
		if strings.Contains(val.Name(), "libvgpu.so") {
			continue
		}
		if !strings.Contains(val.Name(), ".cache") {
			continue
		}
		cacheFile = filepath.Join(fpath, val.Name())
		break
	}
	if cacheFile == "" {
		klog.V(5).Infof("No cache file in %s", fpath)
		return nil, nil
	}
	info, err := os.Stat(cacheFile)
	if err != nil {
		klog.Errorf("Failed to stat cache file: %s, error: %v", cacheFile, err)
		return nil, err
	}
	if info.Size() < int64(unsafe.Sizeof(headerT{})) {
		return nil, fmt.Errorf("cache file size %d too small", info.Size())
	}
	f, err := os.OpenFile(cacheFile, os.O_RDWR, 0666)
	if err != nil {
		klog.Errorf("Failed to open cache file: %s, error: %v", cacheFile, err)
		return nil, err
	}
	defer f.Close()

	usage := &ContainerUsage{}
	usage.Data, err = syscall.Mmap(int(f.Fd()), 0, int(info.Size()), syscall.PROT_WRITE|syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		klog.Errorf("Failed to mmap cache file: %s, error: %v", cacheFile, err)
		return nil, err
	}
	head := (*headerT)(unsafe.Pointer(&usage.Data[0]))
	if head.initializedFlag != SharedRegionMagicFlag {
		_ = syscall.Munmap(usage.Data)
		return nil, fmt.Errorf("cache file magic flag not matched")
	}
	// v0 cache files have a fixed known size
	if info.Size() == 1197897 {
		klog.V(4).Infoln("casting......v0")
		usage.Info = v0.CastSpec(usage.Data)
	} else if head.majorVersion == 1 {
		klog.V(4).Infoln("casting......v1")
		usage.Info = v1.CastSpec(usage.Data)
	} else {
		_ = syscall.Munmap(usage.Data)
		return nil, fmt.Errorf("unknown cache file size %d version %d.%d", info.Size(), head.majorVersion, head.minorVersion)
	}
	return usage, nil
}

func (l *ContainerLister) initInformer() error {
	l.informerFactory = informers.NewSharedInformerFactoryWithOptions(
		l.clientset,
		resyncInterval,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("spec.nodeName=%s", l.nodeName)
		}),
	)

	podInformer := l.informerFactory.Core().V1().Pods()
	l.podLister = podInformer.Lister()
	l.podListerSynced = podInformer.Informer().HasSynced

	l.informerFactory.Start(l.stopCh)
	if !cache.WaitForCacheSync(l.stopCh, l.podListerSynced) {
		return fmt.Errorf("failed to sync pod informer cache")
	}

	klog.Info("ContainerLister pod informer started successfully")
	return nil
}
