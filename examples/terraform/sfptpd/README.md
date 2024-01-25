# `sfptpd` Nomad System Job Example

Stands up `sfptpd` as a Nomad System Job using  [Terraform](https://www.terraform.io).  It requires `nomad-onload` plugin installed to discover the `ptp` devices.

For timekeeping to be avilable, the job must have several capabilities added.   The Nomad Job template requests it with [`cap_add` Docker driver config](https://developer.hashicorp.com/nomad/docs/drivers/docker#cap_add), but the Nomad Client must also be configured to allow it with [`allow_caps`](https://developer.hashicorp.com/nomad/docs/drivers/docker#allow_caps) and possibly `privileged` as well.

Here's a matrix of settings for the `local`` values:

| `is_privileged` | `device_type` | Result |
|-----------------|---------------|------- |
| `true`  | any | PTP/PPS devices available due to `privileged = true` and Docker settings |
| `false`| `"pps"` or `"ptp"` | PPS or PTP devices will be availble |
| `false`| `""` | Will run but **will not sync** because no timekeeping devices |

```
plugin "docker" {
  config {
    ....
    allow_privileged = true
    allow_caps = [
      # default: https://developer.hashicorp.com/nomad/docs/drivers/docker#allow_caps
      "audit_write", "chown", "dac_override", "fowner", "fsetid", "kill", "mknod",
      "net_bind_service", "setfcap", "setgid", "setpcap", "setuid", "sys_chroot",
      # timekeeping
      "net_admin", "net_raw", "sys_time"
    ]
  }
```

----

To stand it up with Terraform:

```
cd examples/terraform/sfptpd
terraform init
terraform apply
```

This is my first stab at this, so don't take it as anything more than demonstrative.  But I do get clock sync.

