# ChessGrowth v1.3.3 — See every position as your repertoire grows

The **Watch your opening tree grow** dialog now includes a **Current Position** chessboard beside the live branching graph. The view updates to show the position ChessGrowth is currently exploring or evaluating, plus the last move, side to move and any engine score already available. The board is rendered with the same fixed-coordinate SVG pieces as the setup view, so it does not resize when pieces move. It automatically orients toward the repertoire player's side.

This UI feature does not add Lichess API requests or Stockfish calls. It preserves all v1.3.2 features including 10,000 default / 100,000 maximum nodes, Cloud rate-limit handling, persistent token, checkpoints and transpositions.

Native macOS and Windows installer bundles are unsigned.
