"""Strict safe receipt admission; no app, browser or external tool execution."""
import json
import math
import re

from campaign_q14_suites import ACCEPTANCE, PREREQUISITES, SAFETY_NAMES, SPECS as EXTENDED_SPECS, SUITES, ATTACHMENTS as EXTENDED_ATTACHMENTS, boundary, full_title

SPECS = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts"]
PRIMARY = [f"visible Player Back preserves Movies query, offset, extent, focus and scroll at {width}px" for width in (390, 1440)]
COLLECTION = [(SPECS[0], title) for title in PRIMARY] + [
    (SPECS[0], "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow"),
    *[(SPECS[0], f"visible Player Back restores original Shows action via {action}") for action in ("direct Play", "details and episode")],
    *[(SPECS[1], f"cold native Back restores later Movie cards at {width}px") for width in (390, 1440)],
    (SPECS[1], "live query uses current URL rather than the document's initial browse key"),
    (SPECS[2], "native BFCache preserves loaded Movie DOM without repeated continuation"),
]
LABELS = ACCEPTANCE + PREREQUISITES
ATTACHMENTS = {"before-state", "player-state", "returned-state", "served-browse-asset"}


def strict_report_json(raw):
    """Emitter keys are fixed; reject JSON ambiguity before schema admission."""
    def unique(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise ValueError("duplicate proof field")
            value[key] = item
        return value
    def nonfinite(_value):
        raise ValueError("nonfinite proof number")
    return json.loads(raw, object_pairs_hook=unique, parse_constant=nonfinite)


def fields(value, keys):
    return isinstance(value, dict) and set(value) == set(keys.split())


def integer(value, low=0, high=1_000_000):
    return type(value) is int and low <= value <= high


def match(value, pattern):
    return isinstance(value, str) and len(value) <= 512 and re.fullmatch(pattern, value) is not None


def route(value):
    return value is None or match(value, r"/(watch|show)/[a-f0-9]{16}")


def coordinate(value):
    return value is None or (type(value) in (int, float) and math.isfinite(value) and abs(value) <= 10_000_000)


def query(value):
    patterns = {"view": r"all|movies|shows", "q": r"Return (Movie|Show)", "sort": r"title", "offset": r"\d{1,7}", "limit": r"\d{1,4}", "letter": r"[A-Z]", "lang": r"[a-z]{2}(-[A-Z]{2})?"}
    return (isinstance(value, dict) and set(value) <= set(patterns) and all(match(item, patterns[key]) for key, item in value.items())
            and ("offset" not in value or 0 <= int(value["offset"]) <= 1_000_000)
            and ("limit" not in value or 1 <= int(value["limit"]) <= 200))


def failure(value):
    if not fields(value, "phase label location"):
        return False
    label = value["label"]
    if label is not None and label not in LABELS:
        return False
    expected = "acceptance" if label and label in ACCEPTANCE else "prerequisite" if label else "unclassified"
    location = value["location"]
    return value["phase"] == expected and (location is None or (
        fields(location, "file line column") and location["file"] in [*EXTENDED_SPECS, "browse-return-helpers.ts"]
        and integer(location["line"], 1, 1000) and integer(location["column"], 1, 1000)))


def observation(value, asset, extended=False, rejection=False):
    if value is None:
        return True
    if rejection:
        return (fields(value, "name href noBrowseRequests") and value["name"] in SAFETY_NAMES
                and route(value["href"]) and value["href"] is not None and type(value["noBrowseRequests"]) is bool)
    if asset:
        return (fields(value, "src bytes sha256") and match(value["src"], r"/static/main\.kinosail\.bundle\.js\?v=[a-zA-Z0-9._-]{1,80}")
                and integer(value["bytes"], 1, 10_000_000) and match(value["sha256"], r"[a-f0-9]{64}"))
    if not fields(value, "state peer"):
        return False
    state, peer = value["state"], value["peer"]
    if extended:
        state = extended_state(state)
        if state is None:
            return False
    if not fields(state, "path values profile titles hrefs focused scroll selected"):
        return False
    if not (state["path"] == "/" or route(state["path"])) or not query(state["values"]) or state["profile"] not in ("local-owner", None):
        return False
    if not isinstance(state["titles"], list) or len(state["titles"]) > 64 or not all(match(title, r"Return (Movie|Show) \d{2}|Anchor Movie|Zeta Movie") for title in state["titles"]):
        return False
    if not isinstance(state["hrefs"], list) or len(state["hrefs"]) > 64 or not all(route(item) for item in state["hrefs"]) or not route(state["focused"]):
        return False
    scroll, selected = state["scroll"], state["selected"]
    if not fields(scroll, "x y") or not all(coordinate(item) for item in scroll.values()):
        return False
    if selected is not None and not (fields(selected, "href top bottom") and route(selected["href"]) and coordinate(selected["top"]) and coordinate(selected["bottom"])):
        return False
    return isinstance(peer, list) and len(peer) <= 256 and all(fields(item, "values continuation history htmx") and query(item["values"]) and all(type(item[key]) is bool for key in ("continuation", "history", "htmx")) for item in peer)


def extended_state(state):
    if not fields(state, "path values profile titles hrefs focused scroll selected document navigation focusedBrowse"):
        return None
    doc, nav, focused, selected = state["document"], state["navigation"], state["focusedBrowse"], state["selected"]
    if not (fields(doc, "idSHA256 shows histories") and match(doc["idSHA256"], r"[a-f0-9]{64}")
            and integer(doc["histories"], 0, 256) and isinstance(doc["shows"], list) and len(doc["shows"]) <= 16
            and all(fields(item, "persisted") and type(item["persisted"]) is bool for item in doc["shows"])):
        return None
    if not isinstance(nav, list) or len(nav) > 4 or not all(item in ("navigate", "reload", "back_forward", "prerender") for item in nav):
        return None
    if focused is not None and (not query(focused) or state["focused"] is not None):
        return None
    if not fields(state["scroll"], "x y") or not all(type(item) in (int, float) and coordinate(item) for item in state["scroll"].values()):
        return None
    stripped = {key: value for key, value in state.items() if key not in ("document", "navigation", "focusedBrowse")}
    if selected is not None:
        if not fields(selected, "href browse top bottom") or not all(type(selected[k]) in (int, float) and coordinate(selected[k]) for k in ("top", "bottom")):
            return None
        if selected["browse"] is not None and (not query(selected["browse"]) or selected["href"] is not None):
            return None
        stripped["selected"] = {key: value for key, value in selected.items() if key != "browse"}
    return stripped


def attachment(value, extended=False):
    return (fields(value, "name bytes sha256 observation") and isinstance(value["name"], str) and value["name"] in (EXTENDED_ATTACHMENTS if extended else ATTACHMENTS)
            and integer(value["bytes"], 1, 1_000_000) and match(value["sha256"], r"[a-f0-9]{64}")
            and observation(value["observation"], value["name"] == "served-browse-asset", extended, value["name"] == "safe-rejection"))


def admit(report, collection=False, suite="primary", project="chromium"):
    """Return only schema-validated known safe fields; arbitrary JSON becomes None."""
    identified = isinstance(report, dict) and report.get("schemaVersion") == 3
    extended = isinstance(report, dict) and report.get("schemaVersion") in (2, 3)
    keys = "schemaVersion status collected cases errors" + (" suite" if extended else "") + (" project" if identified else "")
    if not fields(report, keys) or not integer(report["schemaVersion"], 1, 3) or not isinstance(suite, str) or suite not in SUITES or suite == "all":
        return None
    if identified and (project not in ("chromium", "firefox", "webkit") or report["project"] != project):
        return None
    selected = "all" if collection and suite == "primary" else suite
    if extended and report["suite"] != selected or not extended and suite != "primary":
        return None
    if report["status"] not in ("passed", "failed", "timedout", "interrupted"):
        return None
    if not isinstance(report["errors"], list) or len(report["errors"]) > 16 or not all(failure(item) for item in report["errors"]):
        return None
    expected = SUITES[selected] if extended else COLLECTION if collection else [(SPECS[0], title) for title in PRIMARY]
    collected = report["collected"]
    if not isinstance(collected, list) or len(collected) != len(expected) or not all(fields(item, "file title" + (" fullTitle" if identified else "")) and (item["file"], item["title"]) in expected and (not identified or item["fullTitle"] == full_title(item["file"], item["title"])) for item in collected):
        return None
    if len({(item["file"], item["title"]) for item in collected}) != len(expected):
        return None
    cases = report["cases"]
    if not isinstance(cases, list) or len(cases) > (0 if collection else len(expected)):
        return None
    for case in cases:
        if not fields(case, "file title status retry durationMs expectedStatus failures attachments" + (" fullTitle" if identified else "")) or (case["file"], case["title"]) not in expected or (identified and case["fullTitle"] != full_title(case["file"], case["title"])):
            return None
        if case["status"] not in ("passed", "failed", "timedOut", "skipped", "interrupted") or not integer(case["retry"], 0, 0) or not integer(case["durationMs"], 0, 60_000) or case["expectedStatus"] != "passed":
            return None
        if not isinstance(case["failures"], list) or len(case["failures"]) > 16 or not all(failure(item) for item in case["failures"]):
            return None
        items = case["attachments"]
        if not isinstance(items, list) or len(items) > (16 if extended else 4) or not all(attachment(item, extended) for item in items) or len({item["name"] for item in items}) != len(items):
            return None
    if len({(item["file"], item["title"]) for item in cases}) != len(cases):
        return None
    return report


def complete(report, collection=False, suite="primary", project="chromium"):
    if report is None or admit(report, collection, suite, project) is None or report["errors"]:
        return False
    if collection:
        return report["status"] == "passed" and not report["cases"]
    if len(report["cases"]) != len(SUITES[suite]) or report["status"] not in ("passed", "failed"):
        return False
    if (report["status"] == "passed") != all(case["status"] == "passed" for case in report["cases"]):
        return False
    for case in report["cases"]:
        if case["status"] not in ("passed", "failed", "timedOut"):
            return False
        if case["status"] == "passed":
            observed = boundary(suite, case) if report["schemaVersion"] in (2, 3) else {item["name"] for item in case["attachments"] if item["observation"] is not None} == ATTACHMENTS
            if case["failures"] or not observed:
                return False
        if case["status"] != "passed" and not case["failures"]:
            return False
    return True


def go_boundary(raw):
    """A Playwright exit alone cannot certify the Go fixture completed normally."""
    passed = re.findall(rb"(?m)^--- PASS: TestBrowseReturnBrowserJourney \([\d.]+s\)$", raw)
    failed = re.findall(rb"(?m)^--- FAIL: TestBrowseReturnBrowserJourney \([\d.]+s\)$", raw)
    if re.search(rb"(?m)^(panic:|fatal error:|FAIL\s+.*\[build failed\])", raw):
        return "incomplete"
    if len(passed) == 1 and not failed and re.search(rb"(?m)^PASS\r?$", raw):
        return "completed-pass"
    if len(failed) == 1 and not passed and re.search(rb"(?m)^FAIL\r?$", raw):
        return "completed-fail"
    return "incomplete"


CACHE_REASONS = ["unload-listener","unload-handler","response-cache-control-no-store","response-cache-control-no-store-with-cookie-modification","related-active-contents","masked","websocket","outstanding-network-request","other"]


def cache_diagnostic(raw):
    """Admit bounded reason codes only; never copy frame metadata or error text."""
    lines = [line[21:] for line in raw.splitlines() if line.startswith(b"Q14_CACHE_DIAGNOSTIC ")]
    if len(lines) != 1 or len(lines[0]) > 4096:
        return None
    try:
        value = json.loads(lines[0])
    except (ValueError, TypeError):
        return None
    if not fields(value, "schemaVersion supported present frameCount reasons truncated navigationType"):
        return None
    if type(value["schemaVersion"]) is not int or value["schemaVersion"] != 1:
        return None
    if not all(type(value[key]) is bool for key in ("supported", "present", "truncated")) or not integer(value["frameCount"], 0, 64):
        return None
    reasons = value["reasons"]
    if not isinstance(reasons, list) or len(reasons) > 16 or not all(isinstance(reason, str) and reason in CACHE_REASONS for reason in reasons):
        return None
    if reasons != sorted(set(reasons)) or value["navigationType"] not in ("navigate", "reload", "back_forward", "prerender", "unknown"):
        return None
    if value["present"] and (not value["supported"] or value["frameCount"] < 1):
        return None
    if not value["present"] and (value["frameCount"] != 0 or reasons):
        return None
    return value
