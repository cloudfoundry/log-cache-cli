#!/bin/bash

owner=cloudfoundry
repo=log-cache-cli
platform="$(uname | tr 'A-Z' 'a-z')"

if [ "$platform" != "linux" ] && [ "$platform" != "darwin" ]; then
  echo "Error: Unable to detect platform. Installation script currently works \
        for Linux and OSX platforms."
  exit 1
fi

latest_release_id="$(
    curl -s "https://api.github.com/repos/$owner/$repo/releases/latest" | \
        python -c 'import sys, json; print(json.load(sys.stdin).get("id", ""))'
)"
assets_json="$(
    curl -s "https://api.github.com/repos/$owner/$repo/releases/$latest_release_id/assets"
)"
asset_id="$(
    echo "$assets_json " \
        | python -c 'import sys, json; print([x for x in json.load(sys.stdin) if "log-cache-'"$platform"'" in x.get("name", "")][0].get("id", ""))'
)"
asset_name="$(
    echo "$assets_json " \
        | python -c 'import sys, json; print([x for x in json.load(sys.stdin) if "log-cache-'"$platform"'" in x.get("name", "")][0].get("name", ""))'
)"

echo "Downloading $asset_name from $owner/$repo..."
curl -sL "https://api.github.com/repos/$owner/$repo/releases/assets/$asset_id" \
    -H "Accept: application/octet-stream" -o "$asset_name"

chmod +x "$asset_name"
# test if we can write to destintaion
(if >> /usr/local/bin/lc; then
    mv "$asset_name" /usr/local/bin/lc
else
    sudo mv "$asset_name" /usr/local/bin/lc
fi) > /dev/null 2>&1

read -r -p 'Did you want to create the shortcut "log-cache" for the "lc" CLI? [y/N]: ' create_shortcut
if [[ "$create_shortcut" =~ ^[Yy]$ ]]; then
    # test if we can write to destintaion
    (if >> /usr/local/bin/log-cache; then
        rm -f /usr/local/bin/log-cache
        ln -s /usr/local/bin/lc /usr/local/bin/log-cache
    else
        sudo ln -s /usr/local/bin/lc /usr/local/bin/log-cache
    fi) > /dev/null 2>&1
fi
