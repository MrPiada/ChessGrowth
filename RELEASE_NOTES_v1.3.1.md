# ChessGrowth v1.3.1 — Reliable generation and Stockfish fallback

This update addresses Cloud evaluation rate limits and interrupted analyses without changing the opening-selection strategy.

- **Immediate fallback on Cloud HTTP 429**: Hybrid mode stops retrying Cloud for the current generation and switches to Stockfish. Subsequent Cloud calls are skipped. HTTP 502/503/504 also trigger immediate fallback in Hybrid mode.
- **Lichess-compliant cooldown**: a 429 pauses *all new Lichess API requests* for at least 60 seconds or longer according to `Retry-After`. The cooldown persists across app restarts and is visible in the live progress. Cached results and local Stockfish evaluation remain available during the pause.
- **Persistent Stockfish evaluation cache**: results are reused across runs by normalized position, engine executable signature and requested depth. No external database or Python dependency; cache files are stored atomically in the user's cache directory.
- **Two-phase generation**: build the opening tree first, evaluating move candidates as needed, then complete the scores of remaining retained nodes. Statistics from Opening Explorer are still required to grow new branches.
- **Resume interrupted analyses**: secure local JSON checkpoints retain the last completed position, queue, visited positions and evaluation phase. Use the new checkbox to resume with matching settings, or uncheck it to start fresh.
- **Diagnostics and authentication**: includes startup request progress, bounded macOS Keychain reads, and the Opening Explorer authentication fix from the earlier diagnostics patch.

## Important

Lichess asks API users to make requests sequentially and to wait at least one full minute after HTTP 429. Stockfish replaces Cloud **evaluations**, not Opening Explorer statistics. If Explorer is also rate limited and no cached statistics exist, the app waits or reports the error rather than bypassing the provider's limits.

This release uses a file-based cache (JSON, atomic 0600 writes) instead of SQLite, to keep the self-contained cross-platform Go binaries free of external database dependencies. No saved Lichess token is included in cache files, PGN, reports or checkpoints.

The generated installers are unsigned; test on macOS, Windows and Ubuntu before wider rollout.
