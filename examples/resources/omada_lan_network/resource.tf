# Manages an L2 LAN network (VLAN) on the Omada controller via the V3 Open API
# check → confirm workflow.
#
# `device_type` (DHCP Server Device) selects which device serves DHCP on the
# network. Supported values: external_device, gateway, switch, none.
#
#   none            — no DHCP server on this L2-only VLAN (addresses served elsewhere).
#   external_device — an upstream/external DHCP server serves this VLAN.
#   gateway         — the controller's gateway is the DHCP server; requires a
#                     `gateway_subnet` and is the only `device_type` that
#                     actually materialises a DHCP pool (dhcp_enabled/start/end).
#   switch          — requires a specific switch device. Not yet modeled.

# none — pure L2 VLAN, no DHCP here.
resource "omada_lan_network" "vlan_none" {
  name        = "IOT"
  vlan_id     = 30
  device_type = "none"
}

# external_device — L2 VLAN whose address pool is served by an upstream DHCP
# server (e.g. a router or dedicated DHCP appliance).
resource "omada_lan_network" "vlan_external" {
  name        = "Guest"
  vlan_id     = 40
  device_type = "external_device"
}

# gateway — the controller's gateway is the DHCP server for this network. This
# is the only device_type where dhcp_enabled/dhcp_start/dhcp_end take effect;
# a gateway_subnet (CIDR) is required.
resource "omada_lan_network" "vlan_gateway" {
  name           = "Management"
  vlan_id        = 20
  device_type    = "gateway"
  gateway_subnet = "192.168.50.1/24"
  dhcp_enabled   = true
  dhcp_start     = "192.168.50.10"
  dhcp_end       = "192.168.50.200"
}
