#!/bin/sh
# Build dist/wasaping.app (Apple Silicon + Intel) and dist/wasaping-macos.zip.
# Works from any OS: Go cross-compiles, and the arm64 binary is ad-hoc signed by the Go linker.
set -e
VERSION="${VERSION:-dev}"
APP=dist/wasaping.app
rm -rf "$APP" && mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

for arch in arm64 amd64; do
  name=$arch; [ "$arch" = amd64 ] && name=x86_64
  CGO_ENABLED=0 GOOS=darwin GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$VERSION" -o "$APP/Contents/MacOS/wasaping-$name" .
done

cat > "$APP/Contents/MacOS/wasaping" <<'SH'
#!/bin/sh
# Launcher: picks the binary for this Mac and opens the interface in the browser.
DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$DIR/wasaping-$(uname -m)" app
SH
chmod +x "$APP/Contents/MacOS/"*
cp assets/wasaping.icns "$APP/Contents/Resources/"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>wasaping</string>
  <key>CFBundleDisplayName</key><string>wasaping</string>
  <key>CFBundleIdentifier</key><string>eu.wasabi.wasaping</string>
  <key>CFBundleVersion</key><string>${VERSION#v}</string>
  <key>CFBundleShortVersionString</key><string>${VERSION#v}</string>
  <key>CFBundleExecutable</key><string>wasaping</string>
  <key>CFBundleIconFile</key><string>wasaping</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>LSUIElement</key><true/>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

# zip keeping the executable bits
python3 - "$APP" dist/wasaping-macos.zip <<'PY'
import os, sys, zipfile
src, out = sys.argv[1], sys.argv[2]
base = os.path.dirname(src)
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
    for root, dirs, files in os.walk(src):
        for f in sorted(files):
            p = os.path.join(root, f)
            import time; info = zipfile.ZipInfo(os.path.relpath(p, base), time.localtime(os.stat(p).st_mtime)[:6])
            info.external_attr = (os.stat(p).st_mode & 0xFFFF) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            with open(p, "rb") as fh:
                z.writestr(info, fh.read())
PY
echo "→ $APP e dist/wasaping-macos.zip"
