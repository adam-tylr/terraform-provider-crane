// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &DigestDataSource{}

// DigestDataSource resolves the manifest digest for a reference.
type DigestDataSource struct {
	options []crane.Option
}

// DigestDataSourceModel describes the data source model.
type DigestDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Reference types.String `tfsdk:"reference"`
	Digest    types.String `tfsdk:"digest"`
}

func NewDigestDataSource() datasource.DataSource {
	return &DigestDataSource{}
}

func (d *DigestDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_digest"
}

func (d *DigestDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resolve the manifest digest for a container image reference.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Equivalent to the requested reference.",
				Computed:            true,
			},
			"reference": schema.StringAttribute{
				MarkdownDescription: "A tag or digest identifying the image to inspect (for example `registry/repository:tag`).",
				Required:            true,
			},
			"digest": schema.StringAttribute{
				MarkdownDescription: "Content digest of the referenced image, such as `sha256:...`.",
				Computed:            true,
			},
		},
	}
}

func (d *DigestDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	options, ok := req.ProviderData.([]crane.Option)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *[]crane.Option, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.options = options
}

func (d *DigestDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DigestDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ref := data.Reference.ValueString()
	options := append([]crane.Option{}, d.options...)
	options = append(options, crane.WithContext(ctx))

	digest, err := crane.Digest(ref, options...)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("reading digest for %q", ref), err.Error())
		return
	}

	data.ID = types.StringValue(ref)
	data.Digest = types.StringValue(digest)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
