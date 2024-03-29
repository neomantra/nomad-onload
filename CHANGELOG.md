Changelog
=========

## v0.5.0 (2024-03-23)

 * Homogenize binary names to `nomad-device-onload` and `nomad-probe-onload`
   for consistency with other Nomad Device plugins.

## v0.4.1 (2024-01-25)

 * Added `ptp` and `pps` device types. I'm more of a bottom, but I do like disciplining clocks.
 * Added `sfptpd` example at [`examples/terraform/sfptpd`](./examples/terraform/sfptpd/main.tf)

## v0.3.0 (2024-01-24)

 * Add `num_pseudo` to control the number of `onload` psuedo-devices are created.

## v0.2.0 (2024-01-21)

 * Add PCIBusID probing
 * Remove `need_nic` config
 * Add control of copying through empty strings
 
## v0.1.0 (2024-01-20)

 * Initial release
