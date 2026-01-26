# Documentation Publishing Workflow for the anax repository

> **Last Updated:** January 2026

## Overview

This document describes how documentation from the anax repository is automatically published to the Open Horizon website.

## Source Locations

- **docs/** folder → Published to: https://open-horizon.github.io/docs/anax/docs/
- **agent-install/README.md** → Published to: https://open-horizon.github.io/docs/anax/docs/overview/

## Automation Process

The documentation publishing workflow follows these steps:

1. Changes are pushed to the master branch in the anax repository
2. GitHub Actions trigger automatically:
   - `copyagentreadme.yml` - Copies agent-install documentation
   - `copydocs.yml` - Copies docs folder content
3. Files are copied to the open-horizon.github.io repository
4. A rebuild is triggered in the open-horizon.github.io repository
5. Updated documentation is published to the website

## GitHub Actions

The following GitHub Actions automate the documentation publishing process:

- [copyagentreadme.yml](../.github/workflows/copyagentreadme.yml)
- [copydocs.yml](../.github/workflows/copydocs.yml)

## Troubleshooting

If documentation updates don't appear on the website:

- Check GitHub Actions status in both repositories (anax and open-horizon.github.io)
- Verify changes were merged to the master branch
- Allow 5-10 minutes for the full publishing pipeline to complete
- Review the GitHub Actions logs for any error messages