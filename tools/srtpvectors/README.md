# SRTP golden-vector generator

Regenerates `internal/srtp/testdata/vectors_gcm.txt` (the AEAD AES-GCM
golden vectors for `internal/srtp`) from a pristine libsrtp build.

```
tools/srtpvectors/build_vectors.sh [dest]   # default dest: the repo testdata
```

The script pins the libsrtp commit the committed vectors were produced with
(`SRC_SHA`, override via env) and builds it statically with
`-DENABLE_OPENSSL=ON` (required for the AEAD GCM profiles). With
`LIBSRTP_DIR` set it skips the clone/build and links against an existing
tree instead. Needs gcc, make, cmake and OpenSSL dev headers.

After a run, the fresh file must match the committed one byte-for-byte;
any drift means the libsrtp revision or build flavour changed, in which
case investigate before replacing `vectors_gcm.txt`.
