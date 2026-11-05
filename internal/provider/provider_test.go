package provider

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"crane": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {

}

func TestNormalizeRegistry(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"https://example.com", "example.com"},
		{"http://example.com/", "example.com"},
		{"docker.io", name.DefaultRegistry},
		{"registry-1.docker.io", name.DefaultRegistry},
		{"123456789.dkr.ecr.eu-west-1.amazonaws.com", "123456789.dkr.ecr.eu-west-1.amazonaws.com"},
		{"https://123456789.dkr.ecr.eu-west-1.amazonaws.com/", "123456789.dkr.ecr.eu-west-1.amazonaws.com"},
	}

	for _, tt := range tests {
		got := normalizeRegistry(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeRegistry(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

type dummyResource struct {
	reg string
}

func (d dummyResource) String() string      { return d.reg + "/myrepo" }
func (d dummyResource) RegistryStr() string { return d.reg }

func TestStaticKeychain(t *testing.T) {
	kc := staticKeychain{
		"123456789.dkr.ecr.eu-west-1.amazonaws.com": authn.AuthConfig{
			Username: "AWS",
			Password: "ecr-password",
		},
		"index.docker.io": authn.AuthConfig{
			Username: "dockeruser",
			Password: "dockerpassword",
		},
	}

	// 1. Exact match ECR
	auth, err := kc.Resolve(dummyResource{reg: "123456789.dkr.ecr.eu-west-1.amazonaws.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == authn.Anonymous {
		t.Errorf("expected authenticated, got Anonymous")
	}

	// 2. DockerHub match via docker.io alias
	auth, err = kc.Resolve(dummyResource{reg: "docker.io"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == authn.Anonymous {
		t.Errorf("expected authenticated for docker.io, got Anonymous")
	}

	// 3. Unknown registry fallback to Anonymous
	auth, err = kc.Resolve(dummyResource{reg: "unknown-registry.io"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != authn.Anonymous {
		t.Errorf("expected Anonymous, got %v", auth)
	}
}

func TestProviderSchema(t *testing.T) {
	ctx := context.Background()
	p := New("test")()

	var resp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", resp.Diagnostics)
	}

	diags := resp.Schema.ValidateImplementation(ctx)
	if diags.HasError() {
		t.Fatalf("schema validation error: %v", diags)
	}
}
