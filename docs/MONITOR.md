# Monitor Component Documentation

The monitor component is an optional feature of HAMi DRA Webhook that collects and exposes GPU resource metrics via Prometheus.

## Overview

The monitor component watches Kubernetes ResourceSlice and ResourceClaim resources, maintains an in-memory cache of GPU device allocations, and exposes Prometheus metrics for monitoring GPU resource usage across the cluster.


## Installation

The monitor component is enabled by default when installing the Helm chart. To disable it:

```bash
helm install hami-dra ./charts/hami-dra \
--set monitor.enabled=false
```

## Configuration

### Basic Configuration

Configure the monitor in `charts/hami-dra/values.yaml`:

```yaml
monitor:
  enabled: true
  replicas: 1
  logLevel: 2
  metricsBindAddress: ":8080"
  healthProbeBindAddress: ":8000"
  kubeAPIQPS: 40.0
  kubeAPIBurst: 60
  collectInterval: "30s"
```

**Configuration Parameters**:

- `enabled`: Enable or disable the monitor component (default: `true`)
- `replicas`: Number of monitor pod replicas (default: `1`)
- `logLevel`: Log verbosity level (default: `2`)
- `metricsBindAddress`: Address and port for metrics endpoint (default: `:8080`)
- `healthProbeBindAddress`: Address and port for health probe endpoints (default: `:8000`)
- `kubeAPIQPS`: QPS limit for Kubernetes API client (default: `40.0`)
- `kubeAPIBurst`: Burst limit for Kubernetes API client (default: `60`)
- `collectInterval`: Interval for metrics collection (default: `30s`)

### Service Configuration

The monitor service can be configured to use different service types depending on your access requirements.

#### ClusterIP (Default)

Use ClusterIP for internal cluster access:

```yaml
monitor:
  enabled: true
  service:
    type: ClusterIP
```

Access metrics via port-forward:
```bash
kubectl port-forward svc/hami-dra-monitor 8080:8080 -n <namespace>
curl http://localhost:8080/metrics
```

#### NodePort

Use NodePort to expose metrics outside the cluster:

**With specified ports**:
```yaml
monitor:
  enabled: true
  service:
    type: NodePort
    nodePort:
      metrics: 30080  # NodePort for metrics endpoint
```

**With auto-assigned ports**:
```yaml
monitor:
  enabled: true
  service:
    type: NodePort
    nodePort:
      metrics: ""  # Kubernetes will assign a random port
```

Access metrics via NodePort:
```bash
# Get the NodePort
kubectl get svc hami-dra-monitor -n <namespace> -o jsonpath='{.spec.ports[?(@.name=="metrics")].nodePort}'

# Access metrics
curl http://<node-ip>:<nodeport>/metrics
```

#### LoadBalancer

Use LoadBalancer for cloud provider load balancer integration:

```yaml
monitor:
  enabled: true
  service:
    type: LoadBalancer
```

## Exposed Metrics

The monitor exposes the following Prometheus metrics:

### Node-Level Metrics

#### GPUDeviceMemoryLimit
Device memory limit for a GPU (in MB).

**Labels**:
- `nodeid`: Kubernetes node name
- `deviceuuid`: GPU device UUID
- `deviceidx`: Device index on the node
- `devicename`: Device name
- `devicebrand`: Device brand (e.g., Tesla)
- `deviceproductname`: Device product name (e.g., Tesla V100)

**Example**:
```
GPUDeviceMemoryLimit{nodeid="node1", deviceuuid="gpu-uuid-123", deviceidx="0", devicename="gpu0", devicebrand="Tesla", deviceproductname="Tesla V100"} 16000
```

#### GPUDeviceCoreLimit
Device core limit for a GPU.

**Labels**: Same as `GPUDeviceMemoryLimit`

#### GPUDeviceMemoryAllocated
Device memory currently allocated for a GPU (in MB).

**Labels**: Same as `GPUDeviceMemoryLimit`

#### GPUDeviceCoreAllocated
Device cores currently allocated for a GPU.

**Labels**: Same as `GPUDeviceMemoryLimit`

### Pod-Level Metrics

#### vGPUDeviceMemoryAllocated
vGPU device memory allocated for a container (in MB).

**Labels**:
- `nodeid`: Kubernetes node name
- `deviceuuid`: GPU device UUID
- `deviceidx`: Device index on the node
- `devicename`: Device name
- `devicebrand`: Device brand
- `deviceproductname`: Device product name
- `podnamespace`: Pod namespace
- `podname`: Pod name

**Example**:
```
vGPUDeviceMemoryAllocated{nodeid="node1", deviceuuid="gpu-uuid-123", deviceidx="0", devicename="gpu0", devicebrand="Tesla", deviceproductname="Tesla V100", podnamespace="default", podname="my-pod"} 8000
```

#### vGPUDeviceCoreAllocated
vGPU device cores allocated for a container.

**Labels**: Same as `vGPUDeviceMemoryAllocated`

### Container-Level Real-Time Metrics (HAMi-core shared memory)

These metrics are read directly from HAMi-core shared memory cache files on each
node. They are available only when the monitor runs in **node-level mode**
(`--node-name` and `--hook-path` are set) and the host vGPU directory is mounted
into the monitor pod.

#### Base labels (all container-level metrics)

| Label | Description |
|---|---|
| `podnamespace` | Pod namespace |
| `podname` | Pod name |
| `ctrname` | Container name |
| `vdeviceid` | Virtual device index inside the container (0, 1, …) |
| `deviceuuid` | Physical GPU UUID (first 40 chars) |

#### Extended labels (only on `vGPU_device_memory_usage_real_in_MiB` and `vGPU_device_memory_limit_in_MiB`)

| Label | Description |
|---|---|
| `pod_uid` | Kubernetes Pod UID |
| `image` | Container image with tag (e.g. `pytorch/pytorch:2.1.0`) |
| `image_id` | Container image digest (e.g. `docker.io/pytorch/pytorch@sha256:abc123…`) |
| `device_type` | GPU product name from DRA ResourceSlice (e.g. `Tesla V100-PCIE-32GB`) |

#### Metrics

| Metric | Description | Labels |
|---|---|---|
| `Device_memory_desc_of_container` | Raw device memory total (bytes) reported by HAMi-core | base |
| `Device_utilization_desc_of_container` | SM utilization (%) | base |
| `Device_last_kernel_of_container` | Seconds since the last GPU kernel executed | base |
| `vGPU_device_memory_usage_in_MiB` | cudaMalloc-tracked memory usage (MiB, rounded) | base |
| `vGPU_device_memory_usage_real_in_MiB` | Real GPU memory usage from NVML matching nvidia-smi (MiB, rounded) | base + extended |
| `vGPU_device_memory_limit_in_MiB` | Memory limit configured for this vGPU (MiB, rounded) | base + extended |
| `vGPU_device_memory_context_size_MiB` | CUDA context memory (MiB, rounded) | base |
| `vGPU_device_memory_module_size_MiB` | CUDA module memory (MiB, rounded) | base |
| `vGPU_device_memory_buffer_size_MiB` | CUDA buffer memory (MiB, rounded) | base |

#### Example

```
vGPU_device_memory_usage_real_in_MiB{
  podnamespace="default",
  podname="training-job-0",
  ctrname="trainer",
  vdeviceid="0",
  deviceuuid="GPU-7ee58073-64f3-b5c7-860d-4f8cdb7c3f4e",
  pod_uid="71a6f8ca-dc6f-497e-b791-d82fe673fb91",
  image="docker.io/deepspeed/deepspeed:latest",
  image_id="docker.io/deepspeed/deepspeed@sha256:589ec255…",
  device_type="Tesla V100-PCIE-32GB"
} 2586

vGPU_device_memory_limit_in_MiB{
  podnamespace="default",
  podname="training-job-0",
  ctrname="trainer",
  vdeviceid="0",
  deviceuuid="GPU-7ee58073-64f3-b5c7-860d-4f8cdb7c3f4e",
  pod_uid="71a6f8ca-dc6f-497e-b791-d82fe673fb91",
  image="docker.io/deepspeed/deepspeed:latest",
  image_id="docker.io/deepspeed/deepspeed@sha256:589ec255…",
  device_type="Tesla V100-PCIE-32GB"
} 4096

vGPU_device_memory_usage_in_MiB{
  podnamespace="default",
  podname="training-job-0",
  ctrname="trainer",
  vdeviceid="0",
  deviceuuid="GPU-7ee58073-64f3-b5c7-860d-4f8cdb7c3f4e"
} 2400
```

## Endpoints

### Metrics Endpoint

- **Path**: `/metrics`
- **Port**: `8080` (configurable via `metricsBindAddress`)
- **Format**: Prometheus text format
- **Access**: `http://<service-address>:8080/metrics`

### Health Check Endpoints

- **Liveness Probe**: `/healthz` on port `8000`
- **Readiness Probe**: `/readyz` on port `8000`
- **Access**: `http://<service-address>:8000/healthz` or `/readyz`

The readiness probe returns `200 OK` when the cache is synced and ready, `503 Service Unavailable` otherwise.

## Prometheus Integration

### Service Discovery Configuration

To automatically discover and scrape metrics from the monitor, add the following to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'hami-dra-monitor'
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - <monitor-namespace>  # Replace with your namespace
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name]
        action: keep
        regex: hami-dra-monitor
      - source_labels: [__meta_kubernetes_service_port_name]
        action: keep
        regex: metrics
```

### Static Configuration

Alternatively, you can use static configuration:

```yaml
scrape_configs:
  - job_name: 'hami-dra-monitor'
    static_configs:
      - targets:
        - 'hami-dra-monitor.<namespace>.svc.cluster.local:8080'
```

## Resource Requirements

Default resource requests and limits:

```yaml
monitor:
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

Adjust these values based on your cluster size and monitoring requirements.

## Troubleshooting

### Check Monitor Status

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/component=monitor -n <namespace>

# Check logs
kubectl logs -l app.kubernetes.io/component=monitor -n <namespace>

# Check service
kubectl get svc hami-dra-monitor -n <namespace>
```

### Verify Metrics Endpoint

```bash
# Port-forward to access metrics
kubectl port-forward svc/hami-dra-monitor 8080:8080 -n <namespace>

# Check metrics
curl http://localhost:8080/metrics | grep GPUDevice
```

### Check Cache Sync Status

The monitor requires the cache to be synced before it can collect metrics. Check the logs for:

```
Cache started and synced successfully
Cache is ready
```

If the cache fails to sync, check:
- RBAC permissions for ResourceSlice and ResourceClaim resources
- Network connectivity to the Kubernetes API server
- ResourceSlice and ResourceClaim resources exist in the cluster

## Architecture

The monitor component consists of:

1. **Cache Layer**: Maintains an in-memory cache of ResourceSlice and ResourceClaim resources
2. **Metrics Collector**: Implements Prometheus Collector interface to gather metrics from the cache
3. **HTTP Servers**: 
   - Metrics server on port 8080
   - Health probe server on port 8000

The monitor uses Kubernetes informers to watch ResourceSlice and ResourceClaim resources, ensuring the cache stays up-to-date with cluster state.

## Performance Considerations

- **Cache Sync**: The monitor waits for ResourceSlice cache to sync before processing ResourceClaim events to ensure data consistency
- **Concurrency**: Uses node-level locking to minimize contention when updating device usage
- **API Rate Limiting**: Configure `kubeAPIQPS` and `kubeAPIBurst` to control API server load
- **Metrics Collection**: Metrics are collected on-demand when Prometheus scrapes the endpoint, not on a fixed interval

