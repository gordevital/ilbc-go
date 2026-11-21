#!/usr/bin/env bash
# Regenerates internal/srtp/testdata/vectors_gcm.txt from a pristine libsrtp.
#
# The pinned libsrtp revision below is the one the committed golden vectors
# were produced with; changing SRC_SHA (or the openssl build flavour) may
# change the bytes and thus the Go test expectations.
#
# Requirements: gcc, make, cmake, openssl development headers.
#   - Default flow: clones libsrtp into $OUT_DIR, builds it, links vectorgen,
#     and writes the vectors straight into the repo testdata.
#   - Prebuilt tree: set LIBSRTP_DIR to a libsrtp checkout that already has
#     its headers under <dir>/include and a static libsrtp2.a somewhere under
#     <dir> (typical cmake layout <dir>/build). Nothing is compiled then.
#
# Usage: tools/srtpvectors/build_vectors.sh [dest-vectors-file]
set -euo pipefail
cd "$(dirname "$0")/../.."

SRC_SHA=${SRC_SHA:-a566a9c}
SRC_URL=${SRC_URL:-https://github.com/cisco/libsrtp.git}
OUT_DIR=${OUT_DIR:-/tmp/srtpvec_build}
DEST=${1:-internal/srtp/testdata/vectors_gcm.txt}

LIB_DIR=""
LIB_ROOT=""
mkdir -p "$OUT_DIR"
if [[ -n "${LIBSRTP_DIR:-}" ]]; then
  LIB_DIR="$LIBSRTP_DIR"
  LIB_ROOT="$LIBSRTP_DIR"
else
  mkdir -p "$OUT_DIR"
  if [[ ! -d "$OUT_DIR/libsrtp" ]]; then
    git clone --quiet "$SRC_URL" "$OUT_DIR/libsrtp"
  fi
  git -C "$OUT_DIR/libsrtp" fetch --quiet --depth 1 origin "$SRC_SHA" || true
  git -C "$OUT_DIR/libsrtp" checkout --quiet "$SRC_SHA"
  LIB_DIR="$OUT_DIR/libsrtp"
  LIB_ROOT="$LIB_DIR"
  cmake -S "$LIB_DIR" -B "$LIB_DIR/build" -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=OFF -DENABLE_OPENSSL=ON > /dev/null
  cmake --build "$LIB_DIR/build" --quiet -j"$(nproc)"
fi

lib_archive=""
for cand in \
  "$LIB_ROOT/build/libsrtp2.a" \
  "$LIB_ROOT/build/lib/libsrtp2.a" \
  "$LIB_ROOT/build/src/libsrtp2.a" \
  "$LIB_ROOT/src/libsrtp2.a" \
  "$LIB_ROOT/libsrtp2.a"; do
  if [[ -f "$cand" ]]; then
    lib_archive="$cand"
    break
  fi
done
[[ -n "$lib_archive" ]] || {
  echo "error: no static libsrtp2.a found under $LIB_ROOT" >&2
  exit 1
}

BIN="$OUT_DIR/vectorgen"
gcc -O2 -Wall -o "$BIN" "$PWD/tools/srtpvectors/vectorgen.c" \
  -I"$LIB_DIR/include" -L"$(dirname "$lib_archive")" -lsrtp2 -lssl -lcrypto -lm

"$BIN" > "$DEST.tmp"
mv "$DEST.tmp" "$DEST"
echo "wrote $DEST ($(wc -l < "$DEST") vectors) from $(basename "$lib_archive")"
