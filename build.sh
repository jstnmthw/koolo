#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status

echo "Start building Koolo"
echo "Cleaning up previous artifacts..."
if [ -d build ]; then
    rm -rf build || { echo "Error occurred during cleanup."; exit 1; }
fi

echo "Building Koolo binary..."
VERSION=${1:-dev}  # Set VERSION to the first argument, default to 'dev' if not provided
GOOS=windows GOARCH=amd64 go build -trimpath -tags static \
    --ldflags "-extldflags=-static -s -w -H windowsgui -X 'github.com/hectorgimenez/koolo/internal/config.Version=$VERSION'" \
    -o build/koolo.exe ./cmd/koolo || { echo "Error occurred during build."; exit 1; }

echo "Copying assets..."
mkdir -p build/config || { echo "Error occurred while creating config directory."; exit 1; }
cp config/koolo.yaml.dist build/config/koolo.yaml || { echo "Error occurred copying koolo.yaml."; exit 1; }
cp config/Settings.json build/config/Settings.json || { echo "Error occurred copying Settings.json."; exit 1; }
cp -r config/template build/config/template || { echo "Error occurred copying templates."; exit 1; }
cp -r tools build/tools || { echo "Error occurred copying tools."; exit 1; }
cp README.md build/ || { echo "Error occurred copying README.md."; exit 1; }

echo "Done! Artifacts are in the build directory."