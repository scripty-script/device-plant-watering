#!/bin/bash

# Define variables
APP_NAME="pws"
GITHUB_REPO="scripty_script/device-plant-watering"
INSTALL_DIR="/usr/local/bin"

# Download the latest release from GitHub
wget -q https://github.com/$GITHUB_REPO/releases/latest/download/$APP_NAME -O $INSTALL_DIR/$APP_NAME

# Set executable permissions
chmod +x $INSTALL_DIR/$APP_NAME

sudo mkdir ~/.pws
sudo chmod 777 ~/.pws

sudo touch ~/.pws/service.log

# Display installation success message
echo "$APP_NAME installed successfully!"