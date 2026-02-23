# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is a Terraform provider for the [Crucible Framework](https://github.com/cmu-sei/crucible), enabling infrastructure-as-code management of Crucible resources. The provider interfaces with three Crucible APIs: Player API, VM API, and Caster API.

**Managed Resources:**
- `crucible_player_virtual_machine` - Virtual machines (VSphere, Guacamole, Proxmox)
- `crucible_player_view` - Player views with nested teams and applications
- `crucible_player_application_template` - Application templates
- `crucible_player_user` - Player users
- `crucible_vlan` - VLAN allocation from Caster API

## Technology Stack

- **Language:** Go (1.21+)
- **Framework:** Terraform Plugin Framework (v1.4.2+) - migrated from legacy SDK in v1.0.0
- **Build Tool:** GoReleaser
- **Authentication:** OAuth2 Password Credentials Grant with token caching
- **HTTP Client:** Centralized `CrucibleClient` with automatic retry and rich error messages

## Project Structure

```
terraform-provider-crucible/
├── cmd/
│   └── main.go                                      # Plugin entry point (Framework provider server)
├── internal/
│   ├── client/
│   │   ├── client.go                                # Centralized HTTP client with token caching
│   │   └── client_test.go                           # Client unit tests
│   ├── provider/
│   │   ├── provider_framework.go                    # Plugin Framework provider (v1.0.0+)
│   │   ├── provider.go                              # Legacy SDK v1 provider (deprecated, will be removed)
│   │   ├── player_user_resource.go                  # User resource (Framework)
│   │   ├── player_application_template_resource.go  # App template resource (Framework)
│   │   ├── caster_vlan_resource.go                  # VLAN resource (Framework)
│   │   ├── player_virtual_machine_resource.go       # VM resource (Framework)
│   │   ├── player_view_resource.go                  # View resource (Framework)
│   │   ├── *_server.go                              # Legacy SDK v1 resources (deprecated)
│   │   └── *_test.go                                # Terraform acceptance tests
│   ├── api/
│   │   ├── vm_api.go                                # VM API wrappers (both Framework and SDK v1)
│   │   ├── player_api_view.go                       # Player View API wrappers
│   │   ├── player_api_application.go                # Application API wrappers
│   │   ├── player_api_team.go                       # Team API wrappers
│   │   ├── player_api_user.go                       # User API wrappers
│   │   ├── player_api_application_template.go       # Template API wrappers
│   │   └── caster_api_vlan.go                       # VLAN API wrappers
│   ├── structs/
│   │   └── structs.go                               # Data structures for API payloads
│   └── util/
│       └── util.go                                  # Legacy utilities (mostly deprecated)
├── configs/
│   └── testConfigs.json                             # Test fixture data
├── docs/
│   └── index.md                                     # Terraform Registry documentation
├── go.mod                                           # Go module definition (now committed)
├── MIGRATION.md                                     # v0.x to v1.0.0 migration guide
├── .goreleaser.yml                                  # Multi-platform build configuration
└── Dockerfile                                       # Multi-stage build for releases
```

## Common Commands

### Development Setup

The provider now includes `go.mod` in source control (as of v1.0.0). Initialize dependencies:

```bash
# Download dependencies (required before building)
go mod tidy
```

### Building

```bash
# Build for local development (creates executable in cmd/)
cd cmd
go build

# Build for local development (creates executable in project root)
cd cmd
go build -o ../terraform-provider-crucible

# Cross-compile for specific platform
cd cmd
GOOS=linux GOARCH=amd64 go build -o ../terraform-provider-crucible
```

### Testing

```bash
# Run all tests
go test -v crucible_provider/internal/provider

# Run tests with coverage
go test -v -cover crucible_provider/internal/provider

# Run specific test
go test -v crucible_provider/internal/provider -run TestAccBasicSuccessful
```

**Note:** Most acceptance tests are commented out and require a live Crucible environment with proper credentials.

### Local Testing with Terraform

```bash
# 1. Build the provider
cd cmd && go build -o ../terraform-provider-crucible

# 2. Install to Terraform plugins directory
# Linux/macOS:
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/cmusei/crucible/0.1.0/linux_amd64/
cp terraform-provider-crucible ~/.terraform.d/plugins/registry.terraform.io/cmusei/crucible/0.1.0/linux_amd64/

# 3. Create a test main.tf with required provider config
# 4. Run terraform commands
terraform init
terraform plan
terraform apply
```

### Release Process

Releases are automated via GitHub Actions + GoReleaser:

```bash
# Create and push a tag
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0

# GitHub Actions will:
# 1. Build for all platforms (Linux, Darwin, Windows; 386, amd64, arm, arm64)
# 2. Create release artifacts
# 3. Sign checksums with GPG
# 4. Publish to GitHub releases
```

## Architecture

### Plugin Framework Architecture (v1.0.0+)

The provider now uses Terraform Plugin Framework for type safety and improved developer experience.

1. **Provider Definition** (`internal/provider/provider_framework.go`):
   - Implements `provider.Provider` interface
   - Type-safe configuration model using `types.String`, `types.Bool`, etc.
   - Environment variables override HCL configuration
   - Creates `CrucibleClient` and passes to all resources via `Configure()`

2. **Centralized HTTP Client** (`internal/client/client.go`):
   - `CrucibleClient` handles all HTTP communication
   - OAuth2 token caching with automatic refresh
   - Rich error extraction from API responses
   - Retry logic for 401 Unauthorized
   - Eliminates 940+ lines of duplicated HTTP code

3. **Resource Implementation** (`internal/provider/*_resource.go`):
   - Each `*_resource.go` file implements `resource.Resource` interface
   - Type-safe models using Framework types (`types.String`, `types.List`, `types.Object`)
   - Schema with validators and plan modifiers
   - CRUD methods with context and structured diagnostics
   - Import state support

4. **API Layer** (`internal/api/*_api.go`):
   - Each file contains both Framework and legacy SDK v1 functions
   - Framework functions: `func(ctx context.Context, client *CrucibleClient, ...) error`
   - SDK v1 functions: `func(..., m map[string]string) error` (deprecated, will be removed)
   - Uses `CrucibleClient` methods: `DoGet()`, `DoPost()`, `DoPut()`, `DoDelete()`

5. **Data Structures** (`internal/structs/structs.go`):
   - Go structs for API payloads with JSON tags
   - Framework resources use these directly, no ToMap/FromMap needed
   - Legacy SDK v1 code still uses ToMap/FromMap (will be removed)

### Authentication Flow (v1.0.0+)

```
CrucibleClient.GetToken(ctx)
  → Check cached token (fast path)
  → If expired or missing:
    → OAuth2 Password Credentials Token exchange
    → Cache token with expiry
  → Return access token
  → DoRequest() injects: "Authorization: Bearer <token>"
  → On 401, invalidate cache and retry once
```

### Legacy Architecture (v0.9.x - SDK v1)

The legacy architecture used `*_server.go` files with manual state management and per-request authentication. This code is deprecated and will be removed in a future release. See git tag `v0.9.0` for the last SDK v1 version.

### Resource CRUD Pattern

Each resource follows this pattern:

```
Terraform Operation → CRUD Function → API Wrapper → HTTP Request → Crucible API

Example for VM creation:
terraform apply
  → playerVirtualMachineCreate() [provider/player_virtual_machine_server.go]
  → api.CreateVM() [api/vm_api.go]
  → HTTP POST to {vm_api_url}/vms
  → Returns error or nil
```

## Key Implementation Patterns

### Optional Fields with interface{}

Go doesn't have an `Option<T>` type. This provider uses `interface{}` for optional fields to distinguish between:
- Field not set (nil)
- Field explicitly set to empty string ("")
- Field set to value ("someValue")

Example from `structs.AppInfo`:
```go
type AppInfo struct {
    Name       interface{}  // Can be nil, "", or "AppName"
    Embeddable interface{}  // Can be nil, "true", "false", true, or false
}
```

### URL Normalization

All API URLs are normalized in `client.normalizeAPIURL()`:
- Strips trailing `/` and `/api`
- Appends `/api/`
- Example: `https://player.crucible.dev` → `https://player.crucible.dev/api/`

### State Management (Framework)

Plugin Framework uses type-safe models:
- Resources define model structs with `tfsdk` tags
- Framework handles serialization/deserialization automatically
- `req.Plan.Get(ctx, &model)` - read planned values
- `resp.State.Set(ctx, &model)` - write state values
- No manual `ToMap()`/`FromMap()` conversions needed

### Ordering Requirements

**IMPORTANT:** Resources within views are automatically sorted alphabetically to avoid spurious state changes:
- Applications sorted by `name` in Read()
- Teams sorted by `name` in Create()
- Users within teams sorted by `user_id`
- App instances within teams sorted by `name`

Framework's built-in comparison handles this - if only order changes, no update is triggered.

## Configuration

### Provider Block

Required environment variables (or set in provider block):

```bash
export SEI_CRUCIBLE_USERNAME=<username>
export SEI_CRUCIBLE_PASSWORD=<password>
export SEI_CRUCIBLE_AUTH_URL=<auth service URL>
export SEI_CRUCIBLE_TOKEN_URL=<token endpoint URL>  # v1.0.0+: renamed from SEI_CRUCIBLE_TOK_URL
export SEI_CRUCIBLE_CLIENT_ID=<OAuth client ID>
export SEI_CRUCIBLE_CLIENT_SECRET=<OAuth client secret>
export SEI_CRUCIBLE_CLIENT_SCOPES='["player-api","vm-api","caster-api"]'
export SEI_CRUCIBLE_VM_API_URL=<VM API base URL>
export SEI_CRUCIBLE_PLAYER_API_URL=<Player API base URL>
export SEI_CRUCIBLE_CASTER_API_URL=<Caster API base URL>
```

**Note:** The old `SEI_CRUCIBLE_TOK_URL` is still supported for backward compatibility but is deprecated.

### Resource Examples

See `docs/index.md` for comprehensive examples of each resource type. Key examples:

**Virtual Machine (VSphere):**
```hcl
resource "crucible_player_virtual_machine" "example" {
  vm_id    = "6a7ec409-d275-4b31-94d3-a51cb61d2519"
  name     = "User1"
  team_ids = ["46420756-9421-41b7-99b4-1b6d2cba29b3"]
}
```

**View with Teams and Applications:**
```hcl
resource "crucible_player_view" "example" {
  name        = "example"
  description = "This was created from terraform!"
  status      = "Active"

  application {
    name               = "testApp"
    embeddable         = false  # v1.0.0+: proper boolean
    load_in_background = true   # v1.0.0+: proper boolean
  }

  team {
    name = "test_team"
    user {
      user_id = "6fb5b293-668b-4eb6-b614-dfdd6b0e0acf"
    }
    app_instance {
      name          = "testApp"
      display_order = 0
    }
  }
}
```

## Version-Specific Notes

### v1.0.0+ Boolean Syntax (Plugin Framework)

The `embeddable` and `load_in_background` fields in application blocks now use proper boolean types:
```hcl
application {
  embeddable = false  # ✅ Correct in v1.0.0+
}
```

### v0.9.x Boolean Syntax (Legacy SDK)

**If using v0.9.x or earlier**, these fields require quoted strings:
```hcl
application {
  embeddable = "false"  # ✅ Correct in v0.9.x
}
```

**See [MIGRATION.md](./MIGRATION.md) for upgrade instructions.**

### No go.mod in Source Control

The `go.mod` file is:
- NOT committed to source control (in `.gitignore`)
- Created dynamically during build: `go mod init crucible_provider`
- Required before any `go` commands work

### VM ID Generation

If `vm_id` is omitted from a VM resource, the provider generates a GUID automatically. However, for VSphere VMs, you typically want to provide the VSphere VM's GUID explicitly.

### Default URL Behavior

If `url` is omitted from a VM resource, the API computes a default URL based on the VM's type. The provider tracks this via the computed `default_url` field to avoid spurious state diffs.

## Debugging

### Enable Detailed Logging

The provider uses Go's `log.Printf()` extensively. To see logs during Terraform execution:

```bash
# Set Terraform logging level
export TF_LOG=DEBUG
export TF_LOG_PATH=./terraform.log

terraform apply

# View logs
tail -f terraform.log
```

### Common Issues

**Authentication Failures:**
- Verify all `SEI_CRUCIBLE_*` environment variables are set
- Check token endpoint URL is correct
- Ensure client_id/client_secret have proper scopes
- Test authentication separately: Check `util.GetAuth()` logic

**State Drift Detection:**
- If Terraform constantly detects changes despite no modifications:
  - Check alphabetical ordering of teams/apps/users
  - Verify boolean field quoting in view application blocks
  - Look for URL normalization issues (trailing slashes)

**Build Errors:**
- Always run `go mod init crucible_provider && go mod tidy` first
- Ensure Go 1.23+ is installed
- Check that `cmd/` directory is the build target

## Testing Strategy

### Unit Tests vs Acceptance Tests

- **Unit Tests:** None present (provider uses acceptance tests)
- **Acceptance Tests:** In `*_test.go` files, mostly commented out
  - Require live Crucible environment
  - Use `resource.Test()` from Terraform SDK
  - Verify both local state and remote API state

### Running Acceptance Tests

Acceptance tests require environment setup:

1. Set all `SEI_CRUCIBLE_*` environment variables
2. Ensure Crucible services are running and accessible
3. Uncomment test cases in `*_test.go` files
4. Run: `go test -v crucible_provider/internal/provider -run TestAccBasicSuccessful`

### Test Data

Test fixture data is in `configs/testConfigs.json` with predefined GUIDs for resources.

## Working with the Codebase

### Adding a New Resource Type

1. Create `internal/provider/<resource>_server.go`:
   - Define resource schema
   - Implement CRUD functions (Create, Read, Update, Delete)

2. Create `internal/api/<api>_<resource>.go`:
   - Implement API wrapper functions for HTTP calls

3. Add struct to `internal/structs/structs.go` if needed:
   - Define Go struct for API payload
   - Implement `ToMap()` and `FromMap()` methods

4. Register resource in `internal/provider/provider.go`:
   ```go
   ResourcesMap: map[string]*schema.Resource{
       "crucible_new_resource": newResource(),
   }
   ```

### Modifying Existing Resources

1. Read the resource's `*_server.go` file first to understand the schema
2. Update schema definition in the resource function
3. Update CRUD functions to handle new fields
4. Update corresponding `structs.go` struct and methods
5. Update API wrapper functions in `internal/api/` if API changes
6. Test locally with a `main.tf` configuration

### Code Patterns to Follow

**Error Handling:**
```go
if err != nil {
    log.Printf("! Error description with context")
    return err
}
```

**HTTP Requests:**
```go
auth, err := util.GetAuth(m)  // Get OAuth token
req, err := http.NewRequest("POST", util.GetPlayerApiUrl(m)+"endpoint", body)
req.Header.Add("Authorization", "Bearer "+auth)
req.Header.Set("Content-Type", "application/json")
```

**Schema Definition:**
```go
"field_name": {
    Type:     schema.TypeString,
    Required: true,  // or Optional: true
    ForceNew: true,  // If changing this field requires resource recreation
}
```

## Environment Variables

All provider configuration can be set via environment variables with `SEI_CRUCIBLE_` prefix:

| Variable | Purpose |
|----------|---------|
| `SEI_CRUCIBLE_USERNAME` | Authentication username |
| `SEI_CRUCIBLE_PASSWORD` | Authentication password |
| `SEI_CRUCIBLE_AUTH_URL` | OAuth authorization endpoint |
| `SEI_CRUCIBLE_TOK_URL` | OAuth token endpoint |
| `SEI_CRUCIBLE_CLIENT_ID` | OAuth client ID |
| `SEI_CRUCIBLE_CLIENT_SECRET` | OAuth client secret |
| `SEI_CRUCIBLE_CLIENT_SCOPES` | JSON array of OAuth scopes |
| `SEI_CRUCIBLE_VM_API_URL` | VM API base URL |
| `SEI_CRUCIBLE_PLAYER_API_URL` | Player API base URL |
| `SEI_CRUCIBLE_CASTER_API_URL` | Caster API base URL |

## Release Process

Releases use GoReleaser and are triggered by pushing version tags:

```bash
# Create annotated tag
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

GitHub Actions workflow (`.github/workflows/release.yml`):
1. Runs GoReleaser with `.goreleaser.yml` configuration
2. Builds binaries for multiple platforms
3. Signs checksums with GPG
4. Publishes to GitHub releases

**Platforms Built:**
- Linux: 386, amd64, arm, arm64
- Darwin (macOS): amd64, arm64
- Windows: 386, amd64, arm, arm64

## Reporting Issues

Report bugs in the main Crucible issue tracker:
https://github.com/cmu-sei/crucible/issues
