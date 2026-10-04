"""Strict safe receipt admission; no app, browser or external tool execution."""
import math
import re

SPECS = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts"]
PRIMARY = [f"visible Player Back preserves Movies query, offset, extent, focus and scroll at {width}px" for width in (390, 1440)]
COLLECTION = [(SPECS[0], title) for title in PRIMARY] + [
    (SPECS[0], "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow"),
    *[(SPECS[0], f"visible Player Back restores original Shows action via {action}") for action in ("direct Play", "details and episode")],
    *[(SPECS[1], f"cold native Back restores later Movie cards at {width}px") for width in (390, 1440)],
    (SPECS[1], "live query uses current URL rather than the document's initial browse key"),
    (SPECS[2], "native BFCache preserves loaded Movie DOM without repeated continuation"),
]
LABELS = ["Q14 acceptance: return to the same public browse URL", "Q14 acceptance: selected title action regains focus", "Q14 acceptance: same settled browse position", "fixture prerequisite:", "BFCache prerequisite:", "cold boundary prerequisite:", "HTMX prerequisite:"]
ATTACHMENTS = {"before-state", "player-state", "returned-state", "served-browse-asset"}


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
    patterns = {"view": r"movies|shows", "q": r"Return (Movie|Show)", "sort": r"title", "offset": r"\d{1,7}", "limit": r"\d{1,4}", "letter": r"[A-Z]", "lang": r"[a-z]{2}(-[A-Z]{2})?"}
    return isinstance(value, dict) and set(value) <= set(patterns) and all(match(item, patterns[key]) for key, item in value.items())


def failure(value):
    if not fields(value, "phase label location"):
        return False
    label = value["label"]
    if label is not None and label not in LABELS:
        return False
    expected = "acceptance" if label and label.startswith("Q14 acceptance:") else "prerequisite" if label else "unclassified"
    location = value["location"]
    return value["phase"] == expected and (location is None or (
        fields(location, "file line column") and location["file"] in [*SPECS, "browse-return-helpers.ts"]
        and integer(location["line"], 1, 1000) and integer(location["column"], 1, 1000)))


def observation(value, asset):
    if value is None:
        return True
    if asset:
        return (fields(value, "src bytes sha256") and match(value["src"], r"/static/main\.kinosail\.bundle\.js\?v=[a-zA-Z0-9._-]{1,80}")
                and integer(value["bytes"], 1, 10_000_000) and match(value["sha256"], r"[a-f0-9]{64}"))
    if not fields(value, "state peer"):
        return False
    state, peer = value["state"], value["peer"]
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


def attachment(value):
    return (fields(value, "name bytes sha256 observation") and isinstance(value["name"], str) and value["name"] in ATTACHMENTS
            and integer(value["bytes"], 1, 1_000_000) and match(value["sha256"], r"[a-f0-9]{64}")
            and observation(value["observation"], value["name"] == "served-browse-asset"))


def admit(report, collection=False):
    """Return only schema-validated known safe fields; arbitrary JSON becomes None."""
    if not fields(report, "schemaVersion status collected cases errors") or not integer(report["schemaVersion"], 1, 1):
        return None
    if report["status"] not in ("passed", "failed", "timedout", "interrupted"):
        return None
    if not isinstance(report["errors"], list) or len(report["errors"]) > 16 or not all(failure(item) for item in report["errors"]):
        return None
    expected = COLLECTION if collection else [(SPECS[0], title) for title in PRIMARY]
    collected = report["collected"]
    if not isinstance(collected, list) or len(collected) != len(expected) or not all(fields(item, "file title") and (item["file"], item["title"]) in expected for item in collected):
        return None
    if len({(item["file"], item["title"]) for item in collected}) != len(expected):
        return None
    cases = report["cases"]
    if not isinstance(cases, list) or len(cases) > (0 if collection else 2):
        return None
    for case in cases:
        if not fields(case, "file title status retry durationMs expectedStatus failures attachments") or (case["file"], case["title"]) not in expected:
            return None
        if case["status"] not in ("passed", "failed", "timedOut", "skipped", "interrupted") or not integer(case["retry"], 0, 0) or not integer(case["durationMs"], 0, 60_000) or case["expectedStatus"] != "passed":
            return None
        if not isinstance(case["failures"], list) or len(case["failures"]) > 16 or not all(failure(item) for item in case["failures"]):
            return None
        items = case["attachments"]
        if not isinstance(items, list) or len(items) > 4 or not all(attachment(item) for item in items) or len({item["name"] for item in items}) != len(items):
            return None
    if len({(item["file"], item["title"]) for item in cases}) != len(cases):
        return None
    return report


def complete(report, collection=False):
    if report is None or report["errors"]:
        return False
    if collection:
        return report["status"] == "passed" and not report["cases"]
    if len(report["cases"]) != 2 or report["status"] not in ("passed", "failed"):
        return False
    if (report["status"] == "passed") != all(case["status"] == "passed" for case in report["cases"]):
        return False
    for case in report["cases"]:
        if case["status"] not in ("passed", "failed", "timedOut"):
            return False
        if case["status"] == "passed" and (case["failures"] or {item["name"] for item in case["attachments"] if item["observation"] is not None} != ATTACHMENTS):
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
