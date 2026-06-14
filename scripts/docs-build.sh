#!/bin/bash
# Anjungan docs build script
# Copies markdown docs to the site output directory for CF Pages deployment
# Usage: ./scripts/docs-build.sh

set -e
mkdir -p site/docs
cp docs/*.md site/docs/
echo "Docs copied to site/docs/"
