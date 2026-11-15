# ChessGrowth v1.0.0

**The first public release of ChessGrowth**, a standalone opening-repertoire generator and editor written in Go.

## Highlights

- Build your repertoire from an interactive chessboard, SAN/UCI moves, or FEN, for White or Black.
- Use Lichess Opening Explorer statistics to cover the opponent's most frequent replies.
- Choose **Cloud → Stockfish fallback** to evaluate positions locally whenever Lichess Cloud has no score.
- Optionally save a personal Lichess API token across launches: macOS Keychain or per-user Windows/Linux storage. No token is embedded in the app or exported results.
- Inspect the complete opening graph, delete entire branches, undo pruning, and export PGN or JSON.
- Built-in chess engine availability check with OS-specific installation guidance.
- Native ChessGrowth icon on macOS/Windows and Linux desktop integration.

## Downloads

- **macOS Apple Silicon:** `ChessGrowth-v1.0.0-darwin-arm64.dmg` or `.zip` (DMG made on macOS).
- **macOS Intel:** `ChessGrowth-v1.0.0-darwin-amd64.dmg` or `.zip` (DMG made on macOS).
- **Windows x64:** `ChessGrowth-v1.0.0-windows-amd64-Setup.exe` (if NSIS is installed), or portable `.zip`.
- **Ubuntu x64/ARM64:** matching `.deb` for installer-based deployment, or `.tar.gz` portable archives.
- Verify downloads against `SHA256SUMS.txt`.

## Requirements / limitations

Internet access is required for Lichess opening data. The optional Stockfish engine is **not bundled**; install it separately for local or hybrid evaluations. Saved credentials on Windows/Linux are kept in a user-private file, not encrypted at rest. macOS uses Keychain. Session pruning is held in memory until export. Standard chess only.

These builds are unsigned and not Apple-notarized. Please test installers on target operating systems before wide deployment. Not affiliated with Lichess or Stockfish.
