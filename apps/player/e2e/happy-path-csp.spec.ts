import {expect, test} from "@playwright/test";
import {createServer} from "node:http";
import {captureHappyPathErrors} from "./happy-path-errors";

const policy = "default-src 'self'; script-src 'self'; style-src 'self'; object-src 'none'";
const select = '<label>Language <select name="language"><option>English</option><option>Deutsch</option></select></label>';
const scenarios: Record<string, {body: string; reported: boolean}> = {
  "select control": {body: select, reported: false},
  "select control with script-set styles": {body: `${select}<p>Styled</p><script src="/cssom.js"></script>`, reported: false},
  "select with inline style attribute": {body: '<label>Language <select name="language" style="color:red"><option>English</option></select></label>', reported: true},
  "inline style element": {body: `${select}<style>p{color:red}</style><p>Inline</p>`, reported: true},
  "inline style attribute": {body: `${select}<p style="color:red">Inline</p>`, reported: true},
  "script style injection": {body: `${select}<script src="/inject.js"></script>`, reported: true},
  "script style attribute": {body: `${select}<p>Styled</p><script src="/attribute.js"></script>`, reported: true},
};

for (const [scenario, {body, reported}] of Object.entries(scenarios)) {
  test(`happy path CSP errors classify ${scenario}`, async ({page, browserName}, info) => {
    const server = createServer((request, response) => {
      if (request.url === "/inject.js") {
        response.writeHead(200, {"Content-Type": "text/javascript", "Content-Security-Policy": policy});
        response.end("const style=document.createElement('style');style.textContent='p{color:red}';document.head.append(style);");
        return;
      }
      if (request.url === "/attribute.js") {
        response.writeHead(200, {"Content-Type": "text/javascript", "Content-Security-Policy": policy});
        response.end("document.querySelector('p').setAttribute('style', 'color:red');");
        return;
      }
      if (request.url === "/cssom.js") {
        response.writeHead(200, {"Content-Type": "text/javascript", "Content-Security-Policy": policy});
        response.end("for (const element of document.querySelectorAll('p, select')) element.style.setProperty('--accent', 'red');");
        return;
      }
      if (request.url === "/favicon.ico") {
        response.writeHead(204);
        response.end();
        return;
      }
      response.writeHead(request.url === "/" ? 200 : 404,{"Content-Type": "text/html; charset=utf-8", "Content-Security-Policy": policy});
      response.end(request.url === "/" ? `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>CSP control</title></head><body><main>${body}</main></body></html>` : "");
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("HTTP control did not bind");
    const monitor = captureHappyPathErrors(page, browserName);
    try {
      await page.goto(`http://127.0.0.1:${address.port}/`);
      await expect(page.getByRole("combobox", {name: "Language"})).toBeVisible();
      await page.waitForTimeout(250);
      await expect.poll(() => monitor.settled()).toBe(true);
      const observed = monitor.evidence().map(error => error.message);
      if (reported) expect(monitor.errors().length).toBeGreaterThan(0);
      else expect(monitor.errors()).toEqual([]);
      await info.attach("csp-error-classification", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION, browser: browserName,
        version: page.context().browser()?.version(), scenario, policy, observed, unexpected: monitor.errors(), result: "passed"}), contentType: "application/json"});
    } finally {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    }
  });
}
