# Manage a wired LAN network (802.1Q VLAN). Choose the DHCP Server Device with
# `device_type`.

resource "omada_lan_network" "vlan_none" {
  name        = "IOT"
  vlan_id     = 30
  device_type = "none"
}

resource "omada_lan_network" "vlan_external" {
  name        = "Guest"
  vlan_id     = 40
  device_type = "external_device"
}

# `gateway` is the DHCP server; it requires a gateway_subnet (CIDR).
resource "omada_lan_network" "vlan_gateway" {
  name           = "Management"
  vlan_id        = 20
  device_type    = "gateway"
  gateway_subnet = "192.168.50.1/24"
  dhcp_enabled   = true
  dhcp_start     = "192.168.50.10"
  dhcp_end       = "192.168.50.200"
}
o