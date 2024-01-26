# nomad-onload
# sfptpd Terraform Example
# Copyright (c) 2024 Neomantra BV
#
# Point the Provider address to your Nomad infrastracture
#
# The nomad-onload-plugin is only needed if you need Onload-enabled sfptpd;
# you can use it with muggle devices.
#
# Install the nomad-onload Plugin on the Nomad Client and set "nomad_nodename".
#

# Yes, it should be variables, but this is a demo.
locals {
  # target this to your own infrastructure
  nomad_address    = "http://localhost:4646"
  nomad_datacenter = "*"
  nomad_node       = "node1"
  nic_interface    = "eth0"
  sfptpd_image     = "onload/sfptpd:3.7.1.1007"

  # use either privileged mode to get all devices and caps
  # is_privileged = "true"
  # device_type   = ""
  # or picka "pps" or "ptp" device
  is_privileged = "false"
  device_type   = "none/ptp/ptp0"
}

terraform {
  required_providers {
    nomad = {
      source  = "hashicorp/nomad"
      version = ">= 2.0.0"
    }
  }
  required_version = ">= 1.6.2"
}

provider "nomad" {
  address = local.nomad_address
}

resource "nomad_variable" "sfptpd" {
  namespace = "default"
  path      = "conf"
  items = {
    sfptpd = file("files/ptp_slave.yml.tpl")
  }
}

resource "nomad_job" "sfptpd" {
  jobspec = templatefile("files/sfptpd.nomad.tpl", {
    NOMAD_DATACENTERS     = local.nomad_datacenter,
    NOMAD_NODE_CONSTRAINT = local.nomad_node,
    NIC_INTERFACE         = local.nic_interface,
    SFPTPD_IMAGE          = local.sfptpd_image
    IS_PRIVILEGED         = local.is_privileged
    DEVICE_TYPE           = local.device_type
  })
}
