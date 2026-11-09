import * as Crypto from 'expo-crypto';
const alphabet = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';

function decodeBase58(value: string): Uint8Array | null {
  let number = 0n;
  for (const character of value) {
    const index = alphabet.indexOf(character);
    if (index < 0) return null;
    number = number * 58n + BigInt(index);
  }
  const bytes: number[] = [];
  while (number > 0n) { bytes.push(Number(number % 256n)); number /= 256n; }
  bytes.reverse();
  let leadingZeroes = 0;
  while (leadingZeroes < value.length && value[leadingZeroes] === '1') leadingZeroes++;
  return Uint8Array.from([...new Array(leadingZeroes).fill(0), ...bytes]);
}
function equalBytes(left: Uint8Array, right: Uint8Array): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}
/** 执行完整 TRON Base58Check 校验，而不是只判断 T 前缀。 */
export async function validateTronAddress(input: string): Promise<string | null> {
  const address = input.trim();
  if (!address) return '请输入 TRON 地址';
  if (address.startsWith('0x')) return '这是 EVM 地址，请输入 TRON Base58 地址';
  if (address.length !== 34) return 'TRON 地址长度不正确';
  const decoded = decodeBase58(address);
  if (!decoded || decoded.length !== 25 || decoded[0] !== 0x41) return 'TRON 地址格式不正确';
  const payload = decoded.slice(0, 21);
  const checksum = decoded.slice(21);
  const firstHash = await Crypto.digest(Crypto.CryptoDigestAlgorithm.SHA256, payload);
  const secondHash = await Crypto.digest(Crypto.CryptoDigestAlgorithm.SHA256, new Uint8Array(firstHash));
  return equalBytes(checksum, new Uint8Array(secondHash).slice(0, 4)) ? null : 'TRON 地址校验和错误';
}
export function truncateAddress(address: string, start = 7, end = 6): string {
  if (address.length <= start + end + 3) return address;
  return `${address.slice(0, start)}…${address.slice(-end)}`;
}
