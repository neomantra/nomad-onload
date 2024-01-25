# nomad-onload

** This is still experimental! **

`nomad-onload` is tooling to integrate [Nomad](https://www.nomadproject.io) and [OpenOnload](https://github.com/Xilinx-CNS/onload).

It provides a [Nomad Device Plugin](https://developer.hashicorp.com/nomad/docs/concepts/plugins/devices) that exposes OpenOnload capabilites to Nomad via virtual devices.  This enables kernel-bypass of the networking stack of any Docker-driver Nomad Job.  In addition to TCP and UDP acceleration, facilities like `epoll` and pipes are brought to userspace as well.

After [installing the plugin](#installation) and adding `device "onload" {}` to a Nomad Tasks' `resources` stanza, that container will become Onload-accelerated!  With proper tuning, you can get extreme performance :sunglasses:.

Running high-performance kernel-bypass workloads is a vast topic. This [High Performance Redis](https://gist.github.com/neomantra/3c9b89887d19be6fa5708bf4017c0ecd) gist includes an introduction to it with Onload and kernel tuning.

 * [Installation](#installation)
 * [Onload Devices](#onload-devices)
 * [Timekeeping Devices](#timekeeping-devices)
 * [Plugin Configuration](#plugin-configuration)
 * [Tips](#tips)
 * [Building](#building)
 * [Motivation](#motivation)
 * [Roadmap](#roadmap)
 * [Credits and License](#credits-and-license)

----

## Installation

Binaries for multiple platforms are [released on GitHub](https://github.com/neomantra/nomad-onload/releases).

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

## Onload Devices

When [installed](#installation), this plugin will discover an Onload installation and make [plugin-based devices](https://developer.hashicorp.com/nomad/docs/concepts/plugins/devices) available to Nomad Clients.  The plugin publishes the following "device types"; which are available depend on whether the host OS has installed Onload and [TCPDirect]().

| Device Type | Onload? | TCPDirect? | Notes |
|:-----|:-------:|:----------:|:------|
| | N | N | Nothing mounted. No devices published, even with SFC hardware |
| `onload` | Y | N | Onload mounted, `LD_PRELOAD` per `set_preload` config |
| `zf` | N | Y | Only TCPDirect mounted, `LD_PRELOAD` skipped |
| `onloadzf` | Y | Y |  Like `onload`, but TCPDirect is also mounted. |

When a one of those device types, such as `onload` is specified in a Nomad Job's [resource stanza](https://developer.hashicorp.com/nomad/docs/job-specification/resources#device),
then the plugin will install Onload binaries and libraries and device files into the Task,
and optionally `LD_PRELOAD` Onload.   Onload performance tuning may be applied via its various `EF_` environment variable knobs.


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

----

Here is how Onload devices are fingerprinted:

 * If Onload/TCPDirect is not installed, there are no devices available.
 * Each "SFC interface" (Solarflare/Xilinx/AMD Network Card) is discovered with Vendor `amd`
 * If there are no SFC interfaces found, we create a fake one called `none`

So if we have both Onload and TCPDirect installed along with two SFC interfces `eth0` and `eth1`, we'd have the following devices available to a Nomad Client:
  * `amd/onload/eth0` `amd/zf/eth0` `amd/onloadzf/eth0`
  * `amd/onload/eth1` `amd/zf/eth1` `amd/onloadzf/eth1`

Or similarly, with Onload and TCPDirect installed, but without SFC interfaces:
 * `amd/onload/none` `amd/zf/none` `amd/onloadzf/none`

Nomad allows devices to be selected per this [device name](https://developer.hashicorp.com/nomad/docs/job-specification/device#name):

 * `<device_type>`
 * `<vendor>/<device_type>`
 * `<vendor>/<device_type>/<model>`

Thus, by simply specifying the Device Type name `onload`, we get the Onload capability.  However, the full information can be used in `name`, as well as the attributes used in `contraint` and `affinity`.

## Timekeeping Devices

If configured with `probe_pps` or `probe_ptp`, this plugin will also detect devices under `/dev/pps*` and `/dev/ptp*`.  The will be made available as `pps` and `ptp` device types.

 * `<vendor>/pps/<interface>`
 * `<vendor>/ptp/<interface>`

## Plugin Configuration

The following settings are available to configure the plugin behavior, per above.

Devices and libraries are always installed when a `nomad-onload` device is used in the `resources` stanza.  Setting any partiular path to empty string `""` will disable that mount.  For example, `host_profile_dir_path = ""` will prevent mount profiles.

If `mount_onload` is enables mounting of all the files and paths configured below it,
  All mounts are read-only.

| Name | Type | Default | Description |
|:-----|:----:|:-------:|:------------|
| `set_preload` | `bool` | `true` | Should the Device Plugin set the `LD_PRELOAD` environment variable in the Nomad Task? |
| `mount_onload` | `bool` | `true` | Should the Device Plugin mount Onload files into the Nomad Task? |
| `probe_nic` | `bool` |  | `true` | Should the Device Plugin probe for Onload-enabled NICs? |
| `probe_xdp` | `bool` |  | `true` | Should the Device Plugin probe for Onload-enabled XDP? **NOT IMPLEMENTED** |
| `probe_pps` | `bool` |  | `true` | Should the Device Plugin probe for PPS devices? |
| `probe_ptp` | `bool` |  | `true` | Should the Device Plugin probe for PTP devices? |
| `ignored_interfaces` | `list(string)` | `[]` | List of interfaces to ignore.  Include `none` to prevent that pseudo-devices creation |
| `num_nic` | `number` | `false` | `10` | Number of psuedo-devices per NIC device, limiting the number of simultaneous Onloaded Jobs |
| `num_pps` | `number` | `false` | `10` | Number of psuedo-devices per PPS device, limiting the number of simultaneous PPS device claims |
| `num_ptp` | `number` | `false` | `10` | Number of psuedo-devices per PTP device, limiting the number of simultaneous PTP device claims |
| `task_device_path` | `string` | `"/dev"` | Path to place device files in the Nomad Task |
| `host_device_path` | `string` | `"/dev"` | Path to find device files on the Host |
| `task_onload_lib_path` | `string` | `"/opt/onload/usr/lib64"` | Path to place Onload libraries in the Nomad Task |
| `host_onload_lib_path` | `string` | `"/usr/lib64"` | Path to find Onload libraries on the Host |
| `task_onload_bin_path` | `string` | `"/opt/onload/usr/bin"` | Path to place Onload binaries in the Nomad Task |
| `host_onload_bin_path` | `string` | `"/usr/bin"` | Path to find Onload binaries on the Host |
| `task_profile_dir_path` | `string` | `" /usr/libexec/onload/profiles"` | Path to place Onload profile directory in the Nomad Task |
| `host_profile_dir_path` | `string` | `" /usr/libexec/onload/profiles"` | Path to find Onload profile directory on the Host |
| `task_zf_bin_path` | `string` | `"/opt/onload/usr/bin/"` | Path to place TCPDirect/ZF binaries in the Nomad Task |
| `host_zf_bin_path` | `string` | `"/usr/bin"` | Path to find TCPDirect/ZF binaries on the Host |
| `task_zf_lib_path` | `string` | `"/opt/onload/usr/bin"` | Path to place TCPDirect/ZF libraries in the Nomad Task |
| `host_zf_lib_path` | `string` | `"/usr/lib64"` | Path to find TCPDirect/ZF libraries on the Host |
| `fingerprint_period` | `string` | `"1m"` | Period of time between attemps to fingerpint devices |

## Tips

The binary distribution includes `nomad-onload-probe`, which scans a system using the same code as `nomad-onload-device`:

```
$ ./bin/nomad-onload-probe 
Onload version: 8.1.2.26
TCPDirect version: 8.1.2
Onload hardware-accelerated interfaces:
  eth0     0000:b1:00.0
  eth1     0000:b1:00.1
XDP hardware-accelerated interfaces: (FAKE, ROADMAP)
PPS devices:
  /dev/pps0 
PTP devices:
  /dev/ptp0 
  /dev/ptp1 
  /dev/ptp2
```

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

 * `nomad-onload-probe` (simple test tool)
 * `nomad-onload-device` (the plugin)

```
$ task
task: [tidy] go mod tidy
task: [tidy] go mod tidy
task: [tidy] go mod tidy
task: [install-deps] go build -o ./bin/launcher github.com/hashicorp/nomad/plugins/shared/cmd/launcher
task: [build-onload-probe] go build -o ./bin/nomad-onload-probe cmd/onload-probe/*.go
task: [build-plugin] go build -o ./bin/nomad-onload-device cmd/nomad-device-onload/*.go
```

We publish with [GitHub Actions](https://github.com/neomantra/nomad-onload/actions) and [Goreleaser](https://goreleaser.com).

## Motivation

When using Onload in containerized environment, all the Onload devices, libraries, and executables need to be present *inside* the container.  Furthermore, the versions of everything need to match *exactly* between Host and Container.

One way to manage this is to build Onload into your image.   Neomantra maintains [`docker-onload` tooling](https://github.com/neomantra/docker-onload) which create [Onload-enabled Docker base images](https://hub.docker.com/r/neomantra/onload).  You can then build your application on top of those base images.  If you maintain multiple Onload versions in your cluster, you would need to apply a CI/CD build matrix of all your Dockerfiles and all your Onload versions.

If you wanted to Onload-enable a third-party application, such as Redis, you would need to either build Redis from an Onload base image, or you would need to add the matching binaries/libraries to a new image derived from a Redis image.

Then, when you actually want to *run* the image, you must hook up `/usr/bin/onload` to activate `LD_PRELOAD`, and you would need to tell Docker to mount devices like `/dev/sfc`.  Clearly this is all cumbersome, but it is necessary.   Typically, teams build scripts and tooling to manage this complexity.

Cluster Orchestrators can help with this as they manage the control plane and prepare Containers for launch.  The Onload team released [Kubernetes Onload](https://github.com/Xilinx-CNS/kubernetes-onload), which provides aKubernetes Operator and resources for automatically injecting the require enviornment into a Kubernetes Pod.

Here we are with this same capability for [HashiCorp Nomad](https://www.nomadproject.io).   Simply ask Nomad for an `onload` device for **any Docker-driver Nomad Job** and the plugin will take care of the rest of the plumbing!

## Roadmap

 * [ ] Device Attributes
 * [ ] Device Statistics
 * [ ] Redis example
 * [ ] XDP example

## Credits and License

Thanks to the [Nomad](https://www.nomadproject.io) and [Onload](https://github.com/Xilinx-CNS/onload) teams and the organizations that have supported their open collaboration!

Much of the code has been reviewed and adapted from:
 * [Nomad Skeleton Device Plugin](https://github.com/hashicorp/nomad-skeleton-device-plugin)
 * [Nvidia Device Plugin](https://github.com/hashicorp/nomad-device-nvidia)
 * [Kubernetes Onload](https://github.com/Xilinx-CNS/kubernetes-onload)

Made with :heart: and :fire: by Evan Wies.

Copyright (c) 2024 [Neomantra BV](https://www.neomantra.com).

Released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE.txt](./LICENSE.txt).