ChessGrowth v1.0.0 — Standalone release
=====================================
macOS: Open ChessGrowth.app (move it to /Applications if preferred).
Windows: Double-click ChessGrowth.exe (or use the optional Setup.exe).
Linux: Run ./ChessGrowth or ./install-linux.sh for the desktop launcher.

The standalone application opens a browser-based local GUI at 127.0.0.1.
No Python, Go or Node.js installation is needed on the target machine.
Live repertoire generation needs Lichess Internet access. Stockfish is
required only for local or Cloud → Local engine evaluation.

Lichess personal API tokens are optional. Save a token through the settings UI;
macOS uses Keychain and Windows/Linux use per-user configuration storage.
Do not commit credentials or share installation-specific configuration files.

Website: https://github.com/MrPiada/ChessGrowth
Unsigned releases may trigger OS security warnings. Verify the source and
SHA-256 checksums before running. Installer signing/notarization is separate.
