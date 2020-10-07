#!/bin/bash

read -r -p "Are you sure you want to remove the log-cache CLI? [y/N]: " remove_cli
if [[ ! "$remove_cli" =~ ^[Yy]$ ]]; then
    exit 0
fi

# Remove the log-cache plugin from the CF CLI
cf uninstall-plugin log-cache
