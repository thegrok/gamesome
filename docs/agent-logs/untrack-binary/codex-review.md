- **High** [gamesom](/home/claude/repos/gamesom:1): `git diff main...HEAD` does not remove the tracked root binary. `git ls-tree -l HEAD gamesom` still shows the 15,700,833-byte executable tracked at `HEAD`, violating the spec's load-bearing contract to `git rm --cached gamesom`.

- **High** [.gitignore](/home/claude/repos/gamesom/.gitignore:4): The committed branch does not add `/gamesom` and `/gamesome` under the binaries section. That misses both named edge cases in the spec: pre-rename `gamesom` binaries and post-rename `gamesome` rebuilds.

Note: the working tree has unstaged `.gitignore` and `gamesom` changes, but they are not part of `git diff main...HEAD`, so I did not count them as implemented.
