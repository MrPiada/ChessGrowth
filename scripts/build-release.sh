#!/usr/bin/env bash
# Cross-compile ChessGrowth from macOS (also works from Linux).
# macOS creates DMGs; optional makensis creates a Windows installer on the Mac.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
VERSION="${1:-v1.3.3}"
DIST="${DIST_DIR:-${ROOT}/dist}"
mkdir -p "$DIST"
python3 scripts/build_windows_icon.py
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
for target in darwin/arm64 darwin/amd64 windows/amd64 linux/amd64 linux/arm64;do
 OS="${target%/*}";ARCH="${target#*/}"
 NAME="ChessGrowth-${VERSION}-${OS}-${ARCH}"
 DIR="${STAGE}/${NAME}";mkdir -p "$DIR"
 EXE=ChessGrowth;if [[ "$OS" == windows ]];then EXE=ChessGrowth.exe;fi
 echo "==> Compiling ${target}"
 LDFLAGS="-s -w -X main.version=${VERSION#v}"
 if [[ "$OS" == windows ]];then LDFLAGS="$LDFLAGS -H=windowsgui";fi
 CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="$LDFLAGS" -o "$DIR/$EXE" .
 cp LICENSE "$DIR/LICENSE";cp packaging/PORTABLE_README.txt "$DIR/README.txt"
 if [[ "$OS" == darwin ]];then
  APP="$DIR/ChessGrowth.app"
  mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
  mv "$DIR/$EXE" "$APP/Contents/MacOS/ChessGrowth"
  APP_VERSION="${VERSION#v}";APP_VERSION="${APP_VERSION%%-*}"
  sed "s/__VERSION__/$APP_VERSION/g" packaging/Info.plist > "$APP/Contents/Info.plist"
  cp packaging/ChessGrowth.icns "$APP/Contents/Resources/ChessGrowth.icns"
  cp LICENSE "$APP/Contents/Resources/LICENSE"
  if [[ "$(uname -s)" == Darwin ]] && command -v codesign >/dev/null 2>&1;then
   codesign --force --sign - "$APP" >/dev/null 2>&1 || true
  fi
  if [[ "$(uname -s)" == Darwin ]]&&command -v ditto >/dev/null 2>&1;then
   (cd "$DIR"&&ditto -c -k --sequesterRsrc --keepParent ChessGrowth.app "$DIST/${NAME}.zip")
  else (cd "$DIR"&&zip -qr "$DIST/${NAME}.zip" ChessGrowth.app LICENSE README.txt);fi
  if [[ "$(uname -s)" == Darwin ]]&&command -v hdiutil >/dev/null 2>&1;then
   hdiutil create -quiet -volname "ChessGrowth" -srcfolder "$APP" -ov -format UDZO "$DIST/${NAME}.dmg"
  fi
 elif [[ "$OS" == windows ]];then
  (cd "$DIR"&&zip -qr "$DIST/${NAME}.zip" "$EXE" LICENSE README.txt)
  if command -v makensis >/dev/null 2>&1;then
   makensis -V2 -DINPUT_EXE="$DIR/$EXE" -DOUTFILE="$DIST/${NAME}-Setup.exe" -DLICENSE_PATH="$ROOT/LICENSE" -DICON_PATH="$ROOT/packaging/ChessGrowth.ico" packaging/WindowsInstaller.nsi
  fi
 else
  mkdir -p "$DIR/share/icons/hicolor/256x256/apps"
  cp packaging/windows_icon/icon_256.png "$DIR/share/icons/hicolor/256x256/apps/chessgrowth.png"
  cp packaging/ChessGrowth.desktop "$DIR/ChessGrowth.desktop"
  cp packaging/install-linux.sh "$DIR/install-linux.sh"
  chmod +x "$DIR/install-linux.sh"
  (cd "$STAGE"&&tar czf "$DIST/${NAME}.tar.gz" "$NAME")
  python3 scripts/build_deb.py --binary "$DIR/$EXE" --arch "$ARCH" --version "$VERSION" --output "$DIST/${NAME}.deb"
 fi
done
(cd "$DIST" && shasum -a 256 ChessGrowth-"$VERSION"-* > SHA256SUMS.txt)
echo "Ready: $DIST";ls -lh "$DIST"/*
