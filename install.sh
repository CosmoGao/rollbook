#!/bin/sh
set -eu

repo="https://github.com/CosmoGao/rollbook/releases"
version="${ROLLBOOK_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$repo/latest" | sed 's@.*/@@')
fi
if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
  echo "无效的版本号: $version" >&2
  exit 1
fi
case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo '仅支持 Linux 和 macOS；Windows 请下载 release 中的压缩包。' >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo '不支持的处理器架构。' >&2; exit 1 ;;
esac
name="rollbook_${version}_${os}_${arch}.tar.gz"
base="$repo/download/$version"
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT HUP INT TERM
curl -fsSL "$base/$name" -o "$dir/$name"
curl -fsSL "$base/SHA256SUMS" -o "$dir/SHA256SUMS"
expected=$(grep "  $name\$" "$dir/SHA256SUMS" | cut -d ' ' -f 1)
if [ -z "$expected" ]; then echo '校验和清单缺少安装包。' >&2; exit 1; fi
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$dir/$name" | cut -d ' ' -f 1)
else
  actual=$(shasum -a 256 "$dir/$name" | cut -d ' ' -f 1)
fi
if [ "$actual" != "$expected" ]; then echo 'SHA256 校验失败。' >&2; exit 1; fi
tar -C "$dir" -xzf "$dir/$name"
dest="${ROLLBOOK_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$dest"
install -m 0755 "$dir/rollbook" "$dest/rollbook"
echo "已安装 $version 到 $dest/rollbook"
