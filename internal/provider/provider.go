// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure CraneProvider satisfies various provider interfaces.
var _ provider.Provider = &CraneProvider{}
var _ provider.ProviderWithValidateConfig = &CraneProvider{}

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
				MarkdownDescription: "List of registry authentications. When specified, credentials are used directly in memory by Crane operations without requiring host Docker configuration files. Registries not matching any `registry_auth` block fall back to the default host keychain (`~/.docker/config.json`, credential helpers, etc.).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Registry address/hostname (e.g. `123456789.dkr.ecr.eu-west-1.amazonaws.com` or `index.docker.io`). Scheme prefixes (`https://`, `http://`) and trailing slashes are stripped automatically.",
						},
						"username": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Username for registry authentication. Must be set together with `password`. Conflicts with `auth`.",
						},
						"password": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Password for registry authentication. Must be set together with `username`. Conflicts with `auth`.",
						},
						"auth": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Base64 encoded credentials string (`username:password`). Conflicts with `username` and `password`.",
						},
					},
				},
			},
		},
	}
}

type staticKeychain map[string]authn.AuthConfig

func normalizeRegistry(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimRight(addr, "/")
	addr = strings.TrimSuffix(addr, "/v1")
	addr = strings.TrimSuffix(addr, "/v2")
	addr = strings.TrimRight(addr, "/")
	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[:idx]
	}
	if addr == "docker.io" || addr == "registry-1.docker.io" {
		return name.DefaultRegistry
	}
	return addr
}

func decodeBase64Auth(field string) (string, string, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return "", "", fmt.Errorf("auth cannot be empty")
	}

	var (
		decoded []byte
		err     error
	)
	if strings.HasSuffix(field, "=") {
		decoded, err = base64.StdEncoding.DecodeString(field)
	} else {
		decoded, err = base64.RawStdEncoding.DecodeString(field)
	}
	if err != nil {
		return "", "", fmt.Errorf("must be valid base64 encoded 'username:password': %w", err)
	}

	username, password, ok := strings.Cut(string(decoded), ":")
	if !ok || username == "" || password == "" {
		return "", "", fmt.Errorf("decoded auth string must be in 'username:password' format with non-empty username and password")
	}

	return username, password, nil
}

func buildStaticKeychain(registryAuths []registryAuthModel) (staticKeychain, diag.Diagnostics) {
	var diags diag.Diagnostics
	staticKeys := make(staticKeychain)

	for i, auth := range registryAuths {
		blockPath := path.Root("registry_auth").AtListIndex(i)

		hasUsername := !auth.Username.IsNull()
		hasPassword := !auth.Password.IsNull()
		hasAuth := !auth.Auth.IsNull()

		if hasAuth && (hasUsername || hasPassword) {
			diags.AddAttributeError(
				blockPath,
				"Conflicting Registry Auth Credentials",
				"Cannot specify both 'auth' and 'username'/'password' in the same registry_auth block.",
			)
			continue
		}

		if !hasAuth && !hasUsername && !hasPassword {
			diags.AddAttributeError(
				blockPath,
				"Missing Registry Auth Credentials",
				"Either 'auth' or both 'username' and 'password' must be specified in registry_auth.",
			)
			continue
		}

		if !hasAuth && (hasUsername != hasPassword) {
			diags.AddAttributeError(
				blockPath,
				"Incomplete Registry Auth Credentials",
				"Both 'username' and 'password' must be specified together when not using 'auth'.",
			)
			continue
		}

		if auth.Address.IsNull() {
			diags.AddAttributeError(
				blockPath.AtName("address"),
				"Missing Registry Auth Address",
				"Registry address must be specified.",
			)
			continue
		}

		var normalizedAddr string
		if !auth.Address.IsUnknown() {
			normalizedAddr = normalizeRegistry(auth.Address.ValueString())
			if normalizedAddr == "" {
				diags.AddAttributeError(
					blockPath.AtName("address"),
					"Invalid Registry Auth Address",
					"Registry address cannot be empty.",
				)
				continue
			}
		}

		cfg := authn.AuthConfig{}
		if hasAuth {
			if auth.Auth.IsUnknown() {
				continue
			}
			username, password, err := decodeBase64Auth(auth.Auth.ValueString())
			if err != nil {
				diags.AddAttributeError(
					blockPath.AtName("auth"),
					"Invalid Registry Auth Value",
					fmt.Sprintf("Invalid base64 'auth' value: %s", err),
				)
				continue
			}
			cfg.Username = username
			cfg.Password = password
			cfg.Auth = strings.TrimSpace(auth.Auth.ValueString())
		} else {
			if !auth.Username.IsUnknown() && auth.Username.ValueString() == "" {
				diags.AddAttributeError(
					blockPath.AtName("username"),
					"Invalid Registry Auth Username",
					"Registry username cannot be empty.",
				)
				continue
			}
			if !auth.Password.IsUnknown() && auth.Password.ValueString() == "" {
				diags.AddAttributeError(
					blockPath.AtName("password"),
					"Invalid Registry Auth Password",
					"Registry password cannot be empty.",
				)
				continue
			}
			if auth.Username.IsUnknown() || auth.Password.IsUnknown() {
				continue
			}
			cfg.Username = auth.Username.ValueString()
			cfg.Password = auth.Password.ValueString()
		}

		if auth.Address.IsUnknown() {
			continue
		}

		staticKeys[normalizedAddr] = cfg
	}

	return staticKeys, diags
}

func (k staticKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	targetReg := normalizeRegistry(target.RegistryStr())
	if cfg, ok := k[targetReg]; ok && cfg != (authn.AuthConfig{}) {
		return authn.FromConfig(cfg), nil
	}
	for reg, cfg := range k {
		if normalizeRegistry(reg) == targetReg && cfg != (authn.AuthConfig{}) {
			return authn.FromConfig(cfg), nil
		}
	}
	return authn.Anonymous, nil
}

func (p *CraneProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	var config craneProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, authDiags := buildStaticKeychain(config.RegistryAuth)
	resp.Diagnostics.Append(authDiags...)
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
		staticKeys, authDiags := buildStaticKeychain(config.RegistryAuth)
		resp.Diagnostics.Append(authDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(staticKeys) > 0 {
			keychain := authn.NewMultiKeychain(staticKeys, authn.DefaultKeychain)
			craneOpts = append(craneOpts, crane.WithAuthFromKeychain(keychain))
		}
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
