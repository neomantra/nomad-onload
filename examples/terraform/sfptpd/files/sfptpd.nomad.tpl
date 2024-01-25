# nomad-onload sfptpd Nomad Job Template
# Copyright (c) 2024 Neomantra BV

job "sfptpd" {
  region = "global"
  datacenters = ["${NOMAD_DATACENTERS}"]
  type = "system"

  constraint {
    attribute = "$${node.unique.name}"
    value     = "${NOMAD_NODE_CONSTRAINT}"
  }

  update {
    max_parallel = 1
    stagger      = "1m"
    auto_revert  = true
  }

  group "sfptpd" {
    count = 1

    ephemeral_disk {
      sticky = true
      migrate = true
      size = 1024
    }
  
    task "sfptpd" {
      driver = "docker"
      config {
        image = "${SFPTPD_IMAGE}"
        args = [
          "-v", 
          "-i", "${NIC_INTERFACE}",
          "-f", "$${NOMAD_ALLOC_DIR}/conf/sfptpd.cfg",
        ]

        # Sorry, the nomad-onload plugin can't do *everything* for you!
        network_mode = "host"
        privileged = true
        cap_add = [
          "net_bind_service",
          "net_admin",
          "net_raw",
          "sys_time"
        ]
      }
      resources {
        device "ptp" {}
        %{~ if lower("${ONLOAD_ENABLED}") == "true" ~}
        device "onload" {}
        %{~ endif ~} 
      }

      template {
        destination = "$${NOMAD_ALLOC_DIR}/conf/sfptpd.cfg"
        change_mode = "restart"
        data        = <<EOT
{{ with nomadVar "conf" }}{{ .sfptpd }}{{ end }}"
EOT
      }
    }
  }
}