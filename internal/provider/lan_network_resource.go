// Copyright (c) wncservices
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wncservices/terraform-provider-omada/internal/omada"
)

type lanNetworkResource struct{ data *providerData }

// NewLanNetworkResource returns the omada_lan_network resource.
func NewLanNetworkResource() resource.Resource { return &lanNetworkResource{} }

type lanNetworkResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Site          types.String `tfsdk:"site"`
	SiteID        types.String `tfsdk:"site_id"`
	Name          types.String `tfsdk:"name"`
	VLANID        types.Int64  `tfsdk:"vlan_id"`
	DeviceType    types.String `tfsdk:"device_type"`
	GatewaySubnet types.String `tfsdk:"gateway_subnet"`
	DHCPEnable    types.Bool   `tfsdk:"dhcp_enabled"`
	DHCPStart     types.String `tfsdk:"dhcp_start"`
	DHCPEnd       types.String `tfsdk:"dhcp_end"`
}

// deviceType→controller mapping (0=External,1=Gateway,3=None).
func lanNetworkDeviceTypeToController(s string) (int64, bool) {
	switch s {
	case "external_device":
		return 0, true
	case "gateway":
		return 1, true
	case "none":
		return 3, true
	default:
		return 0, false
	}
}

func lanNetworkControllerToDeviceType(v int) string {
	switch v {
	case 1:
		return "gateway"
	case 3:
		return "none"
	default:
		return "external_device"
	}
}

func (r *lanNetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lan_network"
}

func (r *lanNetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage a wired LAN network based on 802.1Q on the Omada Controller. Optionally select a device to serve as the DHCP Server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Controller-assigned network ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"site": schema.StringAttribute{
				MarkdownDescription: "Site name. Defaults to the controller's primary site.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"site_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":    schema.StringAttribute{Required: true, MarkdownDescription: "Network name."},
			"vlan_id": schema.Int64Attribute{Required: true, MarkdownDescription: "VLAN ID (1-4094)."},
			"device_type": schema.StringAttribute{
				MarkdownDescription: "DHCP Server Device: `external_device`, `gateway`, or `none`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("external_device"),
				Validators: []validator.String{
					stringvalidator.OneOf("external_device", "gateway", "none"),
				},
			},
			"gateway_subnet": schema.StringAttribute{
				MarkdownDescription: "Gateway subnet in CIDR notation (e.g. `192.168.50.1/24`). Required when `device_type` is `gateway`.",
				Optional:            true,
				Computed:            true,
			},
			"dhcp_enabled": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable the DHCP server on this network. Only effective when `device_type` is `gateway`."},
			"dhcp_start":   schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "First address of the DHCP pool."},
			"dhcp_end":     schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Last address of the DHCP pool."},
		},
	}
}

func (r *lanNetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.data = req.ProviderData.(*providerData)
}

func (r *lanNetworkResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m lanNetworkResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	isGateway := m.DeviceType.ValueString() == "gateway"
	hasSubnet := !m.GatewaySubnet.IsNull() && m.GatewaySubnet.ValueString() != ""
	if isGateway && !hasSubnet {
		resp.Diagnostics.AddAttributeError(
			path.Root("gateway_subnet"),
			"Missing gateway subnet",
			"`device_type` is set to `gateway`, which requires a `gateway_subnet` (e.g. \"192.168.50.1/24\").",
		)
	}
	if !isGateway && hasSubnet {
		resp.Diagnostics.AddAttributeError(
			path.Root("gateway_subnet"),
			"Unexpected gateway subnet",
			"`gateway_subnet` is only valid when `device_type` is `gateway`.",
		)
	}
}

func (r *lanNetworkResource) siteName(m lanNetworkResourceModel) string {
	if !m.Site.IsNull() && m.Site.ValueString() != "" {
		return m.Site.ValueString()
	}
	return r.data.defaultSite
}

func (r *lanNetworkResource) fieldsFrom(ctx context.Context, m lanNetworkResourceModel) (omada.CreateVlanParam, diag.Diagnostics) {
	var diags diag.Diagnostics
	dt, _ := lanNetworkDeviceTypeToController(m.DeviceType.ValueString())
	purpose := 0 // VLAN (L2)
	if dt == 1 {
		purpose = 1 // interface — gateway serves the network (and its DHCP)
	}
	lan := omada.LanNetwork{
		Site:              m.SiteID.ValueString(),
		Name:              m.Name.ValueString(),
		Purpose:           purpose,
		VLANType:          0,
		VLAN:              int(m.VLANID.ValueInt64()),
		GatewaySubnet:     m.GatewaySubnet.ValueString(),
		DeviceType:        int(dt),
		IGMPSnoopEnable:   false,
		MLDSnoopEnable:    false,
		DHCPL2RelayEnable: false,
		QOSQueueEnable:    false,
		SubnetOverride:    false,
		DHCPv6Guard:       &omada.EnableFlag{Enable: false},
		DHCPGuard:         &omada.EnableFlag{Enable: false},
		LANNetworkIPv6Cfg: &omada.LanNetworkIPv6Config{Proto: 0, Enable: 0, SLAAC: omada.IPv6SLAAC{PreType: 0}, RDNSS: omada.IPv6RDNSS{PreType: 0}},
	}
	// DHCP settings are only honored by the controller when the gateway is the
	// DHCP server device. Even when disabled, the controller requires `dhcpns`
	// (dhcpns) and `leasetime` present inside dhcpSettings or it rejects the
	// request; `ipRange` must remain empty when disabled.
	dhcp := &omada.DHCPSettingsV3{Enable: false, Options: []omada.DHCPConfigOpt{}}
	if dt == 1 {
		if !m.DHCPEnable.IsNull() && !m.DHCPEnable.IsUnknown() {
			dhcp.Enable = m.DHCPEnable.ValueBool()
		}
		if dhcp.Enable {
			if !m.DHCPStart.IsNull() && !m.DHCPStart.IsUnknown() && m.DHCPStart.ValueString() != "" {
				dhcp.IPRange = append(dhcp.IPRange, omada.DHCPRangeOpen{IPAddrStart: m.DHCPStart.ValueString(), IPAddrEnd: m.DHCPEnd.ValueString()})
			}
			dhcp.DNSMode = "auto"
			dhcp.LeaseTime = 120
		}
	}
	lan.DHCPSettings = dhcp
	return omada.CreateVlanParam{
		LanNetwork:   lan,
		DeviceConfig: omada.SelectPortBinding{PortIsolationEnable: true, FlowControlEnable: false, TagIDs: []string{}, DeviceList: []omada.PortBinding{}, InternetPorts: []string{}},
	}, diags
}

func (r *lanNetworkResource) apply(ctx context.Context, n *omada.LanNetwork, m *lanNetworkResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(n.ID)
	m.SiteID = types.StringValue(n.Site)
	m.Name = types.StringValue(n.Name)
	m.VLANID = types.Int64Value(int64(n.VLAN))
	m.DeviceType = types.StringValue(lanNetworkControllerToDeviceType(n.DeviceType))
	m.GatewaySubnet = types.StringValue(n.GatewaySubnet)
	if n.DHCPSettings != nil {
		m.DHCPEnable = types.BoolValue(n.DHCPSettings.Enable)
		if len(n.DHCPSettings.IPRange) > 0 {
			m.DHCPStart = types.StringValue(n.DHCPSettings.IPRange[0].IPAddrStart)
			m.DHCPEnd = types.StringValue(n.DHCPSettings.IPRange[0].IPAddrEnd)
		}
	}
	// Ensure computed fields are known after apply even when the controller
	// returns empty values (dhcp_start/dhcp_end/dhcp_enabled/gateway_subnet).
	// `site` is a config attribute (RequiresReplace) and must NOT be rewritten.
	if m.DHCPStart.IsNull() || m.DHCPStart.IsUnknown() {
		m.DHCPStart = types.StringValue("")
	}
	if m.DHCPEnd.IsNull() || m.DHCPEnd.IsUnknown() {
		m.DHCPEnd = types.StringValue("")
	}
	if m.DHCPEnable.IsNull() || m.DHCPEnable.IsUnknown() {
		m.DHCPEnable = types.BoolValue(false)
	}
	if m.GatewaySubnet.IsNull() || m.GatewaySubnet.IsUnknown() {
		m.GatewaySubnet = types.StringValue("")
	}
	return diags
}

func (r *lanNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lanNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	siteID, err := r.data.client.ResolveSiteID(ctx, r.siteName(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to resolve site", err.Error())
		return
	}
	plan.SiteID = types.StringValue(siteID)
	params, diags := r.fieldsFrom(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	params.LanNetwork.Site = siteID
	id, err := r.data.client.CreateLanNetwork(ctx, siteID, params)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create lan network", err.Error())
		return
	}
	created, err := r.data.client.GetLanNetworkV3(ctx, siteID, id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read lan network", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, created, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *lanNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lanNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	net, err := r.data.client.GetLanNetworkV3(ctx, state.SiteID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read lan network", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, net, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lanNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan lanNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params, diags := r.fieldsFrom(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	params.LanNetwork.Site = plan.SiteID.ValueString()
	params.LanNetwork.ID = plan.ID.ValueString()
	if err := r.data.client.UpdateLanNetwork(ctx, plan.SiteID.ValueString(), plan.ID.ValueString(), omada.ModifyVlanParam{
		SkipEnable:   true,
		LanNetwork:   params.LanNetwork,
		DeviceConfig: params.DeviceConfig,
	}); err != nil {
		resp.Diagnostics.AddError("Unable to update lan network", err.Error())
		return
	}
	net, err := r.data.client.GetLanNetworkV3(ctx, plan.SiteID.ValueString(), plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read lan network", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, net, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *lanNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lanNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.data.client.DeleteLanNetwork(ctx, state.SiteID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete lan network", err.Error())
		return
	}
}

func (r *lanNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var _ resource.Resource = &lanNetworkResource{}
