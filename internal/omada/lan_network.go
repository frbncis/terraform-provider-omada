// Copyright (c) wncservices
// SPDX-License-Identifier: MPL-2.0

package omada

import (
	"context"
	"fmt"
)

// LAN network operations on the documented Open API "networks" (V3) workflow.
//
// Unlike the web-API/`lan-networks` V2 surface (see networks.go), the V3
// workflow operates on the `LanNetworkOpenApiV3VO` object and exposes the DHCP
// Server Device (`deviceType`, 0=External,1=Gateway,2=Switch,3=None) plus
// `dhcpSettings`/`dhcpServer`/`dhcpRelay`. Create and modify are a
// check → confirm pair: POST /networks/check then /networks/confirm (create),
// or POST /networks/{id}/check then PUT /networks/{id}/confirm (modify).

// IPv6SLAAC is the SLAAC sub-object of lanNetworkIpv6Config.
type IPv6SLAAC struct {
	PreType int `json:"preType"`
}

// IPv6RDNSS is the RDNSS sub-object of lanNetworkIpv6Config.
type IPv6RDNSS struct {
	PreType int `json:"preType"`
}

// LanNetworkIPv6Config is the lanNetworkIpv6Config sub-object.
type LanNetworkIPv6Config struct {
	Proto  int       `json:"proto"`
	Enable int       `json:"enable"`
	SLAAC  IPv6SLAAC `json:"slaac"`
	RDNSS  IPv6RDNSS `json:"rdnss"`
}

// LanNetwork is the LanNetworkOpenApiV3VO object.
type LanNetwork struct {
	ID                string                `json:"id"`
	Site              string                `json:"site"`
	Name              string                `json:"name"`
	Purpose           int                   `json:"purpose"` // 0=VLAN, 1=interface
	VLANType          int                   `json:"vlanType"`
	VLAN              int                   `json:"vlan"`
	Application       int                   `json:"application"` // 0=Gateway+Switch, 1=Switch
	GatewaySubnet     string                `json:"gatewaySubnet,omitempty"`
	DHCPSettings      *DHCPSettingsV3       `json:"dhcpSettings,omitempty"`
	DHCPL2RelayEnable bool                  `json:"dhcpL2RelayEnable"`
	IGMPSnoopEnable   bool                  `json:"igmpSnoopEnable"`
	MLDSnoopEnable    bool                  `json:"mldSnoopEnable"`
	DHCPv6Guard       *EnableFlag           `json:"dhcpv6Guard,omitempty"`
	DHCPGuard         *EnableFlag           `json:"dhcpGuard,omitempty"`
	LANNetworkIPv6Cfg *LanNetworkIPv6Config `json:"lanNetworkIpv6Config,omitempty"`
	DeviceType        int                   `json:"deviceType"` // 0=External,1=Gateway,2=Switch,3=None
	DeviceMac         string                `json:"deviceMac,omitempty"`
	StackID           string                `json:"stackId,omitempty"`
	Mode              int                   `json:"mode,omitempty"` // DHCP mode, valid when deviceType=2
	DHCPRelay         interface{}           `json:"dhcpRelay,omitempty"`
	DHCPRelayEnable   bool                  `json:"dhcpRelayEnable,omitempty"`
	QOSQueueEnable    bool                  `json:"qosQueueEnable"`
	SubnetOverride    bool                  `json:"subnetOverrideEnable"`
}

// DHCPSettingsV3 is the dhcpSettings sub-object (DhcpSettingConfig).
type DHCPSettingsV3 struct {
	Enable     bool            `json:"enable"`
	IPRange    []DHCPRangeOpen `json:"ipRangePool"`
	Options    []DHCPConfigOpt `json:"options"`
	DNSMode    string          `json:"dhcpns,omitempty"`
	LeaseTime  int             `json:"leasetime,omitempty"`
	DNSGateway string          `json:"gateway,omitempty"`
	NextServer string          `json:"dhcpNextServer,omitempty"`
}

// DHCPRangeOpen is a dhcpSettings.ipRangePool entry.
type DHCPRangeOpen struct {
	IPAddrStart string `json:"ipaddrStart"`
	IPAddrEnd   string `json:"ipaddrEnd"`
}

// DHCPConfigOpt is a custom DHCP option.
type DHCPConfigOpt struct {
	Code  int    `json:"code"`
	Type  int    `json:"type"`
	Value string `json:"value"`
}

// PortBinding is a deviceConfig.deviceList entry (PortBindingVO).
type PortBinding struct {
}

// CreateVlanParam is the CreateVlanParamOpenApiVO request body.
type CreateVlanParam struct {
	LanNetwork   LanNetwork        `json:"lanNetwork"`
	DeviceConfig SelectPortBinding `json:"deviceConfig"`
}

// SelectPortBinding is the deviceConfig object (SelectPortBindingBriefVO).
// Fields must always be present (no omitempty): the controller's check rejects
// a sparse deviceConfig with -1 General error.
type SelectPortBinding struct {
	PortIsolationEnable bool          `json:"portIsolationEnable"`
	FlowControlEnable   bool          `json:"flowControlEnable"`
	TagIDs              []string      `json:"tagIds"`
	DeviceList          []PortBinding `json:"deviceList"`
	InternetPorts       []string      `json:"internetPorts"`
}

// ModifyVlanParam is the ModifyVlanParamOpenApiVO request body.
type ModifyVlanParam struct {
	SkipEnable   bool              `json:"skipEnable,omitempty"`
	LanNetwork   LanNetwork        `json:"lanNetwork"`
	DeviceConfig SelectPortBinding `json:"deviceConfig"`
}

// lanNetworkCheckPath builds the create precheck path.
func lanNetworkCheckPath() string {
	return "/networks/check"
}

// lanNetworkConfirmPath builds the create confirm path.
func lanNetworkConfirmPath() string {
	return "/networks/confirm"
}

// lanNetworkModifyCheckPath builds the modify precheck path.
func lanNetworkModifyCheckPath(id string) string {
	return fmt.Sprintf("/networks/%s/check", id)
}

// lanNetworkModifyConfirmPath builds the modify confirm path.
func lanNetworkModifyConfirmPath(id string) string {
	return fmt.Sprintf("/networks/%s/confirm", id)
}

// CreateLanNetwork runs the V3 precheck → confirm pair and returns the created
// network ID(s).
func (c *Client) CreateLanNetwork(ctx context.Context, siteID string, params CreateVlanParam) (string, error) {
	if err := c.DoOpenAPI(ctx, "POST", c.OpenAPIPath(siteID, lanNetworkCheckPath()), params, nil); err != nil {
		return "", fmt.Errorf("network precheck (check): %w", err)
	}
	var out struct {
		NetworkIDList []string `json:"networkIdList"`
	}
	if err := c.DoOpenAPI(ctx, "POST", c.OpenAPIPath(siteID, lanNetworkConfirmPath()), params, &out); err != nil {
		return "", fmt.Errorf("network confirm: %w", err)
	}
	if len(out.NetworkIDList) == 0 {
		return "", fmt.Errorf("network confirm returned no network id")
	}
	return out.NetworkIDList[0], nil
}

// UpdateLanNetwork runs the V3 modify precheck → confirm pair.
func (c *Client) UpdateLanNetwork(ctx context.Context, siteID, id string, params ModifyVlanParam) error {
	if err := c.DoOpenAPI(ctx, "POST", c.OpenAPIPath(siteID, lanNetworkModifyCheckPath(id)), params, nil); err != nil {
		return fmt.Errorf("network modify precheck (check): %w", err)
	}
	if err := c.DoOpenAPI(ctx, "PUT", c.OpenAPIPath(siteID, lanNetworkModifyConfirmPath(id)), params, nil); err != nil {
		return fmt.Errorf("network modify confirm: %w", err)
	}
	return nil
}

// ListLanNetworksV3 returns every LAN network for the site via the V3 surface.
// The V3 list endpoint uses page/pageSize (not the currentPage/currentSize the
// shared listAll pager sends), so it walks pages itself.
func (c *Client) ListLanNetworksV3(ctx context.Context, siteID string) ([]LanNetwork, error) {
	path := c.OpenAPIPathVersion(3, siteID, "/lan-networks")
	var all []LanNetwork
	for page := 1; ; page++ {
		var pr struct {
			TotalRows int          `json:"totalRows"`
			Data      []LanNetwork `json:"data"`
		}
		if err := c.DoOpenAPI(ctx, "GET", fmt.Sprintf("%s?page=%d&pageSize=100", path, page), nil, &pr); err != nil {
			return nil, fmt.Errorf("listing lan networks (v3): %w", err)
		}
		all = append(all, pr.Data...)
		if len(all) >= pr.TotalRows || len(pr.Data) == 0 {
			break
		}
	}
	return all, nil
}

// GetLanNetworkV3 returns a single LAN network by id via the V3 surface.
func (c *Client) GetLanNetworkV3(ctx context.Context, siteID, id string) (*LanNetwork, error) {
	nets, err := c.ListLanNetworksV3(ctx, siteID)
	if err != nil {
		return nil, err
	}
	for i := range nets {
		if nets[i].ID == id {
			return &nets[i], nil
		}
	}
	return nil, fmt.Errorf("lan network %q not found on site %q", id, siteID)
}

// DeleteLanNetwork removes a network via the web API (the V3 surface has no
// DELETE; the shared web-API item DELETE is used).
func (c *Client) DeleteLanNetwork(ctx context.Context, siteID, id string) error {
	if err := c.Do(ctx, "DELETE", networksPath(siteID)+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("deleting lan network %q: %w", id, err)
	}
	return nil
}
