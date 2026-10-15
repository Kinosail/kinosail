import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const allocator = resolve(execFileSync("git", ["rev-parse", "--show-toplevel"], { encoding: "utf8" }).trim(), "scripts/testing/fixture-totp.py");

export function fixtureTotp(secret = process.env.KINOSAIL_TEST_TOTP_SECRET ?? ""): string {
  const code = execFileSync("python3", [allocator, secret], { encoding: "utf8", timeout: 660_000 }).trim();
  if (!/^\d{6}$/.test(code)) throw new Error("fixture TOTP allocator returned an invalid code");
  return code;
}
