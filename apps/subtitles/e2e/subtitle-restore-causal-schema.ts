import type { JSONValue, JSONObject } from "./subtitle-restore-recovery-fixture";
export type Native = { outcome: "unreached" | "pending" | "fulfilled" | "rejected"; status: number; signalPresent: boolean; signalAborted: boolean; pagehide: boolean };
export type Network = { terminal: "unreached" | "pending" | "finished" | "request-failed"; failureCode: string; resourceType: "unreached" | "Fetch" | "XHR" | "Document" | "Other"; cancelled: boolean; navigation: boolean };
export type Server = { seen: boolean; held: boolean; delivered: boolean; cancelled: boolean; timedOut: boolean; settled: boolean };
export type Mode = "direct" | "captured" | "held";
export type Completion = { scope: "r06-native-bodyless-204-v1"; caseID: string; browserVersion: "153.0.8010.12"; pinnedBrowserMatched: boolean; requestMatched: boolean; responseMatched: boolean; bodyless: boolean; fixtureMatched: boolean };
type Common = { valid: boolean; reason: "none" | "timeout" | "overflow" | "mismatch" | "control" | "unreached"; headersEqual: boolean; framingEqual: boolean; stopped: boolean; probes: { mode: Mode; native: Native; network: Network; server: Server }[]; restoreNative: Native; restoreNetwork: Network };
export type Diagnostic = Common & ({ schema: "r06-causal-v1" } | { schema: "r06-causal-v2"; completion: Completion });
function exact(value: JSONValue, keys: readonly string[]): value is JSONObject {
  return value !== null && typeof value === "object" && !Array.isArray(value) && Object.keys(value).sort().join(",") === [...keys].sort().join(",");
}
export function validNative(value: JSONValue): value is Native {
  return exact(value,["outcome","status","signalPresent","signalAborted","pagehide"]) &&
    typeof value.outcome === "string" && ["unreached","pending","fulfilled","rejected"].includes(value.outcome) &&
    Number.isSafeInteger(value.status) && typeof value.status === "number" && value.status >= 0 && value.status <= 599 &&
    ["signalPresent","signalAborted","pagehide"].every(key => typeof value[key] === "boolean");
}
export function validNetwork(value: JSONValue): value is Network {
  return exact(value,["terminal","failureCode","resourceType","cancelled","navigation"]) &&
    typeof value.terminal === "string" && ["unreached","pending","finished","request-failed"].includes(value.terminal) &&
    typeof value.failureCode === "string" && ["none","aborted","content-length","decoding","connection-reset","connection-closed","empty-response","unclassified"].includes(value.failureCode) &&
    (value.terminal === "request-failed") === (value.failureCode !== "none") &&
    typeof value.resourceType === "string" && ["unreached","Fetch","XHR","Document","Other"].includes(value.resourceType) &&
    typeof value.cancelled === "boolean" && typeof value.navigation === "boolean";
}
export function validServer(value: JSONValue): value is Server {
  return exact(value,["seen","held","delivered","cancelled","timedOut","settled"]) && Object.values(value).every(v => typeof v === "boolean");
}
export function validCausal(value: JSONValue): value is Diagnostic | null {
  if (value === null) return true;
  const fields = ["schema","valid","reason","headersEqual","framingEqual","stopped","probes","restoreNative","restoreNetwork"];
  if(!exact(value,fields) && !exact(value,[...fields,"completion"]))return false;
  const identity=value.schema === "r06-causal-v1" ? exact(value,fields) : value.schema === "r06-causal-v2" && validCompletion(value.completion);
  return identity && ["valid","headersEqual","framingEqual","stopped"].every(key => typeof value[key] === "boolean") &&
    typeof value.reason === "string" && ["none","timeout","overflow","mismatch","control","unreached"].includes(value.reason) &&
    Array.isArray(value.probes) && value.probes.length === 3 && value.probes.every((row,index) =>
      exact(row,["mode","native","network","server"]) && row.mode === ["direct","captured","held"][index] &&
      validNative(row.native) && validNetwork(row.network) && validServer(row.server)) &&
    validNative(value.restoreNative) && validNetwork(value.restoreNetwork);
}

export function validCompletion(value: JSONValue): value is Completion {
 return exact(value,["scope","caseID","browserVersion","pinnedBrowserMatched","requestMatched","responseMatched","bodyless","fixtureMatched"]) &&
  value.scope==="r06-native-bodyless-204-v1" && value.browserVersion==="153.0.8010.12" && typeof value.caseID==="string" &&
  ["r06-restore-headers-desktop","r06-restore-headers-phone","r06-restore-inspect-body-desktop","r06-restore-inspect-body-phone"].includes(value.caseID) &&
  ["pinnedBrowserMatched","requestMatched","responseMatched","bodyless","fixtureMatched"].every(k=>typeof value[k]==="boolean");
}
