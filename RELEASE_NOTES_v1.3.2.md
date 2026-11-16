# ChessGrowth v1.3.2 — Larger repertoires

This release expands the maximum opening-tree size and adjusts the default analysis budget.

- **Default Max nodes:** 10,000 (previously 250).
- **Maximum Max nodes:** 100,000 (previously 5,000).
- The Go backend, GUI numeric input and progress-bar node budget now agree.
- No changes to Lichess rate-limit handling, Stockfish fallback or transposition caching.

Large analyses can require substantial time, memory, disk space and network requests. Lichess API limits remain in effect. Test with a small budget before generating large trees.

Cross-platform release assets can be generated on macOS using `./scripts/build-release.sh v1.3.2`.
