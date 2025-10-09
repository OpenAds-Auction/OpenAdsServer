# GitHub Actions CI/CD Pipeline

This document describes the GitHub Actions workflow for the prebid-server project.

## Overview

The CI/CD pipeline consists of four main jobs:

1. **Test** - Runs Go tests, validation, and formatting checks
2. **Build** - Builds Docker image with cryptographic attestation
3. **Security Scan** - Scans the built image for vulnerabilities
4. **Release** - Creates GitHub releases with build artifacts

## Workflow Triggers

- **Push to main/staging/ttd-auction-server-main**: Runs test, build, security scan, and release jobs
- **Push to feature branches**: Runs test job only
- **Pull Request**: Runs test job only (build available on manual trigger)
- **Release creation**: Runs full pipeline including release job
- **Manual dispatch**: Allows manual triggering with environment selection

## Required Secrets

No additional secrets are required beyond the default `GITHUB_TOKEN` which is automatically provided by GitHub Actions.

## Required Permissions

The workflow requires the following permissions:
- `contents: read` - To checkout code
- `packages: write` - To push to GitHub Container Registry
- `security-events: write` - To upload security scan results
- `actions: read` - To download artifacts

## Build Attestation

The build process generates cryptographic attestation including:
- RSA public/private key pair
- Digital signature of build metadata
- JSON attestation file with all verification data

### Public Key Availability

**Public keys are made available to users in multiple ways:**

1. **GitHub Releases** - All attestation files including `build_key.pub` are attached to releases
2. **GitHub Actions Artifacts** - Public keys are uploaded as standalone artifacts with 365-day retention
3. **Verification Guide** - Each release includes a `VERIFICATION_GUIDE.md` with step-by-step verification instructions

### Verification Process

Users can verify build integrity by:
1. Downloading the public key from a GitHub release
2. Using the provided verification commands to check the signature
3. Confirming the payload matches the expected format: `<commit-hash>:<timestamp>:openads-server-build`

## Docker Images

Images are pushed to GitHub Container Registry at:
- `ghcr.io/{owner}/{repo}:{tag}`

Tags include:
- Branch names for feature branches
- Semantic version tags for releases
- `latest` for the default branch
- SHA-based tags for specific commits

## Security Scanning

The pipeline includes Trivy vulnerability scanning that:
- Scans the built Docker image
- Uploads results to GitHub Security tab
- Fails the pipeline if critical vulnerabilities are found

## Artifact Management

- Build attestation files are stored as artifacts for 30 days
- Old artifacts are automatically cleaned up
- Release artifacts are permanently stored with GitHub releases