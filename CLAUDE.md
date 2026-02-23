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

- **Language:** Go (1.23+)
- **Framework:** Terraform Plugin SDK (legacy SDK, not Framework)
- **Build Tool:** GoReleaser
- **Authentication:** OAuth2 Password Credentials Grant
- **HTTP Client:** Standard library `net/http`

## Project Structure

```
terraform-provider-crucible/
├── cmd/
│   └── main.go                           # Plugin entry point
├── internal/
│   ├── provider/
│   │   ├── provider.go                   # Provider definition and config
│   │   ├── player_virtual_machine_server.go  # VM resource CRUD
│   │   ├── player_view_server.go         # View resource CRUD
│   │   ├── player_application_template_server.go
│   │   ├── player_user_server.go
│   │   ├── caster_vlan_server.go
│   │   └── *_test.go                     # Terraform acceptance tests (mostly commented out)
│   ├── api/
│   │   ├── vm_api.go                     # VM API wrappers
│   │   ├── player_api_view.go            # Player View API wrappers
│   │   ├── player_api_application.go
│   │   ├── player_api_team.go
│   │   ├── player_api_user.go
│   │   ├── player_api_application_template.go
│   │   └── caster_api_vlan.go
│   ├── structs/
│   │   └── structs.go                    # Data structures for API payloads
│   └── util/
│       └── util.go                       # Authentication, URL normalization, helpers
├── configs/
│   └── testConfigs.json                  # Test fixture data
├── docs/
│   └── index.md                          # Terraform Registry documentation
├── scripts/
│   ├── build.bat                         # Windows build script
│   └── test.bat                          # Windows test script
├── .goreleaser.yml                       # Multi-platform build configuration
└── Dockerfile                            # Multi-stage build for releases
```

## Common Commands

### Development Setup

The provider does NOT include `go.mod` in source control. Initialize on first use:

```bash
# Initialize Go module (required before any other commands)
go mod init crucible_provider
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

### Provider Configuration Flow

1. **Provider Definition** (`internal/provider/provider.go`):
   - Defines schema for provider configuration block
   - Maps environment variables to config fields
   - `config()` function creates `map[string]string` with credentials/URLs

2. **Resource Registration** (`internal/provider/provider.go`):
   - `ResourcesMap` registers resource types to their implementations
   - Each resource points to a function returning `*schema.Resource`

3. **Resource Implementation** (`internal/provider/*_server.go`):
   - Each `*_server.go` file defines one resource type
   - Resource function returns `*schema.Resource` with:
     - Schema: Field definitions (types, validation, required/optional)
     - CRUD functions: Create, Read, Update, Delete
   - CRUD functions receive `*schema.ResourceData` (local state) and `interface{}` (provider config map)

4. **API Layer** (`internal/api/*_api.go`):
   - Wrapper functions for HTTP calls to Crucible APIs
   - Each function takes structs and config map, returns error
   - Handles authentication via `util.GetAuth()`

5. **Data Structures** (`internal/structs/structs.go`):
   - Defines Go structs for API payloads
   - `ToMap()` methods convert structs to Terraform state maps
   - `FromMap()` functions create structs from Terraform state

### Authentication Flow

```
util.GetAuth(config map)
  → OAuth2 Password Credentials Token exchange
  → Returns access token string
  → Token added to HTTP request: "Authorization: Bearer <token>"
```

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

All API URLs are normalized in `util.GetApiUrl()`:
- Strips trailing `/` and `/api`
- Appends `/api/`
- Example: `https://player.crucible.dev` → `https://player.crucible.dev/api/`

### State Management

Terraform state is stored as `map[string]interface{}`:
- Read from `schema.ResourceData` via `d.Get(key)`
- Written to `schema.ResourceData` via `d.Set(key, value)`
- Structs converted to/from maps using `ToMap()` and `FromMap()` methods

### Ordering Requirements

**IMPORTANT:** Resources within views must be alphabetically ordered to avoid spurious state changes:
- Applications ordered by `name`
- Teams ordered by `name`
- Users within teams ordered by `user_id`
- App instances within teams ordered by `name`

This is handled in the provider's comparison logic - if order changes but content doesn't, Terraform won't detect a change.

## Configuration

### Provider Block

Required environment variables (or set in provider block):

```bash
export SEI_CRUCIBLE_USERNAME=<username>
export SEI_CRUCIBLE_PASSWORD=<password>
export SEI_CRUCIBLE_AUTH_URL=<auth service URL>
export SEI_CRUCIBLE_TOK_URL=<token endpoint URL>
export SEI_CRUCIBLE_CLIENT_ID=<OAuth client ID>
export SEI_CRUCIBLE_CLIENT_SECRET=<OAuth client secret>
export SEI_CRUCIBLE_CLIENT_SCOPES='["scope1","scope2"]'
export SEI_CRUCIBLE_VM_API_URL=<VM API base URL>
export SEI_CRUCIBLE_PLAYER_API_URL=<Player API base URL>
export SEI_CRUCIBLE_CASTER_API_URL=<Caster API base URL>
```

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
    embeddable         = "false"  # Note: Quoted boolean
    load_in_background = "true"   # Note: Quoted boolean
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

## Important Quirks

### Quoted Booleans in Views

Due to Terraform's type system, `embeddable` and `load_in_background` fields in application blocks within views MUST be quoted strings:
```hcl
application {
  embeddable = "false"  # Correct
  # embeddable = false  # WRONG - will cause errors
}
```

This quirk does NOT apply to top-level resources like `crucible_player_application_template`.

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
