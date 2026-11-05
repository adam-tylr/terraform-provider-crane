// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure CraneProvider satisfies various provider interfaces.
var _ provider.Provider = &CraneProvider{}

// CraneProvider defines the provider implementation.
type CraneProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type registryAuthModel struct {
	Address  types.String `tfsdk:"address"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Auth     types.String `tfsdk:"auth"`
}

type craneProviderModel struct {
	AllowNondistributableArtifacts types.Bool          `tfsdk:"allow_nondistributable_artifacts"`
	RegistryAuth                   []registryAuthModel `tfsdk:"registry_auth"`
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
		Blocks: map[string]schema.Block{
			"registry_auth": schema.ListNestedBlock{
				MarkdownDescription: "List of registry authentications. When specified, credentials are used directly in memory by Crane operations without requiring host Docker configuration files.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Registry address/hostname (e.g. `123456789.dkr.ecr.eu-west-1.amazonaws.com` or `index.docker.io`).",
						},
						"username": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Username for registry authentication.",
						},
						"password": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Password for registry authentication.",
						},
						"auth": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Base64 encoded credentials string (`username:password`). Alternative to username and password.",
						},
					},
				},
			},
		},
	}
}

type staticKeychain map[string]authn.AuthConfig

func normalizeRegistry(addr string) string {
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimSuffix(addr, "/")
	if addr == "docker.io" || addr == "registry-1.docker.io" {
		return name.DefaultRegistry
	}
	return addr
}

func (k staticKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	targetReg := normalizeRegistry(target.RegistryStr())
	if cfg, ok := k[targetReg]; ok {
		return authn.FromConfig(cfg), nil
	}
	for reg, cfg := range k {
		if normalizeRegistry(reg) == targetReg {
			return authn.FromConfig(cfg), nil
		}
	}
	return authn.Anonymous, nil
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

	if len(config.RegistryAuth) > 0 {
		staticKeys := make(staticKeychain)
		for _, auth := range config.RegistryAuth {
			if auth.Address.IsNull() || auth.Address.IsUnknown() {
				continue
			}
			reg := auth.Address.ValueString()
			cfg := authn.AuthConfig{}
			if !auth.Username.IsNull() && !auth.Username.IsUnknown() {
				cfg.Username = auth.Username.ValueString()
			}
			if !auth.Password.IsNull() && !auth.Password.IsUnknown() {
				cfg.Password = auth.Password.ValueString()
			}
			if !auth.Auth.IsNull() && !auth.Auth.IsUnknown() {
				cfg.Auth = auth.Auth.ValueString()
			}
			staticKeys[normalizeRegistry(reg)] = cfg
		}
		keychain := authn.NewMultiKeychain(staticKeys, authn.DefaultKeychain)
		craneOpts = append(craneOpts, crane.WithAuthFromKeychain(keychain))
	}

	craneOpts = append(craneOpts, crane.WithUserAgent(fmt.Sprintf("terraform-provider-crane/%s", p.version)))

	resp.ResourceData = craneOpts
	resp.DataSourceData = craneOpts
}

func (p *CraneProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewImageResource,
	}
}

func (p *CraneProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTagsDataSource,
		NewDigestDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CraneProvider{
			version: version,
		}
	}
}
