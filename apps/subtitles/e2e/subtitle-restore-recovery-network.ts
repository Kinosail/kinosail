import type {JSONValue} from "../../../scripts/testing/json-value";
import type { Page, Request, Response } from "@playwright/test";
import type { SafeCase } from "./subtitle-restore-recovery-helpers";

export const FAILURE_CODES = ["none", "aborted", "content-length", "decoding", "connection-reset", "connection-closed", "empty-response", "unclassified"] as const;
export type FailureCode = typeof FAILURE_CODES[number];
export function requestFailureCode(value: JSONValue | undefined): FailureCode {
  switch (value) {
    case "net::ERR_ABORTED": return "aborted";
    case "net::ERR_CONTENT_LENGTH_MISMATCH": return "content-length";
    case "net::ERR_CONTENT_DECODING_FAILED": return "decoding";
    case "net::ERR_CONNECTION_RESET": return "connection-reset";
    case "net::ERR_CONNECTION_CLOSED": return "connection-closed";
    case "net::ERR_EMPTY_RESPONSE": return "empty-response";
    default: return "unclassified";
  }
}
export type Terminal = "unreached" | "pending" | "finished" | "request-failed";
type Tracker = {
  request: Request | null; response: Response | null; terminal: Terminal; failureCode: FailureCode;
  ended: Promise<"finished" | "request-failed">; settle: (value: "finished" | "request-failed") => void;
};
function tracker(): Tracker {
  let settle: Tracker["settle"] = () => {};
  const ended = new Promise<"finished" | "request-failed">(resolve => { settle = resolve; });
  return { request: null, response: null, terminal: "unreached", failureCode: "none", ended, settle };
}
export function observeRestore(page: Page, origin: string, item: string, result: SafeCase) {
  const restore = tracker(), inspect = tracker();
  const base = origin + "/api/v1/subtitle-library/" + item;
  const refresh = () => {
    result.restoreRequestObserved = restore.request !== null;
    result.restoreResponseObserved = restore.response !== null;
    result.restoreTerminal = restore.terminal;
    result.restoreFailureCode = restore.failureCode;
    result.inspectRequestObserved = inspect.request !== null;
    result.inspectResponseObserved = inspect.response !== null;
    result.inspectTerminal = inspect.terminal;
    result.inspectFailureCode = inspect.failureCode;
  };
  const onRequest = (candidate: Request) => {
    if (candidate.method() === "POST" && candidate.url() === base + "/restore") {
      if (restore.request) { result.failureStage = "terminal"; return; }
      restore.request = candidate; restore.terminal = "pending";
    } else if (restore.request && !inspect.request && candidate.method() === "GET" && candidate.url() === base + "/inspect?language=en") {
      inspect.request = candidate; inspect.terminal = "pending";
    }
    refresh();
  };
  const onResponse = (candidate: Response) => {
    for (const observed of [restore, inspect]) if (candidate.request() === observed.request) observed.response = candidate;
    refresh();
  };
  const terminal = (candidate: Request, value: "finished" | "request-failed") => {
    for (const observed of [restore, inspect]) if (candidate === observed.request) {
      observed.terminal = value;
      observed.failureCode = value === "request-failed" ? requestFailureCode(candidate.failure()?.errorText) : "none";
      observed.settle(value);
    }
    refresh();
  };
  const finished = (request: Request) => terminal(request, "finished");
  const failed = (request: Request) => terminal(request, "request-failed");
  page.on("request", onRequest); page.on("response", onResponse);
  page.on("requestfinished", finished); page.on("requestfailed", failed);
  return {
    restore, inspect, close: () => {
      page.off("request", onRequest); page.off("response", onResponse);
      page.off("requestfinished", finished); page.off("requestfailed", failed);
    },
  };
}
