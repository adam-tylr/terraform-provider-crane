package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &TagsDataSource{}

func NewTagsDataSource() datasource.DataSource {
	return &TagsDataSource{}
}

// TagsDataSource defines the data source implementation.
type TagsDataSource struct {
	options []crane.Option
}

// TagsDataSourceModel describes the data source data model.
type TagsDataSourceModel struct {
	Id             types.String `tfsdk:"id"`
	Repository     types.String `tfsdk:"repository"`
	FullRef        types.Bool   `tfsdk:"full_ref"`
	OmitDigestTags types.Bool   `tfsdk:"omit_digest_tags"`
	Tags           types.List   `tfsdk:"tags"`
}

func (d *TagsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tags"
}

func (d *TagsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List tags for a given repository",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Equivalent to the repository name",
				Computed:            true,
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "The repository to list",
				Required:            true,
			},
			"full_ref": schema.BoolAttribute{
				MarkdownDescription: "If true, the full ref will be returned",
				Optional:            true,
			},
			"omit_digest_tags": schema.BoolAttribute{
				MarkdownDescription: "If true, the digest tags will be omitted",
				Optional:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "List of tags",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *TagsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagsDataSourceModel
	o := crane.GetOptions(d.options...)

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	src := data.Repository.ValueString()
	repo, err := name.NewRepository(src, o.Name...)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error parsing repository name: %s", src), err.Error())
		return
	}

	allTags, err := remote.List(repo, o.Remote...)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error listing tags for repository: %s", src), err.Error())
		return
	}

	tags := make([]string, 0, len(allTags))
	omitDigestTags := data.OmitDigestTags.ValueBool()
	returnFullRef := data.FullRef.ValueBool()
	for _, tag := range allTags {
		if omitDigestTags && strings.HasPrefix(tag, "sha256-") {
			continue
		}
		if returnFullRef {
			tags = append(tags, repo.Tag(tag).String())
		} else {
			tags = append(tags, tag)
		}
	}

	data.Id = types.StringValue(src)
	tagList, diags := types.ListValueFrom(ctx, types.StringType, tags)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	data.Tags = tagList

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
