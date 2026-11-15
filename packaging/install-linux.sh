#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPDIR="${HOME}/.local/share/chessgrowth"
mkdir -p "$APPDIR" "${HOME}/.local/share/applications" "${HOME}/.local/share/icons/hicolor/256x256/apps" "${HOME}/.local/bin"
cp "$ROOT/ChessGrowth" "$APPDIR/ChessGrowth"
chmod +x "$APPDIR/ChessGrowth"
cp "$ROOT/share/icons/hicolor/256x256/apps/chessgrowth.png" "${HOME}/.local/share/icons/hicolor/256x256/apps/chessgrowth.png"
cat > "${HOME}/.local/share/applications/chessgrowth.desktop" <<DESKTOP
[Desktop Entry]
Name=ChessGrowth
Comment=Grow and refine your opening repertoire
Exec=${APPDIR}/ChessGrowth
Icon=chessgrowth
Terminal=false
Type=Application
Categories=Game;Education;
DESKTOP
ln -sf "$APPDIR/ChessGrowth" "${HOME}/.local/bin/chessgrowth"
echo "Installed ChessGrowth for the current user. Start from your app launcher or ~/.local/bin/chessgrowth"
