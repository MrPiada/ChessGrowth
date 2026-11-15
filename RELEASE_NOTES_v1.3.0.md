# ChessGrowth v1.3.0 — Clean application shutdown

This release fixes the application remaining open in the background after the browser tab is closed.

- Added **Quit application** to the top-right toolbar; stops the Go server and cancels active repertoire generation.
- Closing the **last ChessGrowth tab** now shuts the app down after a six-second reconnect grace period.
- Multiple tabs and normal page refreshes are supported without premature exit.
- Uses a persistent Server-Sent Events browser connection plus a best-effort unload beacon.
- `--no-open` intentionally keeps headless sessions running until Ctrl+C.
- Setup chessboard, opening tree, cached transpositions, Stockfish fallback, persistent token and PGN output remain intact.

**Platforms:** macOS ARM64/Intel, Windows x64, Ubuntu Linux x64/ARM64. Build on macOS to generate DMGs and optional Windows NSIS installer.

**QA:** Go tests, `go vet`, cross-builds and live process lifecycle checks over HTTP/SSE. The build environment restricts graphical browser navigation to localhost, so test the native macOS app before marking the release stable. Unsigned/unnotarized packages.
