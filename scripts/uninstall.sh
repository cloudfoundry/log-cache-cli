#!/bin/bash

read -r -p "Are you sure you want to remove the log-cache CLI? [y/N]: " remove_cli
if [[ ! "$remove_cli" =~ ^[Yy]$ ]]; then
    exit 0
fi

rm -f /usr/local/bin/lc
rm -f /usr/local/bin/log-cache
