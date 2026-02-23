# Migration Guide: v0.x to v1.0.0

This guide helps you migrate from the legacy SDK v1-based provider (v0.9.x and earlier) to the new Plugin Framework-based provider (v1.0.0+).

## Breaking Changes

### 1. Boolean Fields in View Applications

**The Issue:** In v0.x, the `embeddable` and `load_in_background` fields within `application` blocks inside `crucible_player_view` resources were incorrectly typed as strings due to a Terraform SDK limitation.

**Old (v0.x):**
```hcl
resource "crucible_player_view" "example" {
  name = "My View"

  application {
    name               = "testApp"
    embeddable         = "false"  # ❌ Quoted string (incorrect)
    load_in_background = "true"   # ❌ Quoted string (incorrect)
  }
}
```

**New (v1.0.0):**
```hcl
resource "crucible_player_view" "example" {
  name = "My View"

  application {
    name               = "testApp"
    embeddable         = false  # ✅ Proper boolean
    load_in_background = true   # ✅ Proper boolean
  }
}
```

**Action Required:**
- Update all `application` blocks in your `.tf` files to use proper booleans (no quotes)
- This change ONLY affects `application` blocks within `crucible_player_view` resources
- The `crucible_player_application_template` resource already used proper booleans and is unchanged

### 2. Environment Variable Rename

**Old:** `SEI_CRUCIBLE_TOK_URL`
**New:** `SEI_CRUCIBLE_TOKEN_URL`

**Backward Compatibility:** v1.0.0 checks both names for compatibility. The old `SEI_CRUCIBLE_TOK_URL` will be supported for several releases but is deprecated.

**Action Required:**
- Update your environment variable name to `SEI_CRUCIBLE_TOKEN_URL`
- Update any scripts, CI/CD pipelines, or documentation referencing the old name

### 3. Improved Error Messages

**What Changed:** Error messages now include detailed API response information instead of just HTTP status codes.

**Old Error:**
```
Error: Player API returned with status code 400 when creating view
```

**New Error:**
```
Error: Could not create view 'My View': API returned status 400: Invalid request:
missing required field 'name' (body: {"message":"Invalid request: missing required
field 'name'"})
```

**Action Required:** None - this is an improvement that provides better debugging information.

### 4. Token Caching

**What Changed:** OAuth2 tokens are now cached and automatically refreshed, reducing API calls to the identity provider.

**Action Required:** None - this is a performance improvement that is transparent to users.

## State Migration

The Plugin Framework handles state migration automatically. Your existing Terraform state from v0.9.x will be upgraded seamlessly.

**Safety Steps:**

1. **Backup your state:**
   ```bash
   terraform state pull > backup-$(date +%Y%m%d).tfstate
   ```

2. **Update provider version in your configuration:**
   ```hcl
   terraform {
     required_providers {
       crucible = {
         source  = "registry.terraform.io/cmu-sei/crucible"
         version = "~> 1.0.0"
       }
     }
   }
   ```

3. **Update boolean syntax in view application blocks** (see Breaking Change #1 above)

4. **Initialize the new provider:**
   ```bash
   terraform init -upgrade
   ```

5. **Verify the plan shows expected changes:**
   ```bash
   terraform plan
   ```

   You should see minimal or no changes if you updated the boolean syntax correctly.

6. **Apply if needed:**
   ```bash
   terraform apply
   ```

## Import Command

All resources now support the `terraform import` command, allowing you to import existing Crucible resources into Terraform state.

**Syntax:**
```bash
terraform import <resource_type>.<name> <resource_id>
```

**Examples:**

```bash
# Import a user
terraform import crucible_player_user.admin 550e8400-e29b-41d4-a716-446655440000

# Import a view
terraform import crucible_player_view.training abc12345-6789-0def-ghij-klmnopqrstuv

# Import a virtual machine
terraform import crucible_player_virtual_machine.win10 def45678-90ab-cdef-1234-567890abcdef

# Import an application template
terraform import crucible_player_application_template.webapp xyz98765-4321-fedc-ba09-876543210fed

# Import a VLAN
terraform import crucible_vlan.network1 vlan12345-6789-0123-4567-890abcdef012
```

After importing, run `terraform plan` to see if any changes are needed to match your desired configuration.

## Rollback Procedure

If you encounter issues with v1.0.0, you can rollback to v0.9.0:

1. **Pin to v0.9.0 in your configuration:**
   ```hcl
   terraform {
     required_providers {
       crucible = {
         source  = "registry.terraform.io/cmu-sei/crucible"
         version = "~> 0.9.0"
       }
     }
   }
   ```

2. **Restore your state backup (if you applied v1.0.0 changes):**
   ```bash
   terraform state push backup-<date>.tfstate
   ```

3. **Reinitialize:**
   ```bash
   terraform init -upgrade
   ```

4. **Revert boolean changes in your .tf files** (back to quoted strings)

## What's New in v1.0.0

### Type Safety

All resources now use type-safe models with proper Go types instead of `interface{}`:
- Booleans are `bool` (not `interface{}` or `string`)
- Optional fields use proper null handling
- Lists and nested objects are type-safe

### Better Validation

- UUID fields validated with regex
- URL fields validated for HTTP/HTTPS
- Enum fields validated (e.g., protocol must be ssh/vnc/rdp)
- ConflictsWith validators for mutually exclusive fields

### Rich Error Messages

Errors now include:
- Context about which resource and operation failed
- Detailed API error messages extracted from response bodies
- Suggestions for common issues (e.g., "Verify your credentials are correct")

### Performance Improvements

- OAuth2 tokens cached with automatic refresh (reduces auth API calls)
- Eliminated 940+ lines of duplicated HTTP code
- More efficient nested attribute handling

### Code Quality

- 1,345 fewer lines of code (26.5% reduction)
- 95% type-safe (up from ~30%)
- Comprehensive unit tests
- Improved code maintainability

## Testing Your Migration

After migrating, test your configuration:

1. **Plan without changes:**
   ```bash
   terraform plan
   ```
   Should show no changes if configuration matches remote state.

2. **Test a small change:**
   Make a minor change (e.g., update a view description) and apply:
   ```bash
   terraform apply
   ```

3. **Verify remote state:**
   Log into your Crucible environment and verify resources match expectations.

## Common Migration Issues

### Issue: "Error: Unsupported argument" for embeddable/load_in_background

**Cause:** You're using quoted strings for boolean fields in view applications.

**Solution:** Remove quotes:
```hcl
# Wrong
embeddable = "false"

# Correct
embeddable = false
```

### Issue: "Authentication Failed" on provider configure

**Cause:** Token URL environment variable not set or incorrect.

**Solution:** Verify environment variables:
```bash
echo $SEI_CRUCIBLE_TOKEN_URL
echo $SEI_CRUCIBLE_USERNAME
```

Ensure token URL points to your identity provider's token endpoint (e.g., `https://identity.crucible.dev/connect/token`).

### Issue: "no membership found" when creating view with users

**Cause:** User IDs in your configuration don't exist in the identity provider.

**Solution:** Verify user IDs exist:
- Check identity provider (Keycloak) for user accounts
- Ensure user_id matches the UUID from the identity provider
- Create users in identity provider first if needed

### Issue: terraform plan shows unexpected changes after migration

**Cause:** State drift or configuration mismatch.

**Solution:**
1. Run `terraform refresh` to sync state
2. Check for alphabetical ordering of teams/apps/users (now automatic)
3. Verify boolean syntax is correct
4. Check diff to see what Terraform thinks changed

## Getting Help

If you encounter issues during migration:

1. **Check error messages carefully** - v1.0.0 provides detailed diagnostic information
2. **Verify configuration syntax** - especially boolean fields
3. **Review state file** - compare v0.9.0 vs v1.0.0 state structure
4. **Open an issue:** https://github.com/cmu-sei/crucible/issues

Include:
- Terraform version (`terraform version`)
- Provider version
- Relevant .tf file snippets (redact sensitive values)
- Full error messages
- Steps to reproduce

## Version Compatibility

| Provider Version | Terraform Version | Go Version |
|------------------|-------------------|------------|
| v0.9.x (SDK v1)  | 0.12 - 1.x        | 1.23+      |
| v1.0.0+ (Framework) | 1.0 - 1.x      | 1.21+      |

**Note:** v1.0.0 requires Terraform 1.0 or later due to Plugin Framework requirements.
