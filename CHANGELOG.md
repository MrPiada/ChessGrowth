## v1.3.3 — Current position preview

- Add live view-only SVG chessboard beside the branching graph while generating a repertoire.
- Transmit only the current node FEN and its SAN/ply/evaluation in the progress API.
- Keep board square sizes stable and rotate it to the selected repertoire color.
- Show side-to-move and available evaluation without additional engine calls.
- Preserve the responsive, non-page-scrolling progress dialog.

## v1.3.2

- Raise the maximum configurable node budget from 5,000 to 100,000.
- Set the default node budget to 10,000 consistently in Go and the browser GUI.
- Add tests for the expanded validation range.

## v1.3.1

- Fail fast on Lichess Cloud rate limits and fall back to local Stockfish.
- Persist API cooldowns, local evaluations and resumable analysis checkpoints.
- Defer nonessential evaluation until after opening-tree construction.
- Add visible cooldown countdown and resume control.

## 1.3.0

- Browser-window lifecycle: close last UI tab to stop server; Quit application button, multiple-tab support, reload grace and safe headless behavior.

## v1.2.0 — Fixed SVG board and deduplicated transpositions

- Fixed-size single-SVG board and inspector chessboard sizing; no piece-related layout shifts.
- Labeled, evaluation-colored live branching preview with scrollable recent branch sampling.
- Normalized FEN identity (board, turn, castling, legal en passant) and cached Explorer/evaluations once per unique position.
- Canonical transposition pointers in report JSON and PGN, plus live deduplication counter.
- Refreshed screenshots, README, tests and multiplatform build script.

# Changelog

## v1.1.0 — Live growth & annotated PGN

- Stable SVG chess-piece rendering, independent of operating-system fonts.
- Live progress tree with search-frontier estimate, queued positions and engine/API counters.
- Removed all user-facing demo functionality and the demo HTTP endpoint.
- Engine evaluation of all retained nodes including opponent replies; hybrid Cloud-to-local fallback remains supported.
- PGN evaluation directives, Lichess White/Draw/Black percentages, filters, heuristic trap labels, opening metadata.
- Graph depth/collapse controls, move-number ruler, individual branch toggles and richer position inspector.
- Expanded validation and packaging for macOS, Windows and Linux.

## v1.0.0 — First native distribution

- Go backend, native packaging and interactive opening-board setup.
- Lichess Explorer and Cloud, optional local Stockfish, pruning and PGN/JSON export.
- Persistent optional Lichess token storage in OS user settings.
