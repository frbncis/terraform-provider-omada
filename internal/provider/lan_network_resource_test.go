// Copyright (c) wncservices
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccLanNetworkResource drives create → update of omada_lan_network via the
// V3 check→confirm workflow, covering device_type (DHCP Server Device).
func TestAccLanNetworkResource(t *testing.T) {
	srv := newMockController(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // create: L2 vlan with device_type "none"
				Config: testProviderConfigOpenAPI(srv.URL) + `
resource "omada_lan_network" "vlan" {
  name        = "IoT-vlan"
  vlan_id     = 99
  device_type = "none"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("omada_lan_network.vlan", "id"),
					resource.TestCheckResourceAttr("omada_lan_network.vlan", "name", "IoT-vlan"),
					resource.TestCheckResourceAttr("omada_lan_network.vlan", "vlan_id", "99"),
					resource.TestCheckResourceAttr("omada_lan_network.vlan", "device_type", "none"),
				),
			},
			{ // update: change vlan_id + device_type → external_device (in-place)
				Config: testProviderConfigOpenAPI(srv.URL) + `
resource "omada_lan_network" "vlan" {
  name        = "IoT-vlan"
  vlan_id     = 100
  device_type = "external_device"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omada_lan_network.vlan", "vlan_id", "100"),
					resource.TestCheckResourceAttr("omada_lan_network.vlan", "device_type", "external_device"),
				),
			},
		},
	})
}

// TestAccLanNetworkDeviceTypeInvalid verifies the enum validator rejects an
// unknown device_type value at plan time.
func TestAccLanNetworkDeviceTypeInvalid(t *testing.T) {
	srv := newMockController(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfigOpenAPI(srv.URL) + `
resource "omada_lan_network" "vlan" {
  name        = "IoT-vlan"
  vlan_id     = 99
  device_type = "bogus"
}
`,
				ExpectError: regexp.MustCompile("value must be one of"),
			},
		},
	})
}
