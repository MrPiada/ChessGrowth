# ChessGrowth v1.2.0 — Stable board and transposition-aware growth

### Changes

- Rebuilt the board as one fixed-coordinate SVG containing **exactly 64 equal-size squares**. Pieces cannot resize board cells; move selection, highlights, promotions and legal-move validation remain supported.
- Live branching preview now labels **every displayed dot with its SAN move and full-move number**. The graph can scroll and follows recently explored branches.
- Position dots are colored continuously **green for favorable** or **red for unfavorable** relative to the selected repertoire color. No engine evaluation is shown neutrally in gray; transpositions are marked ↪.
- Memoized Lichess Explorer and engine evaluations by normalized four-field FEN; the HTTP disk cache also ignores move clocks. Each unique position is scheduled for expansion once, with subsequent visits linking to its canonical node.
- Added a live counter for saved duplicate expansions and PGN transposition annotations.
- Improved inspector chessboard sizing within flexible layouts. Updated README and screenshots.

### Important limitations

This is opening-analysis transposition equality: the halfmove clock and repetition history are not incorporated. A transposition edge remains a short PGN line with a note pointing to its canonical continuation rather than copying that subtree. Engines are external (Stockfish not bundled). Data shown in documentation screenshots is **synthetic**. Installers are unsigned/not notarized; test on actual OS versions before a stable release.
