config {
  # The nomad-onload defaults are geared for harmony with the 
  # latest Onload Kubernetes releases.
  #
  # The following configs are how it may be run on raw Ubuntu.

  host_onload_lib_path = "/usr/lib/x86_64-linux-gnu/"
  host_onload_bin_path = "/usr/bin"
  host_zf_lib_path     = "/usr/lib/x86_64-linux-gnu/"
  host_zf_bin_path     = "/usr/bin"

  mount_onload = true
}