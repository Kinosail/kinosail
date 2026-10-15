import { fixtureTotp } from "../../../scripts/testing/fixture-totp";
import { randomBytes } from "node:crypto";
import type { Page } from "@playwright/test";

function totp(secret: string): string {
  return fixtureTotp(secret);
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
