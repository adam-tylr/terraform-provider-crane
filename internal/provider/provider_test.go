package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
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
		{"  https://example.com//  ", "example.com"},
		{"docker.io", name.DefaultRegistry},
		{"registry-1.docker.io", name.DefaultRegistry},
		{"https://index.docker.io/v1/", name.DefaultRegistry},
		{"https://index.docker.io/v2/", name.DefaultRegistry},
		{"https://docker.io/v1/", name.DefaultRegistry},
		{"https://registry-1.docker.io/v2/", name.DefaultRegistry},
		{"123456789.dkr.ecr.eu-west-1.amazonaws.com", "123456789.dkr.ecr.eu-west-1.amazonaws.com"},
		{"https://123456789.dkr.ecr.eu-west-1.amazonaws.com/", "123456789.dkr.ecr.eu-west-1.amazonaws.com"},
		{"http://localhost:5000/v2/", "localhost:5000"},
	}

	for _, tt := range tests {
		got := normalizeRegistry(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeRegistry(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestDecodeBase64Auth(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantUsername string
		wantPassword string
		wantErr      bool
	}{
		{
			name:         "padded base64",
			input:        base64.StdEncoding.EncodeToString([]byte("myuser:mypassword")),
			wantUsername: "myuser",
			wantPassword: "mypassword",
		},
		{
			name:         "unpadded base64",
			input:        base64.RawStdEncoding.EncodeToString([]byte("myuser:mypassword")),
			wantUsername: "myuser",
			wantPassword: "mypassword",
		},
		{
			name:         "password with colons",
			input:        base64.StdEncoding.EncodeToString([]byte("AWS:my:secret:token")),
			wantUsername: "AWS",
			wantPassword: "my:secret:token",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			input:   "!!!not-base64!!!",
			wantErr: true,
		},
		{
			name:    "missing colon delimiter",
			input:   base64.StdEncoding.EncodeToString([]byte("useronly")),
			wantErr: true,
		},
		{
			name:    "empty username",
			input:   base64.StdEncoding.EncodeToString([]byte(":password")),
			wantErr: true,
		},
		{
			name:    "empty password",
			input:   base64.StdEncoding.EncodeToString([]byte("username:")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, pass, err := decodeBase64Auth(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user != tt.wantUsername || pass != tt.wantPassword {
				t.Errorf("decodeBase64Auth(%q) = (%q, %q), want (%q, %q)", tt.input, user, pass, tt.wantUsername, tt.wantPassword)
			}
		})
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
		"https://ghcr.io/": authn.AuthConfig{
			Username: "ghuser",
			Password: "ghpassword",
		},
		"empty.example.com": authn.AuthConfig{},
	}

	// 1. Exact match ECR and verify returned credentials
	auth, err := kc.Resolve(dummyResource{reg: "123456789.dkr.ecr.eu-west-1.amazonaws.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == authn.Anonymous {
		t.Fatalf("expected authenticated, got Anonymous")
	}
	cfg, err := auth.Authorization()
	if err != nil {
		t.Fatalf("unexpected Authorization() error: %v", err)
	}
	if cfg.Username != "AWS" || cfg.Password != "ecr-password" {
		t.Errorf("got credentials (%q, %q), expected (\"AWS\", \"ecr-password\")", cfg.Username, cfg.Password)
	}

	// 2. DockerHub match via docker.io alias
	auth, err = kc.Resolve(dummyResource{reg: "docker.io"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == authn.Anonymous {
		t.Fatalf("expected authenticated for docker.io, got Anonymous")
	}
	cfg, err = auth.Authorization()
	if err != nil {
		t.Fatalf("unexpected Authorization() error: %v", err)
	}
	if cfg.Username != "dockeruser" || cfg.Password != "dockerpassword" {
		t.Errorf("got credentials (%q, %q), expected (\"dockeruser\", \"dockerpassword\")", cfg.Username, cfg.Password)
	}

	// 3. Unnormalized key in staticKeychain
	auth, err = kc.Resolve(dummyResource{reg: "ghcr.io"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == authn.Anonymous {
		t.Fatalf("expected authenticated for ghcr.io, got Anonymous")
	}
	cfg, err = auth.Authorization()
	if err != nil {
		t.Fatalf("unexpected Authorization() error: %v", err)
	}
	if cfg.Username != "ghuser" || cfg.Password != "ghpassword" {
		t.Errorf("got credentials (%q, %q), expected (\"ghuser\", \"ghpassword\")", cfg.Username, cfg.Password)
	}

	// 4. Empty AuthConfig falls back to Anonymous
	auth, err = kc.Resolve(dummyResource{reg: "empty.example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != authn.Anonymous {
		t.Errorf("expected Anonymous for empty AuthConfig, got %v", auth)
	}

	// 5. Unknown registry fallback to Anonymous
	auth, err = kc.Resolve(dummyResource{reg: "unknown-registry.io"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != authn.Anonymous {
		t.Errorf("expected Anonymous, got %v", auth)
	}
}

func TestBuildStaticKeychain(t *testing.T) {
	encodedAuth := base64.StdEncoding.EncodeToString([]byte("tokenuser:tokenpass"))

	t.Run("valid username/password and base64 auth", func(t *testing.T) {
		kc, diags := buildStaticKeychain([]registryAuthModel{
			{
				Address:  types.StringValue("https://docker.io/"),
				Username: types.StringValue("u1"),
				Password: types.StringValue("p1"),
			},
			{
				Address: types.StringValue("123456789.dkr.ecr.eu-west-1.amazonaws.com"),
				Auth:    types.StringValue(encodedAuth),
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		dockerCfg, ok := kc[name.DefaultRegistry]
		if !ok || dockerCfg.Username != "u1" || dockerCfg.Password != "p1" {
			t.Errorf("unexpected docker.io config: %+v", dockerCfg)
		}
		ecrCfg, ok := kc["123456789.dkr.ecr.eu-west-1.amazonaws.com"]
		if !ok || ecrCfg.Username != "tokenuser" || ecrCfg.Password != "tokenpass" || ecrCfg.Auth != encodedAuth {
			t.Errorf("unexpected ECR config: %+v", ecrCfg)
		}
	})

	t.Run("unknown values at plan time do not error or insert empty AuthConfig", func(t *testing.T) {
		kc, diags := buildStaticKeychain([]registryAuthModel{
			{
				Address:  types.StringValue("123456789.dkr.ecr.eu-west-1.amazonaws.com"),
				Username: types.StringValue("AWS"),
				Password: types.StringUnknown(),
			},
			{
				Address: types.StringUnknown(),
				Auth:    types.StringUnknown(),
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics for unknown values: %v", diags)
		}
		if len(kc) != 0 {
			t.Errorf("expected empty keychain when credentials are unknown, got %+v", kc)
		}
	})

	errorCases := []struct {
		name   string
		models []registryAuthModel
	}{
		{
			name: "missing all credentials",
			models: []registryAuthModel{
				{
					Address: types.StringValue("index.docker.io"),
				},
			},
		},
		{
			name: "username without password",
			models: []registryAuthModel{
				{
					Address:  types.StringValue("index.docker.io"),
					Username: types.StringValue("user"),
				},
			},
		},
		{
			name: "password without username",
			models: []registryAuthModel{
				{
					Address:  types.StringValue("index.docker.io"),
					Password: types.StringValue("pass"),
				},
			},
		},
		{
			name: "conflicting auth and username/password",
			models: []registryAuthModel{
				{
					Address:  types.StringValue("index.docker.io"),
					Username: types.StringValue("user"),
					Password: types.StringValue("pass"),
					Auth:     types.StringValue(encodedAuth),
				},
			},
		},
		{
			name: "invalid base64 auth",
			models: []registryAuthModel{
				{
					Address: types.StringValue("index.docker.io"),
					Auth:    types.StringValue("not-valid-base64!!!"),
				},
			},
		},
		{
			name: "empty address after normalization",
			models: []registryAuthModel{
				{
					Address:  types.StringValue("https://"),
					Username: types.StringValue("user"),
					Password: types.StringValue("pass"),
				},
			},
		},
		{
			name: "empty username string",
			models: []registryAuthModel{
				{
					Address:  types.StringValue("index.docker.io"),
					Username: types.StringValue(""),
					Password: types.StringValue("pass"),
				},
			},
		},
		{
			name: "empty password string",
			models: []registryAuthModel{
				{
					Address:  types.StringValue("index.docker.io"),
					Username: types.StringValue("user"),
					Password: types.StringValue(""),
				},
			},
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := buildStaticKeychain(tc.models)
			if !diags.HasError() {
				t.Errorf("expected diagnostic error for %s, got none", tc.name)
			}
		})
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

func startAuthRegistryServer(t *testing.T, expectedUser, expectedPass string) string {
	t.Helper()

	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}

	regHandler := registry.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != expectedUser || pass != expectedPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		regHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	return u.Host
}

func seedAuthenticatedImage(t *testing.T, targetRef, username, password string) string {
	t.Helper()

	img, err := random.Image(512, 1)
	if err != nil {
		t.Fatalf("failed to generate random image: %v", err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatalf("failed to compute image digest: %v", err)
	}
	err = crane.Push(img, targetRef, crane.WithAuth(&authn.Basic{
		Username: username,
		Password: password,
	}))
	if err != nil {
		t.Fatalf("failed to seed authenticated image at %s: %v", targetRef, err)
	}
	return digest.String()
}

func TestAccProviderRegistryAuthUsernamePassword(t *testing.T) {
	srcHost := startAuthRegistryServer(t, "src-user", "src-pass")
	dstHost := startAuthRegistryServer(t, "dst-user", "dst-pass")

	srcRef := fmt.Sprintf("%s/app/source:v1", srcHost)
	dstRepo := fmt.Sprintf("%s/app/dest", dstHost)
	dstRef := fmt.Sprintf("%s:v1", dstRepo)

	expectedDigest := seedAuthenticatedImage(t, srcRef, "src-user", "src-pass")

	config := fmt.Sprintf(`
provider "crane" {
  registry_auth {
    address  = "http://%s/"
    username = "src-user"
    password = "src-pass"
  }

  registry_auth {
    address  = %q
    username = "dst-user"
    password = "dst-pass"
  }
}

resource "crane_image" "test" {
  source      = %q
  destination = %q
}

data "crane_digest" "test" {
  reference = crane_image.test.reference
}

data "crane_tags" "test" {
  repository = %q
  depends_on = [crane_image.test]
}
`, srcHost, dstHost, srcRef, dstRef, dstRepo)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(expectedDigest),
					),
					statecheck.ExpectKnownValue(
						"data.crane_digest.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(expectedDigest),
					),
					statecheck.ExpectKnownValue(
						"data.crane_tags.test",
						tfjsonpath.New("tags"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("v1"),
						}),
					),
				},
			},
		},
	})
}

func TestAccProviderRegistryAuthBase64(t *testing.T) {
	regHost := startAuthRegistryServer(t, "AWS", "ecr-secret-token")

	srcRef := fmt.Sprintf("%s/ecr/source:latest", regHost)
	dstRepo := fmt.Sprintf("%s/ecr/dest", regHost)
	dstRef := fmt.Sprintf("%s:latest", dstRepo)

	expectedDigest := seedAuthenticatedImage(t, srcRef, "AWS", "ecr-secret-token")
	encodedAuth := base64.StdEncoding.EncodeToString([]byte("AWS:ecr-secret-token"))

	config := fmt.Sprintf(`
provider "crane" {
  registry_auth {
    address = %q
    auth    = %q
  }
}

resource "crane_image" "test" {
  source      = %q
  destination = %q
}

data "crane_digest" "test" {
  reference = crane_image.test.reference
}
`, regHost, encodedAuth, srcRef, dstRef)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"crane_image.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(expectedDigest),
					),
					statecheck.ExpectKnownValue(
						"data.crane_digest.test",
						tfjsonpath.New("digest"),
						knownvalue.StringExact(expectedDigest),
					),
				},
			},
		},
	})
}

func TestAccProviderRegistryAuthInvalidCredentials(t *testing.T) {
	regHost := startAuthRegistryServer(t, "expected-user", "expected-pass")
	srcRef := fmt.Sprintf("%s/private/app:v1", regHost)
	dstRef := fmt.Sprintf("%s/private/app:v2", regHost)

	seedAuthenticatedImage(t, srcRef, "expected-user", "expected-pass")

	wrongCredsConfig := fmt.Sprintf(`
provider "crane" {
  registry_auth {
    address  = %q
    username = "expected-user"
    password = "wrong-password"
  }
}

resource "crane_image" "test" {
  source      = %q
  destination = %q
}
`, regHost, srcRef, dstRef)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      wrongCredsConfig,
				ExpectError: regexp.MustCompile(`(?i)unauthorized|401`),
			},
		},
	})
}

func TestAccProviderRegistryAuthConfigValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "crane" {
  registry_auth {
    address  = "index.docker.io"
    username = "user"
    password = "pass"
    auth     = "dXNlcjpwYXNz"
  }
}

data "crane_tags" "test" {
  repository = "index.docker.io/library/alpine"
}
`,
				ExpectError: regexp.MustCompile("Conflicting Registry Auth Credentials"),
			},
			{
				Config: `
provider "crane" {
  registry_auth {
    address  = "index.docker.io"
    username = "user"
  }
}

data "crane_tags" "test" {
  repository = "index.docker.io/library/alpine"
}
`,
				ExpectError: regexp.MustCompile("Incomplete Registry Auth Credentials"),
			},
			{
				Config: `
provider "crane" {
  registry_auth {
    address = "index.docker.io"
    auth    = "not-valid-base64!!!"
  }
}

data "crane_tags" "test" {
  repository = "index.docker.io/library/alpine"
}
`,
				ExpectError: regexp.MustCompile("Invalid Registry Auth Value"),
			},
		},
	})
}
