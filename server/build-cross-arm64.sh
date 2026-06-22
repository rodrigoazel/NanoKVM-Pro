#!/usr/bin/env bash
#
# Cross-compile NanoKVM-Server for the NanoKVM-Pro (AX630C / ARM64) from a
# non-Linux host (macOS etc.) using zig as the CGO cross C/C++ compiler.
#
# The binary links against link-time STUBS for libkvm and libopus. It is NEVER
# shipped with those stubs: on the device the real libkvm.so (resolved via the
# $ORIGIN/dl_lib rpath, like the official build) and the system libopus.so.0
# provide the actual implementations at runtime. Only the NanoKVM-Server binary
# is replaced on the device; dl_lib/ and libopus stay untouched.
#
# Requires: go, zig, patchelf   ->   brew install go zig patchelf
# Run from the server/ project root:  ./build-cross-arm64.sh
#
set -euo pipefail

# Pin a conservative glibc so the binary runs on the device's (older) rootfs.
GLIBC_TARGET="${GLIBC_TARGET:-aarch64-linux-gnu.2.31}"

if [ ! -f dl_lib/kvmimpl.cpp ]; then
    echo "[ERROR] run from the server/ project root (dl_lib/kvmimpl.cpp not found)" >&2
    exit 1
fi

STUB_DIR="$(mktemp -d)"
cleanup() { rm -rf "$STUB_DIR" dl_lib/libkvm.so dl_lib/libkvm.so.0; }
trap cleanup EXIT

echo "[*] Building link-time stub libkvm.so (arm64) from dl_lib/kvmimpl.cpp ..."
zig c++ -target "$GLIBC_TARGET" -w -fPIC -shared -Wl,-soname,libkvm.so \
    -o dl_lib/libkvm.so dl_lib/kvmimpl.cpp
cp dl_lib/libkvm.so dl_lib/libkvm.so.0

echo "[*] Generating link-time opus stub (header + libopus.so.0) ..."
mkdir -p "$STUB_DIR/opus"
cat > "$STUB_DIR/opus/opus.h" <<'EOF'
#ifndef OPUS_STUB_H
#define OPUS_STUB_H
typedef int opus_int32;
typedef short opus_int16;
typedef struct OpusDecoder OpusDecoder;
#define OPUS_OK 0
#ifdef __cplusplus
extern "C" {
#endif
OpusDecoder *opus_decoder_create(opus_int32 Fs, int channels, int *error);
int opus_decode(OpusDecoder *st, const unsigned char *data, opus_int32 len,
                opus_int16 *pcm, int frame_size, int decode_fec);
void opus_decoder_destroy(OpusDecoder *st);
const char *opus_strerror(int error);
#ifdef __cplusplus
}
#endif
#endif
EOF
cat > "$STUB_DIR/opusstub.c" <<'EOF'
typedef int opus_int32;
typedef short opus_int16;
typedef struct OpusDecoder OpusDecoder;
OpusDecoder *opus_decoder_create(opus_int32 Fs, int ch, int *e){ if(e)*e=0; return (OpusDecoder*)0; }
int opus_decode(OpusDecoder *s, const unsigned char *d, opus_int32 l, opus_int16 *p, int f, int fec){ (void)s;(void)d;(void)l;(void)p;(void)f;(void)fec; return 0; }
void opus_decoder_destroy(OpusDecoder *s){ (void)s; }
const char *opus_strerror(int e){ (void)e; return ""; }
EOF
zig cc -target "$GLIBC_TARGET" -fPIC -shared -Wl,-soname,libopus.so.0 \
    -o "$STUB_DIR/libopus.so" "$STUB_DIR/opusstub.c"

echo "[*] Cross-compiling NanoKVM-Server (linux/arm64, CGO via zig) ..."
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date +%Y-%m-%d_%H:%M:%S)"
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
    CC="zig cc -target $GLIBC_TARGET" \
    CXX="zig c++ -target $GLIBC_TARGET" \
    CGO_CFLAGS="-I$STUB_DIR" \
    CGO_LDFLAGS="-L$PWD/dl_lib -L$STUB_DIR" \
    go build -ldflags "-s -w -X 'main.Commit=$COMMIT' -X 'main.BuildTime=$BUILD_TIME'" \
    -o NanoKVM-Server .

echo "[*] Fixing dynamic deps (rpath + libkvm basename) ..."
patchelf --replace-needed "$PWD/dl_lib/libkvm.so" libkvm.so NanoKVM-Server 2>/dev/null || true
patchelf --set-rpath '$ORIGIN/dl_lib' NanoKVM-Server

echo "[*] Done."
file NanoKVM-Server
echo "NEEDED: $(patchelf --print-needed NanoKVM-Server | tr '\n' ' ')"
echo "RPATH:  $(patchelf --print-rpath NanoKVM-Server)"
