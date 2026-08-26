# DHCP Server Device — the network's "DHCP Server Device" setting is driven by
# `device_type`. Supported values: external_device, gateway, switch, none.

# none — no DHCP server on this L2-only VLAN (addresses served elsewhere).
resource "omada_network" "vlan_dhcp_none" {
  name        = "IOT"
  purpose     = "vlan"
  vlan_id     = 30
  device_type = "none"
}

# external_device — an upstream/external DHCP server serves this VLAN.
resource "omada_network" "vlan_dhcp_external" {
  name        = "Guest"
  purpose     = "vlan"
  vlan_id     = 40
  device_type = "external_device"
}

# gateway — the Omada gateway is the DHCP server (a routed interface network).
resource "omada_network" "vlan_dhcp_gateway" {
  name           = "Home"
  purpose        = "interface"
  vlan_id        = 50
  gateway_subnet = "10.10.50.1/24"
  dhcp_enabled   = true
  dhcp_start     = "10.10.50.100"
  dhcp_end       = "10.10.50.250"
  device_type    = "gateway"
}
