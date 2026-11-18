#!/bin/sh
# Build dist/wasaping.app (universal: Apple Silicon + Intel) and dist/wasaping-macos.zip.
# Needs macOS with the Xcode command line tools: the app window uses Cocoa and
# WebKit through cgo, so it cannot be cross-compiled from Linux or Windows.
set -e
if [ "$(uname -s)" != Darwin ]; then
  echo "build_macapp.sh va eseguito su macOS (serve clang per la finestra nativa)" >&2
  exit 1
fi
VERSION="${VERSION:-dev}"
APP=dist/wasaping.app
TMP=dist/.macapp
rm -rf "$APP" "$TMP" && mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" "$TMP"

for arch in arm64 amd64; do
  carch=$arch; [ "$arch" = amd64 ] && carch=x86_64
  CGO_ENABLED=1 GOOS=darwin GOARCH=$arch \
    CC="clang -arch $carch" \
    CGO_CFLAGS="-mmacosx-version-min=13.0" CGO_LDFLAGS="-mmacosx-version-min=13.0" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$TMP/wasaping-$arch" .
done
lipo -create -output "$APP/Contents/MacOS/wasaping" "$TMP/wasaping-arm64" "$TMP/wasaping-amd64"
rm -rf "$TMP"
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
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSHumanReadableCopyright</key><string>a spicy little IP scanner</string>
  <key>NSLocalNetworkUsageDescription</key><string>wasaping cerca i dispositivi sulla tua rete locale.</string>
  <key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict>
</dict>
</plist>
PLIST

# Ad-hoc signature for the whole bundle (no Apple Developer ID yet).
codesign --force --sign - "$APP"
rm -f dist/wasaping-macos.zip
ditto -c -k --keepParent "$APP" dist/wasaping-macos.zip
echo "→ $APP e dist/wasaping-macos.zip"
