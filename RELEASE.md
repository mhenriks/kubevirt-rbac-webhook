# Release Guide

This guide is for maintainers creating new releases of the KubeVirt RBAC Webhook.

## Release Process

### Prerequisites

- Push access to the repository
- Permission to create releases
- Permission to push to `ghcr.io/mhenriks/kubevirt-rbac-webhook`

### Creating a Release

The release process is fully automated via GitHub Actions. Simply create and push a version tag:

```bash
# 1. Ensure you're on the main branch and up to date
git checkout main
git pull origin main

# 2. Create a version tag (use semantic versioning)
git tag v0.1.0

# 3. Push the tag to trigger the release workflow
git push origin v0.1.0
```

### What Happens Automatically

When you push a version tag (e.g., `v0.1.0`), the GitHub Actions workflow will:

1. **Build multi-platform container images** for:
   - linux/amd64
   - linux/arm64
   - linux/s390x
   - linux/ppc64le

2. **Push container images** to GitHub Container Registry:
   - `ghcr.io/mhenriks/kubevirt-rbac-webhook:v0.1.0` (tagged version)
   - `ghcr.io/mhenriks/kubevirt-rbac-webhook:latest` (latest version)

3. **Generate installation manifest**:
   - Creates `dist/install.yaml` with all required resources
   - Includes webhook, ClusterRoles, RBAC, certificates, etc.

4. **Create GitHub Release**:
   - Attaches `install.yaml` to the release
   - Generates release notes with installation instructions
   - Marks the release as published (not draft)

### Verifying the Release

After the workflow completes:

1. **Check GitHub Releases**: https://github.com/mhenriks/kubevirt-rbac-webhook/releases
   - Verify the release was created
   - Download and inspect `install.yaml`

2. **Check Container Images**:
   ```bash
   docker pull ghcr.io/mhenriks/kubevirt-rbac-webhook:v0.1.0
   ```

3. **Test Installation** (on a test cluster):
   ```bash
   # Install cert-manager first
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=60s

   # Install the webhook
   kubectl apply -f https://github.com/mhenriks/kubevirt-rbac-webhook/releases/download/v0.1.0/install.yaml

   # Verify deployment
   kubectl get pods -n kubevirt-rbac-webhook-system
   kubectl get clusterroles | grep kubevirt.io:vm-
   ```

4. **Test Functionality**:
   - Create a test VM
   - Grant fine-grained permissions to a user
   - Verify webhook enforcement works

### Release Checklist

Before creating a release, ensure:

- [ ] All tests pass on main branch
- [ ] CHANGELOG.md is updated (if you maintain one)
- [ ] README.md reflects current features
- [ ] Version number follows semantic versioning
- [ ] Breaking changes are documented

### Versioning Guidelines

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (v2.0.0): Incompatible API changes
- **MINOR** (v0.2.0): New features (backwards compatible)
- **PATCH** (v0.1.1): Bug fixes (backwards compatible)

Examples:
- `v0.1.0` - Initial release
- `v0.1.1` - Bug fix release
- `v0.2.0` - Add new ClusterRole
- `v1.0.0` - Production-ready, stable API

### Troubleshooting

#### Release Workflow Failed

1. Check the workflow logs: https://github.com/mhenriks/kubevirt-rbac-webhook/actions
2. Common issues:
   - **Docker buildx fails**: Check platform compatibility
   - **Permission denied**: Verify GHCR_TOKEN has write permissions
   - **Kustomize errors**: Test `make build-installer` locally

#### Image Push Failed

Verify permissions:
```bash
# Log in manually to test
echo $GITHUB_TOKEN | docker login ghcr.io -u mhenriks --password-stdin
```

#### Install Manifest Issues

Test locally:
```bash
make build-installer IMG=ghcr.io/mhenriks/kubevirt-rbac-webhook:v0.1.0
kubectl apply --dry-run=server -f dist/install.yaml
```

### Rollback a Release

If you need to delete a bad release:

1. **Delete the GitHub release** (via web UI or API)
2. **Delete the tag**:
   ```bash
   # Delete remote tag
   git push --delete origin v0.1.0

   # Delete local tag
   git tag -d v0.1.0
   ```
3. **Fix the issue** and create a new patch release

### Post-Release Tasks

After a successful release:

1. **Announce the release** (if applicable):
   - Update project documentation
   - Post to relevant communities/forums
   - Update version references in examples

2. **Monitor for issues**:
   - Watch GitHub Issues for bug reports
   - Check container image pull metrics
   - Review installation feedback

## Release Workflow Details

The release workflow is defined in `.github/workflows/release.yml`:

- **Trigger**: Push of tags matching `v*`
- **Permissions**: `contents: write` (for releases), `packages: write` (for images)
- **Steps**:
  1. Checkout code
  2. Setup Go
  3. Setup Docker Buildx
  4. Login to GHCR
  5. Build/push multi-platform images
  6. Generate install.yaml
  7. Create GitHub release with artifacts

## Container Registry

Images are published to GitHub Container Registry (GHCR):

- **Registry**: `ghcr.io`
- **Repository**: `ghcr.io/mhenriks/kubevirt-rbac-webhook`
- **Public Access**: Images are public and can be pulled without authentication
- **Platforms**: Multi-platform support for broad Kubernetes cluster compatibility

## Support

For questions about the release process:
- Open an issue: https://github.com/mhenriks/kubevirt-rbac-webhook/issues
- Contact maintainers: @mhenriks
