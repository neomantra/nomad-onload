# `sfptpd` Nomad System Job Example

Stands up sfptpd as a Nomad System Job using Terraform.  It requires `nomad-onload` plugin installed to discover the `ptp` devices.

```
cd examples/terraform/sfptpd
terraform init
terraform apply
```

This is my first stab at this, so don't take it as anything more than demonstrative.  But I do get clock sync.
