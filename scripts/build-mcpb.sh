#!/bin/sh
# build-mcpb.sh <binary-path> <goos> <goarch> <version>
#
# Packages one built gamesom binary as an MCP Bundle (.mcpb): a zip whose
# root holds manifest.json plus server/gamesom(.exe). Run by GoReleaser as a
# per-build post hook; also runnable by hand against any binary.
set -eu

BIN="$1"
GOOS="$2"
GOARCH="$3"
VERSION="$4"

case "$GOOS" in
windows)
	PLATFORM=win32
	BINNAME=gamesom.exe
	;;
darwin)
	PLATFORM=darwin
	BINNAME=gamesom
	;;
linux)
	PLATFORM=linux
	BINNAME=gamesom
	;;
*)
	echo "build-mcpb: unsupported GOOS $GOOS" >&2
	exit 1
	;;
esac

STAGE="dist/mcpb-stage/${GOOS}_${GOARCH}"
OUT_DIR="dist/mcpb"
OUT="$OUT_DIR/gamesom_${VERSION}_${GOOS}_${GOARCH}.mcpb"

rm -rf "$STAGE"
mkdir -p "$STAGE/server" "$OUT_DIR"
cp "$BIN" "$STAGE/server/$BINNAME"

sed -e "s/__VERSION__/$VERSION/g" \
	-e "s/__BIN__/$BINNAME/g" \
	-e "s/__PLATFORM__/$PLATFORM/g" \
	packaging/mcpb/manifest.template.json >"$STAGE/manifest.json"

rm -f "$OUT"
if command -v zip >/dev/null 2>&1; then
	# Zip from inside the stage so manifest.json sits at the bundle root.
	OUT_ABS="$(pwd)/$OUT"
	(cd "$STAGE" && zip -qr "$OUT_ABS" manifest.json server)
else
	# CI (ubuntu-latest) has zip; this keeps the script runnable in
	# environments that only have python3. Preserves file modes.
	python3 - "$STAGE" "$OUT" <<'PYEOF'
import os, sys, zipfile

stage, out = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
    for root, _, files in os.walk(stage):
        for name in files:
            path = os.path.join(root, name)
            z.write(path, os.path.relpath(path, stage))
PYEOF
fi

echo "build-mcpb: built $OUT"
