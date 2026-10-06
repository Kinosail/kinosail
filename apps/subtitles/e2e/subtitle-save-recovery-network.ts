import type { Page, Request, Response } from "@playwright/test";
import type { SafeCase } from "./subtitle-save-recovery-helpers";

// Install before the real click. Only the exact item's actual POST is bound.
export function observeSave(page: Page, origin: string, item: string, result: SafeCase) {
  let request: Request | null = null, response: Response | null = null;
  let settle: (value: "finished" | "request-failed") => void = () => {};
  const terminal = new Promise<"finished" | "request-failed">(resolve => { settle = resolve; });
  const onRequest = (candidate: Request) => {
    if (candidate.method() !== "POST" || candidate.url() !== origin+"/api/v1/subtitle-library/"+item+"/apply") return;
    if (request) { result.failureStage = "late-response"; return; }
    request = candidate; result.saveRequestObserved = true; result.saveTerminal = "pending";
  };
  const onResponse = (candidate: Response) => { if (candidate.request() === request) { response = candidate; result.saveResponseObserved = true; } };
  const onFinished = (candidate: Request) => { if (candidate === request) { result.saveTerminal = "finished"; settle("finished"); } };
  const onFailed = (candidate: Request) => { if (candidate === request) { result.saveTerminal = "request-failed"; settle("request-failed"); } };
  page.on("request", onRequest); page.on("response", onResponse);
  page.on("requestfinished", onFinished); page.on("requestfailed", onFailed);
  return { terminal, response: () => response, close: () => {
    page.off("request", onRequest); page.off("response", onResponse);
    page.off("requestfinished", onFinished); page.off("requestfailed", onFailed);
  }};
}
