# DHCP Server Device — the network's "DHCP Server Device" setting is driven by
# `device_type`. Supported values: external_device, gateway, switch, none.

# none — no DHCP server on this L2-only VLAN (addresses served elsewhere).
resource "omada_lan_network" "vlan_dhcp_none" {
  name        = "IOT"
  vlan_id     = 30
  device_type = "none"
}

# external_device — an upstream/external DHCP server serves this VLAN.
resource "omada_lan_network" "vlan_dhcp_external" {
  name        = "Guest"
  vlan_id     = 40
  device_type = "external_device"
}
