/**
 * random-password — 敏感口令字段的随机生成纯函数（供 shared/ui/PasswordGenerator 等复用）。
 */

const UPPER = "ABCDEFGHJKLMNPQRSTUVWXYZ";
const LOWER = "abcdefghijkmnpqrstuvwxyz";
const DIGIT = "23456789";
const SPECIAL = "!@#$%^&*+-=?";
const ALL = UPPER + LOWER + DIGIT + SPECIAL;

/** 从字符集中等概率取一个字符（拒绝采样避免取模偏置）。 */
function pick(charset: string): string {
  const limit = Math.floor(0xffffffff / charset.length) * charset.length;
  const buf = new Uint32Array(1);
  do {
    crypto.getRandomValues(buf);
  } while (buf[0] >= limit);
  return charset[buf[0] % charset.length];
}

/** 生成长度为 length（默认 16）的随机口令，至少各含一个大写/小写/数字/特殊字符。 */
export function generateRandomPassword(length = 16): string {
  const size = Math.max(4, Math.min(64, Math.floor(length)));
  const chars = [pick(UPPER), pick(LOWER), pick(DIGIT), pick(SPECIAL)];
  while (chars.length < size) chars.push(pick(ALL));
  // Fisher-Yates 打散固定前缀的位置。
  const rand = new Uint32Array(chars.length);
  crypto.getRandomValues(rand);
  for (let i = chars.length - 1; i > 0; i -= 1) {
    const j = rand[i] % (i + 1);
    [chars[i], chars[j]] = [chars[j], chars[i]];
  }
  return chars.join("");
}
