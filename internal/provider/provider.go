// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure CraneProvider satisfies various provider interfaces.
var _ provider.Provider = &CraneProvider{}
var _ provider.ProviderWithFunctions = &CraneProvider{}

// var _ provider.ProviderWithEphemeralResources = &CraneProvider{}

// CraneProvider defines the provider implementation.
type CraneProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string

	options []crane.Option
}

type craneProviderModel struct {
	AllowNondistributableArtifacts types.Bool `tfsdk:"allow_nondistributable_artifacts"`
}

func (p *CraneProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "crane"
	resp.Version = p.version
}

func (p *CraneProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"allow_nondistributable_artifacts": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow pushing non-distributable (foreign) layers",
			},
		},
	}
}

func (p *CraneProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config craneProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	craneOpts := []crane.Option{}
	if !config.AllowNondistributableArtifacts.IsNull() && config.AllowNondistributableArtifacts.ValueBool() {
		craneOpts = append(craneOpts, crane.WithNondistributable())
	}
	craneOpts = append(craneOpts, crane.WithUserAgent(fmt.Sprintf("terraform-provider-crane/%s", p.version)))

	p.options = craneOpts
	resp.ResourceData = craneOpts
	resp.DataSourceData = craneOpts
}

func (p *CraneProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewImageResource,
	}
}

// func (p *CraneProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
// 	return []func() ephemeral.EphemeralResource{
// 		NewExampleEphemeralResource,
// 	}
// }

func (p *CraneProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTagsDataSource,
	}
}

func (p *CraneProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		func() function.Function {
			return NewDigestFunction(p)
		},
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CraneProvider{
			version: version,
		}
	}
}
