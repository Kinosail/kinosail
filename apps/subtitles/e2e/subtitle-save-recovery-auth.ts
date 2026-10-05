import { createHmac, randomBytes } from "node:crypto";
import type { Page } from "@playwright/test";

function totp(secret: string): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = 0, value = 0; const bytes: number[] = [];
  for (const letter of secret) { const index = alphabet.indexOf(letter); if (index < 0) throw new Error("fixed-auth-boundary");
    value = (value << 5) | index; bits += 5; if (bits >= 8) { bits -= 8; bytes.push((value >>> bits) & 255); } }
  const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac("sha1", Buffer.from(bytes)).update(counter).digest();
  const at = digest[digest.length - 1] & 15;
  return String((digest.readUInt32BE(at) & 0x7fffffff) % 1000000).padStart(6, "0");
}
export async function owner(page: Page, origin: string) {
  const response = await page.request.post(origin + "/setup", {
    form: { name: "Fictional Owner", password: randomBytes(24).toString("hex") + "Aa7!", totp: "true", updateMode: "manual" },
    headers: { Origin: origin }, maxRedirects: 0, timeout: 15000,
  });
  const html = await response.text();
  const secret = /<code>([A-Z2-7]{32})<\/code>/.exec(html)?.[1];
  const cookies = await page.context().cookies(origin);
  if (response.status() !== 200 || Buffer.byteLength(html, "utf8") > 65536 || !secret || !cookies.some(cookie =>
    cookie.name === "__Host-kinosail_subtitles_session" && cookie.secure && cookie.httpOnly &&
    cookie.sameSite === "Strict" && cookie.path === "/")) throw new Error("fixed-auth-boundary");
  // Setup sets the response cookie; read CSRF from an authenticated public page.
  const enrollment = await page.request.get(origin + "/account", { maxRedirects: 0, timeout: 15000 });
  const enrollmentHTML = await enrollment.text();
  const enrollmentTokens = [...enrollmentHTML.matchAll(/name="kinosail-csrf" content="([A-Za-z0-9_-]{43})"/g)];
  const csrf = enrollmentTokens.length === 1 ? enrollmentTokens[0][1] : undefined;
  if (enrollment.status() !== 200 || Buffer.byteLength(enrollmentHTML, "utf8") > 65536 || !csrf) throw new Error("fixed-auth-boundary");
  const confirmed = await page.request.post(origin + "/account/mfa/enable", {
    form: { code: totp(secret), _csrf: csrf }, headers: { Origin: origin }, maxRedirects: 0, timeout: 15000,
  });
  if (confirmed.status() !== 303) throw new Error("fixed-auth-boundary");
  await page.goto(origin + "/?view=library", { waitUntil: "domcontentloaded", timeout: 15000 });
  if (!(await page.locator('meta[name="kinosail-csrf"]').getAttribute("content"))) throw new Error("fixed-auth-boundary");
}
