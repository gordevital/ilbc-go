/* Vector generator for the Go SRTP implementation, driven by the locally
 * built libsrtp 2.5.0 (AEAD AES-GCM profiles, RFC 7714/7712).
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <srtp.h>

#define CHECK_ST(name, st)                                                       \
  do {                                                                           \
    srtp_err_status_t _s_ = (st);                                                \
    if (_s_ != srtp_err_status_ok) {                                             \
      fprintf(stderr, "%s failed: %d\n", name, (int)_s_);                        \
      exit(2);                                                                   \
    }                                                                            \
  } while (0)

static void hexdump(const char *tag, const unsigned char *p, int n) {
  printf("%s ", tag);
  for (int i = 0; i < n; i++) printf("%02x", p[i]);
  printf("\n");
}

static srtp_t ctx_for(uint32_t ssrc, srtp_profile_t prof, const unsigned char *mk,
                      size_t mklen, const unsigned char *ms) {
  size_t klen = mklen + SRTP_AEAD_SALT_LEN;
  unsigned char *key = malloc(klen);
  memcpy(key, mk, mklen);
  memcpy(key + mklen, ms, SRTP_AEAD_SALT_LEN);
  srtp_policy_t pol;
  memset(&pol, 0, sizeof(pol));
  pol.ssrc.type = ssrc_specific;
  pol.ssrc.value = ssrc;
  pol.key = key;
  pol.window_size = 128;
  pol.allow_repeat_tx = 0;
  pol.next = NULL;
  CHECK_ST("set rtp policy", srtp_crypto_policy_set_from_profile_for_rtp(&pol.rtp, prof));
  CHECK_ST("set rtcp policy", srtp_crypto_policy_set_from_profile_for_rtcp(&pol.rtcp, prof));
  srtp_t ctx = NULL;
  CHECK_ST("srtp2_create", srtp_create(&ctx, &pol));
  free(key);
  return ctx;
}

static int make_rtp(unsigned char *pkt, uint32_t ssrc, uint16_t seq, uint32_t ts,
                    const char *payload, int plen) {
  memset(pkt, 0x33, 12 + plen);
  pkt[0] = 0x80;
  pkt[1] = 98;
  pkt[2] = (unsigned char)(seq >> 8);
  pkt[3] = (unsigned char)(seq & 0xff);
  pkt[4] = (unsigned char)((ts >> 24) & 0xff);
  pkt[5] = (unsigned char)((ts >> 16) & 0xff);
  pkt[6] = (unsigned char)((ts >> 8) & 0xff);
  pkt[7] = (unsigned char)(ts & 0xff);
  pkt[8]  = (unsigned char)((ssrc >> 24) & 0xff);
  pkt[9]  = (unsigned char)((ssrc >> 16) & 0xff);
  pkt[10] = (unsigned char)((ssrc >> 8) & 0xff);
  pkt[11] = (unsigned char)(ssrc & 0xff);
  memcpy(pkt + 12, payload, (size_t)plen);
  return 12 + plen;
}

int main(void) {
  if (srtp_init() != srtp_err_status_ok) { puts("srtp_init failed"); return 3; }
  /* ---- Case A: AES-128-GCM ---- */
  static unsigned char mk128[16];
  static unsigned char msA[12];
  for (int i = 0; i < 16; i++) mk128[i] = (unsigned char)i;
  for (int i = 0; i < 12; i++) msA[i] = (unsigned char)(0xf0 + i);

  srtp_t a = ctx_for(0x11223344, srtp_profile_aead_aes_128_gcm, mk128, 16, msA);
  unsigned char p[64];
  int n = make_rtp(p, 0x11223344, 100, 1600, "abcdefghijklmnopqrstuvwx", 24);
  CHECK_ST("protect A1", srtp_protect(a, p, &n));
  hexdump("A_rtp1", p, n);
  n = make_rtp(p, 0x11223344, 101, 1680, "ABCDEFGHIJKLMNOPQRSTUVWX", 24);
  CHECK_ST("protect A2", srtp_protect(a, p, &n));
  hexdump("A_rtp2", p, n);

  unsigned char sr[28];
  memset(sr, 0, sizeof(sr));
  sr[0] = 0x80; sr[1] = 200; sr[2] = 0x00; sr[3] = 0x05;
  sr[4] = 0x11; sr[5] = 0x22; sr[6] = 0x33; sr[7] = 0x44;
  sr[8] = 0x01; sr[9] = 0x02; sr[10] = 0x03; sr[11] = 0x04;
  sr[12] = 0x05; sr[13] = 0x06; sr[14] = 0x07; sr[15] = 0x08;
  sr[16] = 0x00; sr[17] = 0x00; sr[18] = 0x06; sr[19] = 0x40;
  sr[20] = 0x00; sr[21] = 0x00; sr[22] = 0x00; sr[23] = 0x02;
  sr[24] = 0x00; sr[25] = 0x00; sr[26] = 0x00; sr[27] = 0x38;
  n = 28;
  CHECK_ST("protect A rtcp", srtp_protect_rtcp(a, sr, &n));
  hexdump("A_rtcp", sr, n);
  srtp_dealloc(a);

  /* ---- Case B: AES-256-GCM, ROC wrap ---- */
  static unsigned char mk256[32];
  static unsigned char msB[12];
  for (int i = 0; i < 32; i++) mk256[i] = (unsigned char)(0x80 + i);
  for (int i = 0; i < 12; i++) msB[i] = (unsigned char)(0xa0 + i);

  srtp_t b1 = ctx_for(0xdeadc0de, srtp_profile_aead_aes_256_gcm, mk256, 32, msB);
  n = make_rtp(p, 0xdeadc0de, 65535, 200, "iLBC-payload-123456789", 22);
  CHECK_ST("protect B1", srtp_protect(b1, p, &n));
  hexdump("B_rtp1_wrap_at_65535", p, n);
  n = make_rtp(p, 0xdeadc0de, 0, 360, "iLBC-payload-after-wrap", 23);
  CHECK_ST("protect B2", srtp_protect(b1, p, &n));
  hexdump("B_rtp2_after_wrap", p, n);
  n = make_rtp(p, 0xdeadc0de, 1, 520, "iLBC-payload-final-xyz!", 23);
  CHECK_ST("protect B3", srtp_protect(b1, p, &n));
  hexdump("B_rtp3", p, n);
  srtp_dealloc(b1);

  srtp_t b2 = ctx_for(0xfedcbafe, srtp_profile_aead_aes_256_gcm, mk256, 32, msB);
  unsigned char rr[32];
  memset(rr, 0, sizeof(rr));
  rr[0] = 0x80; rr[1] = 201; rr[2] = 0x00; rr[3] = 0x07;
  rr[4] = 0xfe; rr[5] = 0xdc; rr[6] = 0xba; rr[7] = 0xfe; /* receiver */
  rr[8] = 0xde; rr[9] = 0xad; rr[10] = 0xc0; rr[11] = 0xde; /* reported ssrc */
  rr[12] = 0x01;                                   /* fraction lost */
  rr[13] = 0x00; rr[14] = 0x00; rr[15] = 0x02;     /* cumulative lost */
  rr[16] = 0x00; rr[17] = 0x00; rr[18] = 0x00; rr[19] = 0x02; /* ext highest */
  rr[20] = 0x00; rr[21] = 0x00; rr[22] = 0x00; rr[23] = 0x0a; /* jitter */
  rr[24] = 0x12; rr[25] = 0x34; rr[26] = 0x56; rr[27] = 0x78; /* LSR */
  rr[28] = 0x00; rr[29] = 0x00; rr[30] = 0x00; rr[31] = 0x03; /* DLSR */
  n = 32;
  CHECK_ST("protect B rtcp", srtp_protect_rtcp(b2, rr, &n));
  hexdump("B_rtcp_rr", rr, n);
  srtp_dealloc(b2);
  return 0;
}
