import {expect, test} from "@playwright/test";
import {readFile} from "node:fs/promises";

const source = await readFile(new URL("../../../packages/webassets/static/passkeys.js", import.meta.url), "utf8");
const origin = "http://localhost:38127";
const next = "/?view=movies";

test("accepted browser session resumes before any passkey offer @smoke", async ({page}) => {
  let begins = 0;
  await page.route(`${origin}/**`, route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/static/passkeys.js") return route.fulfill({contentType:"text/javascript", body: source});
    if (path === "/api/v1/me") return route.fulfill({status:200, json:{id:"synthetic-viewer"}});
    if (path.endsWith("/begin")) { begins++; return route.fulfill({status:401}); }
    if (path === "/") return route.fulfill({contentType:"text/html", body:"<h1>Resumed library</h1>"});
    return route.fulfill({contentType:"text/html", body:'<body data-login-next="/?view=movies"><button data-passkey-login>Sign in with passkey</button><output data-passkey-status></output><script src="/static/passkeys.js"></script></body>'});
  });
  await page.addInitScript(() => localStorage.setItem("kinosail-passkey", "1"));
  await page.goto(`${origin}/login?next=${encodeURIComponent(next)}`);
  await expect(page).toHaveURL(origin+next);
  await expect(page.getByRole("heading", {name:"Resumed library"})).toBeVisible();
  expect(begins).toBe(0);
});

for (const mode of ["stepup", "switch"]) {
  test(`explicit ${mode} keeps sign-in available`, async ({page}) => {
    let checks = 0;
    await page.route(`${origin}/**`, route => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/static/passkeys.js") return route.fulfill({contentType:"text/javascript", body:source});
      if (path === "/api/v1/me") { checks++; return route.fulfill({status:200, json:{id:"synthetic-viewer"}}); }
      return route.fulfill({contentType:"text/html", body:'<button data-passkey-login>Sign in with passkey</button><script src="/static/passkeys.js"></script>'});
    });
    await page.goto(`${origin}/login?${mode}=1`);
    await expect(page.getByRole("button", {name:"Sign in with passkey"})).toBeVisible();
    expect(new URL(page.url()).pathname).toBe("/login");
    expect(checks).toBe(0);
  });
}

for (const response of [401, 403, "network"] as const) {
  test(`a ${response} session check keeps passkey sign-in`, async ({page}) => {
    let begins = 0;
    await page.route(`${origin}/**`, route => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/static/passkeys.js") return route.fulfill({contentType:"text/javascript", body:source});
      if (path === "/api/v1/me") return response === "network" ? route.abort() : route.fulfill({status:response});
      if (path.endsWith("/begin")) { begins++; return route.fulfill({status:401}); }
      return route.fulfill({contentType:"text/html", body:'<button data-passkey-login>Sign in with passkey</button><output data-passkey-status></output><script src="/static/passkeys.js"></script>'});
    });
    await page.addInitScript(() => localStorage.setItem("kinosail-passkey", "1"));
    await page.goto(`${origin}/login`);
    await expect.poll(() => begins).toBe(1);
    await expect(page.getByRole("button", {name:"Sign in with passkey"})).toBeVisible();
    expect(new URL(page.url()).pathname).toBe("/login");
  });
}

for (const target of ["https://outside.example/", "//outside.example/", "/login?next=%2Flogin"]) {
  test(`resume rejects unsafe or looping return ${target}`, async ({page}) => {
    await page.route(`${origin}/**`, route => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/static/passkeys.js") return route.fulfill({contentType:"text/javascript", body:source});
      if (path === "/api/v1/me") return route.fulfill({status:200, json:{id:"synthetic-viewer"}});
      if (path === "/") return route.fulfill({contentType:"text/html", body:"<h1>Resumed library</h1>"});
      return route.fulfill({contentType:"text/html", body:'<button data-passkey-login>Sign in with passkey</button><script src="/static/passkeys.js"></script>'});
    });
    await page.goto(`${origin}/login?next=${encodeURIComponent(target)}`);
    await expect(page).toHaveURL(origin+"/");
  });
}
