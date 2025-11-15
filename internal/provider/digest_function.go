// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/terraform-plugin-framework/function"
)

// Ensure the implementation satisfies the desired interfaces.
var _ function.Function = &digestFunction{}

// digestFunction implements the Terraform function that resolves an image digest.
type digestFunction struct {
	provider *CraneProvider
}

// NewDigestFunction returns a new digest function instance.
func NewDigestFunction(provider *CraneProvider) function.Function {
	return &digestFunction{provider: provider}
}

func (f *digestFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "digest"
}

func (f *digestFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Return the content digest for an image reference.",
		MarkdownDescription: "Given an image reference, fetch the manifest from the remote registry and return its digest using [crane](https://github.com/google/go-containerregistry).",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "reference",
				MarkdownDescription: "A tag or digest identifying the image to inspect (for example `registry/repository:tag`).",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *digestFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var ref string

	if funcErr := req.Arguments.Get(ctx, &ref); funcErr != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, funcErr)
		return
	}

	opts := make([]crane.Option, 0, len(f.providerOptions())+1)
	opts = append(opts, f.providerOptions()...)
	opts = append(opts, crane.WithContext(ctx))

	digest, err := crane.Digest(ref, opts...)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewFuncError(fmt.Sprintf("reading digest for %q: %s", ref, err)))
		return
	}

	if funcErr := resp.Result.Set(ctx, digest); funcErr != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, funcErr)
	}
}

func (f *digestFunction) providerOptions() []crane.Option {
	if f == nil || f.provider == nil {
		return nil
	}

	return f.provider.options
}
