# nomad-onload

`nomad-onload` is tooling to integrate [Nomad](https://www.nomadproject.io) and [OpenOnload](https://github.com/Xilinx-CNS/onload).

It provides a [Nomad Device Plugin](https://developer.hashicorp.com/nomad/docs/concepts/plugins/devices) that exposes OpenOnload capabilites to Nomad via virtual devices.  This enables kernel-bypass of the networking stack of any Docker-driver Nomad Job.  In addition to TCP and UDP acceleration, facilities like  `epoll` and pipes are brought to userspace as well.

When [installed](./installation), this plugin will discover an Onload installation and make [plugin-based devices](https://developer.hashicorp.com/nomad/docs/concepts/plugins/devices) available to Nomad Clients.  The plugin publishes the following "device types"; which are available depend on whether the host OS has installed Onload and [TCPDirect]().

| Device Type | Onload? | TCPDirect? | Notes |
|:-----|:-------:|:----------:|:------|
| | N | N | Nothing mounted. No devices published, even with SFC hardware |
| `onload` | Y | N | Onload mounted, `LD_PRELOAD` per `set_preload` config |
| `zf` | N | Y | Only TCPDirect mounted, `LD_PRELOAD` skipped |
| `onloadzf` | Y | Y |  Like `onload`, but TCPDirect is also mounted. |

When a one of those device types, such as `onload` is specified in a Nomad Job's [resource stanza](https://developer.hashicorp.com/nomad/docs/job-specification/resources#device),
then the plugin will install Onload binaries and libraries and device files into the Task,
and optionally `LD_PRELOAD` Onload.   Onload performance tuning may be applied via its environment variables:

```hcl
task {
  env {
    EF_TCP_SERVER_LOOPBACK = "2"
    EF_TCP_CLIENT_LOOPBACK = "4"
  } 
  resources {
    device "onload" {}
  }
}
```

You may also be interested in Neomantra's [`docker-onload` tooling](https://github.com/neomantra/docker-onload) for [Onload-enabled Docker images](https://hub.docker.com/r/neomantra/onload).  Running high-performance kernel-bypass workloads is a vast topic; this  [High Performance Redis](https://gist.github.com/neomantra/3c9b89887d19be6fa5708bf4017c0ecd) gist includes an introduction.


## Installation

Binaries for multiple platforms are [released on GitHub](https://github.com/neomantra/nomad-onload/releases) through [GitHub Actions](https://github.com/neomantra/nomad-onload/actions).

To install the Nomad Onload Device Plugin on your Nomad Client instance,
copy the `nomad-onload-device` binary to the host's [Nomad `plugin_dir`](https://developer.hashicorp.com/nomad/docs/configuration#plugin_dir) (e.g. `/opt/nomad/data/plugins`).

Then add a [`plugin` config stanza](https://developer.hashicorp.com/nomad/docs/configuration/plugin) to your Nomad configuration:

```hcl
# onload.hcl
plugin "nomad-onload-device" {
  config {
    # Mount Onload into images
    mount_onload = true
  }
}
```


## Plugin Configuration

The following settings are available to configure the plugin behavior, per above:

| Name | Type | Default | Description |
|:-----|:----:|:-------:|:------------|
| `need_nic` | `bool` | `true` | Should the Device Plugin fail if no compatible nics are found |
| `set_preload` | `bool` | `true` | Should the device plugin set the LD_PRELOAD environment variable in the Task |
| `mount_onload` | `bool` | `false` | Should the device plugin mount Onload files into the Nomad Task |
| `ignored_interfaces` | `list(string)` | `[]` | List of interfaces to ignore.  Include `none` to prevent that pseudo-devices creation |
| `task_device_path` | `string` | `"/dev"` | Path to place device files in the Nomad Task |
| `host_device_path` | `string` | `"/dev"` | Path to find device files on the Host |
| `task_onload_lib_path` | `string` | `"/opt/onload/usr/lib64/"` | Path to place Onload libraries in the Nomad Task |
| `host_onload_lib_path` | `string` | `"/usr/lib64"` | Path to find Onload libraries on the Host |
| `task_onload_bin_path` | `string` | `"/opt/onload/usr/bin/"` | Path to place Onload binaries in the Nomad Task |
| `host_onload_bin_path` | `string` | `"/usr/bin"` | Path to find Onload binaries on the Host |
| `task_zf_bin_path` | `string` | `"/opt/onload/usr/bin/"` | Path to place TCPDirect/ZF binaries in the Nomad Task |
| `host_zf_bin_path` | `string` | `"/usr/bin"` | Path to find TCPDirect/ZF binaries on the Host |
| `task_zf_lib_path` | `string` | `"/opt/onload/usr/bin/"` | Path to place TCPDirect/ZF libraries in the Nomad Task |
| `host_zf_lib_path` | `string` | `"/usr/lib64"` | Path to find TCPDirect/ZF libraries on the Host |
| `fingerprint_period` | `string` | `"1m"` | Period of time between attemps to fingerpint devices |

----

# `onload_stackdump`

You can run `onload_stackdump` inside the container, but you must remove `LD_PRELOAD` first:

```
$ docker exec -it -e LD_PRELOAD= redis-d3e926be-72c4-1940-953b-7e1bbb7a75dd  /usr/bin/onload_stackdump lots | head
============================================================
ci_netif_dump_to_logger: stack=6 name=
  cplane_pid=142738
  namespace=net:[4026531840]
  Onload 8.1.2.26 uid=999 pid=1 ns_flags=80
...
```

## Building

Building is performed with [Taskfile](https://taskfile.dev/), creating the following binaries:

 * `onload-probe` (simple test tool)
 * `nomad-onload-device-plugin` (the plugin)

```
$ task
task: [tidy] go mod tidy
task: [tidy] go mod tidy
task: [tidy] go mod tidy
task: [install-deps] go build -o ./bin/launcher github.com/hashicorp/nomad/plugins/shared/cmd/launcher
task: [build-onload-probe] go build -o ./bin/onload-probe cmd/onload-probe/*.go
task: [build-plugin] go build -o ./bin/nomad-device-plugin-onload cmd/nomad-device-onload/*.go
```

## Roadmap

 * [ ] Mount profiles
 * [ ] `PCIBusID`
 * [ ] `HealthDesc`
 * [ ] Device Statistics
 * [ ] Device Attributes
 * [ ] Redis example
 * [ ] XDP example

## Credits and License

Thanks to the [Nomad](https://www.nomadproject.io) and [Onload](https://github.com/Xilinx-CNS/onload) teams and the organizations that have supported their open collaboration!

Much of the code has been reviewed and adapted from:
 * [Nomad Skeleton Device Plugin](https://github.com/hashicorp/nomad-skeleton-device-plugin)
 * [Nvidia Device Plugin](https://github.com/hashicorp/nomad-device-nvidia)
 * [Kubernetes Onload](https://github.com/Xilinx-CNS/kubernetes-onload)

Copyright (c) 2024 [Neomantra BV](https://www.neomantra.com).  Authored by Evan Wies.

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).