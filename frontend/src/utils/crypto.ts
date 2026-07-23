/**
 * 密码学工具函数库
 * 提供跨 HTTP/HTTPS 场景兼容的 SHA-256 散列计算
 */

/**
 * 校验当前浏览器是否支持 Web Crypto 原生 SHA-256 (仅 Secure Context / HTTPS / localhost 可用)
 */
export function isNativeCryptoSupported(): boolean {
  return typeof window !== 'undefined' && !!(window.crypto && window.crypto.subtle && typeof window.crypto.subtle.digest === 'function')
}

/**
 * 纯 JavaScript 实现的 SHA-256 降级算法 (支持标准 UTF-8 编码)
 * 用于非安全上下文 (HTTP 环境下 window.crypto.subtle 为 undefined 的情况)
 */
export function sha256PureJS(message: string): string {
  const encoder = new TextEncoder()
  const bytes = encoder.encode(message)

  const K = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef4a3f7, 0xc67178f2
  ]

  let H0 = 0x6a09e667
  let H1 = 0xbb67ae85
  let H2 = 0x3c6ef372
  let H3 = 0xa54ff53a
  let H4 = 0x510e527f
  let H5 = 0x9b05688c
  let H6 = 0x1f83d9ab
  let H7 = 0x5be0cd19

  const l = bytes.length
  const bitLen = l * 8
  const k = (55 - (l % 64) + 64) % 64
  const padded = new Uint8Array(l + 1 + k + 8)
  padded.set(bytes)
  padded[l] = 0x80

  const view = new DataView(padded.buffer)
  view.setUint32(padded.length - 4, bitLen & 0xffffffff, false)
  view.setUint32(padded.length - 8, Math.floor(bitLen / 0x100000000), false)

  const W = new Int32Array(64)
  const rightRotate = (v: number, n: number) => (v >>> n) | (v << (32 - n))

  for (let offset = 0; offset < padded.length; offset += 64) {
    for (let i = 0; i < 16; i++) {
      W[i] = view.getInt32(offset + i * 4, false)
    }
    for (let i = 16; i < 64; i++) {
      const s0 = rightRotate(W[i - 15], 7) ^ rightRotate(W[i - 15], 18) ^ (W[i - 15] >>> 3)
      const s1 = rightRotate(W[i - 2], 17) ^ rightRotate(W[i - 2], 19) ^ (W[i - 2] >>> 10)
      W[i] = (W[i - 16] + s0 + W[i - 7] + s1) | 0
    }

    let a = H0, b = H1, c = H2, d = H3, e = H4, f = H5, g = H6, h = H7

    for (let i = 0; i < 64; i++) {
      const S1 = rightRotate(e, 6) ^ rightRotate(e, 11) ^ rightRotate(e, 25)
      const ch = (e & f) ^ (~e & g)
      const temp1 = (h + S1 + ch + K[i] + W[i]) | 0
      const S0 = rightRotate(a, 2) ^ rightRotate(a, 13) ^ rightRotate(a, 22)
      const maj = (a & b) ^ (a & c) ^ (b & c)
      const temp2 = (S0 + maj) | 0

      h = g
      g = f
      f = e
      e = (d + temp1) | 0
      d = c
      c = b
      b = a
      a = (temp1 + temp2) | 0
    }

    H0 = (H0 + a) | 0
    H1 = (H1 + b) | 0
    H2 = (H2 + c) | 0
    H3 = (H3 + d) | 0
    H4 = (H4 + e) | 0
    H5 = (H5 + f) | 0
    H6 = (H6 + g) | 0
    H7 = (H7 + h) | 0
  }

  const toHex = (n: number) => (n >>> 0).toString(16).padStart(8, '0')
  return toHex(H0) + toHex(H1) + toHex(H2) + toHex(H3) + toHex(H4) + toHex(H5) + toHex(H6) + toHex(H7)
}

/**
 * 异步计算 SHA-256 哈希值 (优先原生 Web Crypto API，HTTP 非安全上下文自动降级至 pure JS)
 */
export async function sha256(message: string): Promise<string> {
  if (isNativeCryptoSupported()) {
    const msgBuffer = new TextEncoder().encode(message)
    const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer)
    const hashArray = Array.from(new Uint8Array(hashBuffer))
    return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
  }
  console.info('[Crypto] 原生 Web Crypto 不可用 (HTTP非安全上下文)，已自动降级至 Pure JS SHA-256 算法')
  return sha256PureJS(message)
}
