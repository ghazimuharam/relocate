#!/bin/bash

set -e

echo "Installing relocate..."

# Install with go install
go install github.com/ghazimuharam/relocate/cmd/relocate@latest

# Create config directory
mkdir -p ~/.relocate

# Copy example config if it doesn't exist
if [ ! -f ~/.relocate/config.json ]; then
    cp config.example.json ~/.relocate/config.json
    echo "Config file created at ~/.relocate/config.json"
    echo "Please edit it with your SSH keys and AWS settings."
else
    echo "Config file already exists at ~/.relocate/config.json"
fi

echo "Installation complete!"
echo "Run 'relocate' to start."
