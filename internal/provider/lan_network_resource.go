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
	ID         types.String `tfsdk:"id"`
	Site       types.String `tfsdk:"site"`
	SiteID     types.String `tfsdk:"site_id"`
	Name       types.String `tfsdk:"name"`
	VLANID     types.Int64  `tfsdk:"vlan_id"`
	DeviceType types.String `tfsdk:"device_type"`
	DHCPEnable types.Bool   `tfsdk:"dhcp_enabled"`
	DHCPStart  types.String `tfsdk:"dhcp_start"`
	DHCPEnd    types.String `tfsdk:"dhcp_end"`
}

// deviceType→controller mapping (0=External,1=Gateway,2=Switch,3=None).
func lanNetworkDeviceTypeToController(s string) (int64, bool) {
	switch s {
	case "external_device":
		return 0, true
	case "gateway":
		return 1, true
	case "switch":
		return 2, true
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
	case 2:
		return "switch"
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
		MarkdownDescription: "Manages a LAN network (VLAN) on the Omada controller via the V3 Open API workflow (check → confirm), including the DHCP Server Device (`device_type`).",
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
				MarkdownDescription: "DHCP Server Device: `external_device`, `gateway`, `switch`, or `none`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("external_device"),
				Validators: []validator.String{
					stringvalidator.OneOf("external_device", "gateway", "switch", "none"),
				},
			},
			"dhcp_enabled": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable the DHCP server on this network."},
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

func (r *lanNetworkResource) siteName(m lanNetworkResourceModel) string {
	if !m.Site.IsNull() && m.Site.ValueString() != "" {
		return m.Site.ValueString()
	}
	return r.data.defaultSite
}

func (r *lanNetworkResource) fieldsFrom(ctx context.Context, m lanNetworkResourceModel) (omada.CreateVlanParam, diag.Diagnostics) {
	var diags diag.Diagnostics
	dt, _ := lanNetworkDeviceTypeToController(m.DeviceType.ValueString())
	lan := omada.LanNetwork{
		Site:              m.SiteID.ValueString(),
		Name:              m.Name.ValueString(),
		Purpose:           0, // VLAN (L2)
		VLANType:          0,
		VLAN:              int(m.VLANID.ValueInt64()),
		DeviceType:        int(dt),
		IGMPSnoopEnable:   false,
		MLDSnoopEnable:    false,
		DHCPL2RelayEnable: false,
		QOSQueueEnable:    false,
		SubnetOverride:    false,
		DHCPv6Guard:       &omada.EnableFlag{Enable: false},
		DHCPGuard:         &omada.EnableFlag{Enable: false},
		LANNetworkIPv6Cfg: &omada.LanNetworkIPv6Config{Proto: 0, Enable: 0, SLAAC: omada.IPv6SLAAC{}, RDNSS: omada.IPv6RDNSS{}},
	}
	dhcp := &omada.DHCPSettingsV3{Enable: false}
	if !m.DHCPEnable.IsNull() && !m.DHCPEnable.IsUnknown() {
		dhcp.Enable = m.DHCPEnable.ValueBool()
	}
	if !m.DHCPStart.IsNull() && !m.DHCPStart.IsUnknown() && m.DHCPStart.ValueString() != "" {
		dhcp.IPRange = append(dhcp.IPRange, omada.DHCPRangeOpen{IPAddrStart: m.DHCPStart.ValueString(), IPAddrEnd: m.DHCPEnd.ValueString()})
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
	if n.DHCPSettings != nil {
		m.DHCPEnable = types.BoolValue(n.DHCPSettings.Enable)
		if len(n.DHCPSettings.IPRange) > 0 {
			m.DHCPStart = types.StringValue(n.DHCPSettings.IPRange[0].IPAddrStart)
			m.DHCPEnd = types.StringValue(n.DHCPSettings.IPRange[0].IPAddrEnd)
		}
	}
	// Ensure computed fields are known after apply even when the controller
	// returns empty values (dhcp_start/dhcp_end/dhcp_enabled). `site` is a
	// config attribute (RequiresReplace) and must NOT be rewritten here.
	if m.DHCPStart.IsNull() || m.DHCPStart.IsUnknown() {
		m.DHCPStart = types.StringValue("")
	}
	if m.DHCPEnd.IsNull() || m.DHCPEnd.IsUnknown() {
		m.DHCPEnd = types.StringValue("")
	}
	if m.DHCPEnable.IsNull() || m.DHCPEnable.IsUnknown() {
		m.DHCPEnable = types.BoolValue(false)
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
