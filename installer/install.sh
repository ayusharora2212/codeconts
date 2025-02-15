#!/bin/sh

# RUNS STANDALONE

pkg_bin_dir="$HOME/.codeconts/bin"
pkg_bin="$pkg_bin_dir/codeconts" # IMPORTANT: FILE PATH NOT TO BE CHANGED IN FUTURE

if [[ -n $VERSION ]]; then
    pkg_version=$VERSION
else
    pkg_version="$(curl --silent https://raw.githubusercontent.com/ayusharora2212/codeconts/main/version)"
fi

mkdir $HOME/.codeconts
mkdir $HOME/.codeconts/bin

os="$(echo "$(uname -s)" | tr '[:upper:]' '[:lower:]')"

arch="$(uname -m)"
if [[ "$arch" == "aarch64" || "$arch" == "arm64" ]]; then
    target_arch="arm64"
elif [[ "$arch" == "x86_64" || "$arch" == "amd64" ]]; then
    target_arch="amd64"
elif [[ "$arch" == "i386" || "$arch" == "i486" || "$arch" == "i586" || "$arch" == "i686" ]]; then
    target_arch="386"
else
    echo "Unsupported architecture: $arch"
    exit 1
fi

target_bin_name="codeconts-$os-$target_arch"

echo "Fetching '$target_bin_name' release (version: $pkg_version)"
echo ""

curl -# -f -L -o $pkg_bin https://github.com/ayusharora2212/codeconts/releases/download/$pkg_version/$target_bin_name
curl_exit_code=$?

if [ $curl_exit_code != 0 ]; then
    echo "Error: curl command failed with code ($curl_exit_code)"
    exit 1
fi

chmod +x $pkg_bin

echo "alias codeconts=$pkg_bin"

for file in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile" "$HOME/.zshenv"; do
    if [ -f "$file" ]; then
        if grep -q "alias codeconts=" "$file"; then
            echo "Alias 'codeconts' already exists in $file. Skipping."
        else
            printf "\nalias codeconts=%s\n" "$pkg_bin" >> "$file"
            echo "Appended alias to $file"
        fi
    fi
done

echo "Please restart terminal to run 'codeconts' globally."