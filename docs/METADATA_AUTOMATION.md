# Automated Metadata Management

This document explains the automated workflows for managing ArtifactHub and OLM bundle metadata in the Akamai Operator project.

## Overview

Three workflows have been implemented to automate metadata management:

1. **[CI/CD Pipeline](/.github/workflows/ci-cd.yml)** - Automatic updates on tag releases
2. **[Update Metadata](/.github/workflows/update-metadata.yml)** - Manual on-demand updates
3. **[Validate Metadata](/.github/workflows/validate-metadata.yml)** - Continuous validation

## Automatic Updates (CI/CD Pipeline)

### Trigger
- Automatically runs when you push a new version tag (e.g., `v0.4.0`)

### What it does
- Extracts version from the git tag
- Updates `artifacthub-pkg.yml` with new version and container image
- Updates bundle metadata in `bundle/metadata/annotations.yaml`
- Updates ClusterServiceVersion in `bundle/manifests/akamai-operator.clusterserviceversion.yaml`
- Commits and pushes changes back to the main branch
- Builds and publishes the OLM bundle

### Usage
```bash
# Create and push a new version tag
git tag v0.4.0
git push --tags

# The workflow will automatically:
# 1. Update all metadata files to version 0.4.0
# 2. Update container image to ghcr.io/mmz-srf/akamai-operator:v0.4.0
# 3. Update installation commands
# 4. Build and push bundle
# 5. Create GitHub release
```

### Files Updated Automatically
- `artifacthub-pkg.yml` - version, appVersion, containersImages, install command
- `bundle/metadata/annotations.yaml` - operator-sdk version, project layout
- `bundle/manifests/akamai-operator.clusterserviceversion.yaml` - name, version, container images

## Manual Updates (Update Metadata Workflow)

### Trigger
- Manual execution via GitHub Actions UI
- Navigate to Actions → Update Metadata → Run workflow

### What it does
- Updates metadata to a specified version
- Creates a Pull Request with the changes for review
- Validates version format (must be semver: X.Y.Z)
- Optionally updates timestamps

### Usage
1. Go to GitHub Actions in your repository
2. Select "Update Metadata" workflow
3. Click "Run workflow"
4. Enter the desired version (e.g., `0.4.0`)
5. Choose whether to update timestamps
6. Review and merge the created Pull Request

### Example
- Input version: `0.4.0`
- Creates PR: "chore: update metadata to version 0.4.0"
- Updates all version references to `0.4.0`
- Updates container image to `ghcr.io/mmz-srf/akamai-operator:v0.4.0`

## Validation (Validate Metadata Workflow)

### Trigger
- Automatically runs on:
  - Push to main branch (when metadata files change)
  - Pull requests (when metadata files change)

### What it validates
- **ArtifactHub metadata structure**:
  - Required fields (name, version, description, etc.)
  - Version format (semver)
  - Container image format
  - Installation command format

- **Bundle structure**:
  - Directory structure (`bundle/manifests`, `bundle/metadata`)
  - Required annotations
  - ClusterServiceVersion format

- **Version consistency**:
  - All version fields match across files
  - Container image versions match package version

### Example Output
```
✅ Field 'name': akamai-operator
✅ Field 'version': 0.3.0
✅ Container image 0: akamai-operator -> ghcr.io/mmz-srf/akamai-operator:v0.3.0
✅ Install command format is valid
✅ Bundle structure is valid
✅ CSV validation passed
✅ All versions are consistent: 0.3.0
```

## Files Managed

### artifacthub-pkg.yml
```yaml
version: "0.3.0"                    # ← Updated automatically
appVersion: "0.3.0"                 # ← Updated automatically
containersImages:
  - name: akamai-operator
    image: ghcr.io/mmz-srf/akamai-operator:v0.3.0  # ← Updated automatically
install: kubectl apply -f https://github.com/mmz-srf/akamai-operator/releases/download/v0.3.0/akamai-operator.yaml  # ← Updated automatically
```

### bundle/metadata/annotations.yaml
```yaml
annotations:
  operators.operatorframework.io/metrics.builder: operator-sdk-v1.34.1        # ← Updated automatically
  operators.operatorframework.io/metrics.project_layout: go.kubebuilder.io/v4  # ← Updated automatically
```

### bundle/manifests/akamai-operator.clusterserviceversion.yaml
```yaml
metadata:
  name: akamai-operator.v0.3.0      # ← Updated automatically
spec:
  version: 0.3.0                    # ← Updated automatically
```

## Best Practices

### For Release Management
1. **Use semantic versioning** (X.Y.Z format)
2. **Tag releases consistently** (always prefix with 'v')
3. **Let automation handle metadata** - don't manually edit version files
4. **Review generated PRs** from manual updates before merging

### For Development
1. **Run validation locally** before pushing changes
2. **Check consistency** across all metadata files
3. **Update timestamps** only when making significant changes
4. **Test installation commands** after updates

### For ArtifactHub Submission
1. **Ensure all validations pass** before submitting to ArtifactHub
2. **Use latest release** for submission
3. **Verify container images** are publicly accessible
4. **Test installation instructions** are working

## Troubleshooting

### Common Issues

**Version Mismatch Errors**
```bash
❌ Error: ArtifactHub version (0.3.0) doesn't match CSV version (0.2.0)
```
**Solution**: Run the "Update Metadata" workflow to synchronize versions.

**Invalid Version Format**
```bash
❌ Error: Version 'v0.3.0' is not in valid semver format (X.Y.Z)
```
**Solution**: Use version without 'v' prefix (e.g., `0.3.0` not `v0.3.0`).

**Missing Required Fields**
```bash
❌ Error: Required field 'name' is missing or empty
```
**Solution**: Check the `artifacthub-pkg.yml` file has all required fields.

### Manual Recovery
If automation fails, you can manually update files:

```bash
# Update version in artifacthub-pkg.yml
yq eval ".version = \"0.4.0\"" -i artifacthub-pkg.yml
yq eval ".appVersion = \"0.4.0\"" -i artifacthub-pkg.yml

# Update CSV version
yq eval ".spec.version = \"0.4.0\"" -i bundle/manifests/akamai-operator.clusterserviceversion.yaml

# Validate changes
.github/workflows/validate-metadata.yml
```

## Integration with ArtifactHub

The automated metadata ensures:
- ✅ **Correct package naming** for ArtifactHub discovery
- ✅ **Version consistency** across all manifests
- ✅ **Valid installation instructions** for end users
- ✅ **Proper container image references** for deployments
- ✅ **Up-to-date bundle information** for OLM integration

This automation eliminates the manual effort and potential errors when submitting operators to ArtifactHub, ensuring all metadata is always synchronized and valid.