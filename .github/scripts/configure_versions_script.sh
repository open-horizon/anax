#!/bin/bash

# Get current versions
ORIG_ANAX_IMAGE_VERSION=$(sed -n 's/export VERSION ?= //p' Makefile | cut -d '$' -f 1 | sed 's/ *$//g')
ORIG_CSS_IMAGE_VERSION=$(sed -n 's/CSS_IMAGE_VERSION ?= //p' Makefile | cut -d '$' -f 1 | sed 's/ *$//g')
ORIG_ESS_IMAGE_VERSION=$(sed -n 's/ESS_IMAGE_VERSION ?= //p' Makefile | cut -d '$' -f 1 | sed 's/ *$//g')

# Append our build numbers from the workflow env. variables
ANAX_IMAGE_VERSION="${ORIG_ANAX_IMAGE_VERSION}-${BUILD_NUMBER}"
CSS_IMAGE_VERSION="${ORIG_CSS_IMAGE_VERSION}-${BUILD_NUMBER}"
ESS_IMAGE_VERSION="${ORIG_ESS_IMAGE_VERSION}-${BUILD_NUMBER}"

# Unique version hashing not needed as of now
# # //now Anax things and ESS/CSS will have same date/timestamp hash
# UNIQUE_VERSION_HASH=$(git log -n 1 --pretty=format:'%h')
# ANAX_IMAGE_VERSION="${ORIG_ANAX_IMAGE_VERSION}-${UNIQUE_VERSION_HASH}"
# CSS_IMAGE_VERSION="${ORIG_CSS_IMAGE_VERSION}-${UNIQUE_VERSION_HASH}"
# ESS_IMAGE_VERSION="${ORIG_ESS_IMAGE_VERSION}-${UNIQUE_VERSION_HASH}"

# Output configured versions for viewing
echo "***Anax Version, No Build Number: ${ORIG_ANAX_IMAGE_VERSION}"
echo "***Anax Version: ${ANAX_IMAGE_VERSION}"
echo "***CSS Version: ${CSS_IMAGE_VERSION}"
echo "***ESS Version: ${ESS_IMAGE_VERSION}"

# Put script variables into workflow env. variables
echo "VERSION_NO_BUILD_NUMBER=$ORIG_ANAX_IMAGE_VERSION" >> "$GITHUB_OUTPUT"
echo "ANAX_IMAGE_VERSION=$ANAX_IMAGE_VERSION" >> "$GITHUB_OUTPUT"
echo "CSS_IMAGE_VERSION=$CSS_IMAGE_VERSION" >> "$GITHUB_OUTPUT"
echo "ESS_IMAGE_VERSION=$ESS_IMAGE_VERSION" >> "$GITHUB_OUTPUT"