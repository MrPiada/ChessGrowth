# ChessGrowth v1.1.0 — Live tree and annotated repertoire

ChessGrowth is a self-contained Go opening-repertoire generator with a browser-based interface and native application icons.

### Highlights

- Replaced font-dependent chess Unicode symbols with bundled SVG pieces, consistent on macOS, Windows and Linux.
- New **live branching-tree generation monitor**, with processed/frontier counts, API requests, depth, local Stockfish evaluations and node-budget usage.
- Removed the demo mode, demo buttons and demo API endpoint.
- Evaluate **every retained position**, including opponent moves, rather than only player-choice candidates. In hybrid mode, a Cloud miss falls back to local Stockfish. Available engine mates retain their distance for PGN notation.
- Export correct **`[%eval ...]`** PGN annotations, plus Lichess White/Draw/Black percentages, sample sizes, move frequencies and a documented practical-trap heuristic (`yes/no/unverified`). PGN headers preserve database-filter conditions.
- Expanded Opening Tree controls: opening names, depth limit, collapse-by-depth, individual collapse/expand, synchronized full-move ruler and W/D/B bars in the Position Inspector.
- Persistent Lichess token management retained; OS user scope, credentials not embedded in exports.

### Notes

Stockfish is not bundled; install it separately to use local evaluation. The trap detector is a *heuristic* based on engine-score swings and Lichess frequency, not proof of forced tactics. Screenshots in the README use synthetic test data. These binaries are unsigned and not notarized; test each platform before public stable distribution.
