// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testing

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var SOURCE_REGISTRY = func() string {
	reg := os.Getenv("SOURCE_REGISTRY")
	if reg == "" {
		return "localhost:5001"
	}
	return reg
}()

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateRandomString(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func CreateSourceRef(image string) string {
	return fmt.Sprintf("%s/%s", SOURCE_REGISTRY, image)
}

func CreateRepository(t *testing.T) (string, func()) {
	t.Helper()

	repoName := "test-repo-" + strings.ToLower(t.Name()) + "-" + generateRandomString(6)
	repoUri := fmt.Sprintf("%s/%s", SOURCE_REGISTRY, repoName)
	t.Logf("Generated mock repository URI: %s", repoUri)

	return repoUri, func() {
		// Teardown: For a local registry, we don't strictly need to delete the repo
		// since the registry is ephemeral and will be torn down at the end of the run.
		t.Logf("Tearing down mock repository: %s", repoUri)
	}
}

// SeedMockImage generates a tiny 1KB mock image and pushes it to a remote registry reference.
func SeedMockImage(t *testing.T, targetRef string) error {
	t.Helper()
	t.Logf("Seeding programmatic mock image: %s", targetRef)
	img, err := random.Image(1024, 1)
	if err != nil {
		return fmt.Errorf("failed to generate random image: %w", err)
	}
	return crane.Push(img, targetRef)
}

// SeedMockMultiArchImage generates a multi-platform manifest index of mock images and pushes it.
func SeedMockMultiArchImage(t *testing.T, targetRef string, platforms []string) error {
	t.Helper()
	t.Logf("Seeding programmatic multi-arch mock image index: %s (platforms: %v)", targetRef, platforms)

	var idx v1.ImageIndex = empty.Index

	for _, platStr := range platforms {
		p, err := v1.ParsePlatform(platStr)
		if err != nil {
			return fmt.Errorf("failed to parse platform %s: %w", platStr, err)
		}

		img, err := random.Image(1024, 1)
		if err != nil {
			return fmt.Errorf("failed to generate random image: %w", err)
		}

		cfg, err := img.ConfigFile()
		if err != nil {
			return fmt.Errorf("failed to get image config: %w", err)
		}
		cfg.Architecture = p.Architecture
		cfg.OS = p.OS
		cfg.Variant = p.Variant

		img, err = mutate.ConfigFile(img, cfg)
		if err != nil {
			return fmt.Errorf("failed to set platform config: %w", err)
		}

		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: p,
			},
		})
	}

	ref, err := name.ParseReference(targetRef)
	if err != nil {
		return fmt.Errorf("failed to parse reference %s: %w", targetRef, err)
	}
	return remote.WriteIndex(ref, idx)
}

func CreateLocalTarball(t *testing.T, imageRef string) string {
	t.Helper()

	// Extract a safe name for the tar file
	parts := strings.Split(imageRef, "/")
	namePart := parts[len(parts)-1]
	namePart = strings.ReplaceAll(namePart, ":", "-")
	namePart = strings.ReplaceAll(namePart, "@", "-")

	cwd, _ := os.Getwd()
	index := strings.LastIndex(cwd, "terraform-provider-crane")
	root := cwd[:index+len("terraform-provider-crane")]
	testingDir := path.Join(root, "testing")
	tarPath := path.Join(testingDir, fmt.Sprintf("%s.tar.gz", namePart))

	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		t.Logf("Creating local tarball programmatically: %s", tarPath)

		// Create a random image entirely locally without pulling
		img, err := random.Image(1024, 1)
		if err != nil {
			t.Fatalf("failed to generate random image: %v", err)
		}

		err = crane.Save(img, imageRef, tarPath)
		if err != nil {
			t.Fatalf("failed to create tarball for image %s: %v", imageRef, err)
		}
	}

	return tarPath
}

func CopyImagesToRepository(t *testing.T, targetRepoUri string) []string {
	t.Helper()

	imageTags := []string{"latest", "alpine"}

	for _, tag := range imageTags {
		src := CreateSourceRef(fmt.Sprintf("nginx/nginx:%s", tag))
		dst := fmt.Sprintf("%s:%s", targetRepoUri, tag)

		// Ensure source exists by seeding it
		if err := SeedMockImage(t, src); err != nil {
			t.Fatalf("failed to seed source mock image at %s: %v", src, err)
		}

		t.Logf("Copying seeded source image '%s' to target '%s'", src, dst)
		if err := crane.Copy(src, dst); err != nil {
			t.Fatalf("failed to copy: %v", err)
		}
	}
	return imageTags
}

func DeleteRemoteImage(t *testing.T, repoUri string, tag string) {
	t.Helper()

	ref := fmt.Sprintf("%s:%s", repoUri, tag)
	t.Logf("Resolving digest for delete: %s", ref)
	digest, err := crane.Digest(ref)
	if err != nil {
		t.Fatalf("failed to get digest of %s: %v", ref, err)
	}

	deleteRef := fmt.Sprintf("%s@%s", repoUri, digest)
	t.Logf("Deleting remote image by digest: %s", deleteRef)
	err = crane.Delete(deleteRef)
	if err != nil {
		t.Fatalf("failed to delete image %s: %v", deleteRef, err)
	}
}
