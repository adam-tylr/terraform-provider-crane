// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testing

import (
	"context"
	"fmt"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var _ statecheck.StateCheck = checkRemoteImage{}

type checkRemoteImage struct {
	resourceAddress string
}

func (e checkRemoteImage) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	var resource *tfjson.StateResource

	if req.State == nil {
		resp.Error = fmt.Errorf("state is nil")
		return
	}

	if req.State.Values == nil {
		resp.Error = fmt.Errorf("state does not contain any state values")
		return
	}

	if req.State.Values.RootModule == nil {
		resp.Error = fmt.Errorf("state does not contain a root module")
		return
	}

	for _, r := range req.State.Values.RootModule.Resources {
		if e.resourceAddress == r.Address {
			resource = r
			break
		}
	}

	if resource == nil {
		resp.Error = fmt.Errorf("%s - Resource not found in state", e.resourceAddress)
		return
	}

	idValue, _ := tfjsonpath.Traverse(resource.AttributeValues, tfjsonpath.New("id"))
	id, ok := idValue.(string)
	if !ok {
		resp.Error = fmt.Errorf("expected id to be a string, but got %T", idValue)
		return
	}
	platformValue, _ := tfjsonpath.Traverse(resource.AttributeValues, tfjsonpath.New("platform"))

	repo, err := name.ParseReference(id)
	if err != nil {
		resp.Error = fmt.Errorf("failed to parse image reference %q: %w", id, err)
		return
	}

	// Fetch manifest index/descriptor using standard remote package
	desc, err := remote.Get(repo)
	if err != nil {
		resp.Error = fmt.Errorf("failed to get image from registry: %w", err)
		return
	}

	if platformValue == nil {
		if !desc.MediaType.IsIndex() {
			resp.Error = fmt.Errorf("platform not specified, expected image to have multiple platforms (manifest list/index), but media type is %s", desc.MediaType)
			return
		}

		idx, err := desc.ImageIndex()
		if err != nil {
			resp.Error = fmt.Errorf("failed to parse image index: %w", err)
			return
		}

		im, err := idx.IndexManifest()
		if err != nil {
			resp.Error = fmt.Errorf("failed to read index manifest: %w", err)
			return
		}

		var platforms []v1.Platform
		for _, manifestItem := range im.Manifests {
			if manifestItem.Platform != nil && manifestItem.Platform.Architecture != "unknown" && manifestItem.Platform.OS != "unknown" {
				platforms = append(platforms, *manifestItem.Platform)
			}
		}
		if len(platforms) < 2 {
			resp.Error = fmt.Errorf("platform not specified, expected image to have multiple platforms: %v", platforms)
			return
		}
	} else {
		platform, ok := platformValue.(string)
		if !ok && platformValue != nil {
			resp.Error = fmt.Errorf("expected platform to be a string, but got %T", platformValue)
			return
		}
		p, err := v1.ParsePlatform(platform)
		if err != nil {
			resp.Error = fmt.Errorf("failed to parse platform %q: %w", platform, err)
			return
		}

		opts := []crane.Option{crane.WithPlatform(p)}
		d, err := crane.Digest(id, opts...)
		if err != nil {
			resp.Error = fmt.Errorf("failed to resolve platform digest: %w", err)
			return
		}

		if desc.Digest.String() != d {
			if desc.MediaType.IsIndex() {
				idx, err := desc.ImageIndex()
				if err != nil {
					resp.Error = fmt.Errorf("failed to parse image index: %w", err)
					return
				}
				im, err := idx.IndexManifest()
				if err != nil {
					resp.Error = fmt.Errorf("failed to read index manifest: %w", err)
					return
				}

				found := false
				for _, manifest := range im.Manifests {
					if manifest.Digest.String() == d {
						found = true
						break
					}
				}
				if !found {
					resp.Error = fmt.Errorf("resolved digest %s for platform %s not found in manifest index", d, platform)
					return
				}
			} else {
				resp.Error = fmt.Errorf("image digest %s does not match expected digest: %s", desc.Digest.String(), d)
				return
			}
		}
	}
}

func CheckRemoteImage(resourceAddress string) statecheck.StateCheck {
	return checkRemoteImage{
		resourceAddress: resourceAddress,
	}
}
