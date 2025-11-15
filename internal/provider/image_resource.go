package provider

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ImageResource{}
var _ resource.ResourceWithConfigure = &ImageResource{}
var _ resource.ResourceWithImportState = &ImageResource{}

func NewImageResource() resource.Resource {
	return &ImageResource{}
}

// ImageResource defines the resource implementation.
type ImageResource struct {
	options []crane.Option
}

// ImageResourceModel describes the resource data model.
type ImageResourceModel struct {
	Source       types.String `tfsdk:"source"`
	Destination  types.String `tfsdk:"destination"`
	SourceDigest types.String `tfsdk:"source_digest"`
	Platform     types.String `tfsdk:"platform"`
	Id           types.String `tfsdk:"id"`
	Reference    types.String `tfsdk:"reference"`
	Digest       types.String `tfsdk:"digest"`
}

func (r *ImageResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image"
}

func (r *ImageResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: `Push or copy an image to a remote repository. 
		
This resource is designed to support both Terraform and externally managed roll out and roll back of images which means:

- Resource deletion will not delete images or tags from the destination repository. Use lifecycle policies to manage image retention.
- Resource creation will not fail if the image already exists in the destination repository with the same digest.`,

		Attributes: map[string]schema.Attribute{
			"source": schema.StringAttribute{
				MarkdownDescription: "A remote image reference or path to a local docker-style tarball.",
				Required:            true,
			},
			"destination": schema.StringAttribute{
				MarkdownDescription: "The destination to push the image to (`registry/repo` or `registry/repo:tag`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_digest": schema.StringAttribute{
				MarkdownDescription: "Used to trigger updates for mutable tags. Set using `filemd5` for a local file or the `crane_digest` data source for a remote image.",
				Optional:            true,
			},
			"platform": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "If source is a multi-architecture image, limit copy to a specific platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64). (default all)",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Equivalent to `reference`.",
			},
			"reference": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The destination image reference including the tag or digest.",
			},
			"digest": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The digest of the destination image.",
			},
		},
	}
}

func (r *ImageResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.options = options
}

func (r *ImageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ImageResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	craneOpts, err := setPlatform(r.options, data.Platform)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing platform",
			fmt.Sprintf("Unable to parse platform '%s': %s", data.Platform.ValueString(), err),
		)
		return
	}
	o := crane.GetOptions(craneOpts...)

	source := data.Source.ValueString()
	destination := data.Destination.ValueString()
	doPush := true

	sourceDigest, err := readSourceDigest(ctx, source, craneOpts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading source image",
			fmt.Sprintf("Unable to read source image '%s': %s", source, err),
		)
		return
	}

	destRef, err := name.ParseReference(destination, o.Name...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing destination image reference",
			fmt.Sprintf("Unable to parse destination image reference '%s': %s", destination, err),
		)
		return
	}
	_, err = remote.Image(destRef, o.Remote...)
	// Check if the image already exists at the destination
	if err != nil {
		var remoteErr *transport.Error
		if ok := errors.As(err, &remoteErr); ok && remoteErr.StatusCode != 404 {
			resp.Diagnostics.AddError(
				"Error checking destination repository",
				fmt.Sprintf("Error checking destination repository '%s': %s", destination, err),
			)
			return
		}
	} else {
		dstDigest, err := crane.Digest(destination, craneOpts...)
		if err == nil && dstDigest != sourceDigest {
			resp.Diagnostics.AddError(
				"Destination image already exists but does not match source",
				fmt.Sprintf("Destination image '%s' already exists with a different digest.", destination),
			)
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("Destination image '%s' already exists, skipping push", destination))
		doPush = false
	}

	if doPush {
		err = performOperation(source, destination, craneOpts)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error pushing image to destination",
				fmt.Sprintf("Unable to push image '%s' to destination '%s': %s", source, destination, err),
			)
			return
		}
	}

	data.Id = types.StringValue(destination)
	data.Reference = types.StringValue(destination)
	data.Digest = types.StringValue(sourceDigest)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ImageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ImageResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	craneOpts, err := setPlatform(r.options, data.Platform)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing platform",
			fmt.Sprintf("Unable to parse platform '%s': %s", data.Platform.ValueString(), err),
		)
		return
	}
	o := crane.GetOptions(craneOpts...)

	ref, err := name.ParseReference(data.Id.ValueString(), o.Name...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing image reference from state",
			fmt.Sprintf("Unable to parse image reference '%s': %s", data.Id.ValueString(), err),
		)
		return
	}

	_, err = remote.Image(ref, o.Remote...)
	if err != nil {
		var remoteErr *transport.Error
		if ok := errors.As(err, &remoteErr); ok && remoteErr.StatusCode == 404 {
			resp.Diagnostics.AddWarning(
				"Image Not Found",
				fmt.Sprintf("Image '%s' not found in the registry. It may have been deleted or never pushed.", data.Id.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error fetching image from registry",
			fmt.Sprintf("Unable to fetch image '%s' from the registry: %s", data.Id.ValueString(), err),
		)
		return
	}

	actualDigest, err := crane.Digest(data.Id.ValueString(), craneOpts...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading image digest",
			fmt.Sprintf("Unable to read image digest for '%s': %s", data.Id.ValueString(), err),
		)
		return
	}

	data.Digest = types.StringValue(actualDigest)
	data.Destination = types.StringValue(data.Id.ValueString())
	data.Reference = types.StringValue(data.Id.ValueString())

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ImageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ImageResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	craneOpts, err := setPlatform(r.options, data.Platform)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing platform",
			fmt.Sprintf("Unable to parse platform '%s': %s", data.Platform.ValueString(), err),
		)
		return
	}

	source := data.Source.ValueString()
	destination := data.Destination.ValueString()

	sourceDigest, err := readSourceDigest(ctx, source, craneOpts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading source image",
			fmt.Sprintf("Unable to read source image '%s': %s", source, err),
		)
		return
	}

	err = performOperation(source, destination, craneOpts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error pushing image to destination",
			fmt.Sprintf("Unable to push image '%s' to destination '%s': %s", source, destination, err),
		)
		return
	}

	data.Id = types.StringValue(destination)
	data.Reference = types.StringValue(destination)
	data.Digest = types.StringValue(sourceDigest)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ImageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *ImageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func setPlatform(opts []crane.Option, platform types.String) ([]crane.Option, error) {
	if !platform.IsNull() {
		platform, err := v1.ParsePlatform(platform.ValueString())
		if err != nil {
			return nil, err
		}
		opts = append(opts, crane.WithPlatform(platform))
	}
	return opts, nil
}

func readSourceDigest(ctx context.Context, source string, opts []crane.Option) (string, error) {
	// Image is a tarball
	if _, err := os.Stat(source); err == nil {
		img, err := crane.Load(source, opts...)
		if err != nil {
			return "", fmt.Errorf("failed to load image from tarball: %w", err)
		}
		hash, err := img.Digest()
		if err != nil {
			return "", fmt.Errorf("failed to get digest of image: %w", err)
		}
		return hash.String(), nil
	}
	// Image is a remote image reference
	tflog.Debug(ctx, fmt.Sprintf("Source path '%s' does not exist, treating as remote image reference", source))
	sourceDigest, err := crane.Digest(source, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to read remote image: %w", err)
	}
	return sourceDigest, nil
}

func performOperation(source string, destination string, opts []crane.Option) error {
	// Image is a tarball
	if _, err := os.Stat(source); err == nil {
		img, err := crane.Load(source, opts...)
		if err != nil {
			return fmt.Errorf("failed to load image from tarball: %w", err)
		}
		err = crane.Push(img, destination, opts...)
		if err != nil {
			return fmt.Errorf("failed to push image to destination: %w", err)
		}
	} else {
		err := crane.Copy(source, destination, opts...)
		if err != nil {
			return fmt.Errorf("failed to copy image to destination: %w", err)
		}
	}
	return nil
}
