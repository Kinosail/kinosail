"""Fixed public journey identities and boundary acceptance; no runtime execution."""
SPECS = ["browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts",
         "browse-return-safety.spec.ts", "browse-return-home.spec.ts", "watch-navigation.spec.ts"]
PRIMARY = [(SPECS[0], f"visible Player Back preserves Movies query, offset, extent, focus and scroll at {width}px") for width in (390, 1440)]
SAFETY_NAMES = ["external origin", "protocol-relative origin", "non-browse route", "duplicate query", "unknown query",
                "oversized query", "excessive extent", "different profile", "different destination"]
SUITES = {
    "primary": PRIMARY,
    "navigation": [(SPECS[5], f"Player has one accessible return link with Movies context and a direct-entry fallback at {width}px") for width in (390, 1440, 1920)]
        + [(SPECS[5], title) for title in ("Home keeps its exact return after Mark watched", "Plain root keeps its exact return after Mark watched")],
    "cold": [(SPECS[1], f"cold native Back restores later Movie cards at {width}px") for width in (390, 1440)],
    "bfcache": [(SPECS[2], "native BFCache preserves loaded Movie DOM without repeated continuation")],
    "htmx": [(SPECS[0], "HTMX title-letter Back fetches current browse data and restores extent without a native pageshow")],
    "shows": [(SPECS[0], f"visible Player Back restores original Shows action via {action}") for action in ("direct Play", "details and episode")],
    "search": [(SPECS[1], "live query uses current URL rather than the document's initial browse key")],
    "safety": [(SPECS[3], f"saved return rejects {name} before navigation or continuation") for name in SAFETY_NAMES]
        + [(SPECS[3], "a direct Player opened in another tab does not inherit browse return state")],
    "home": [(SPECS[4], title) for title in (
        "visible Player Back restores the Home Continue watching action without a Library grid",
        "cold native Back restores the Home Continue watching action without a Library grid",
        "cold native Back restores the scrolled Home Movies destination without a Library grid")],
}
SUITES["all"] = PRIMARY + SUITES["htmx"] + SUITES["shows"] + SUITES["cold"] + SUITES["search"] + SUITES["bfcache"]
BASE = {"before-state", "player-state", "returned-state", "served-browse-asset"}
HOME = {"home-before-state", "home-returned-state", "served-browse-asset"}
ATTACHMENTS = BASE | HOME | {"letter-state", "original-url-state", "cold-boundary-state", "native-cache-boundary-state",
                           "htmx-boundary-state", "home-player-state", "home-cold-boundary-state", "safe-rejection",
                           "movies-player-state", "direct-player-state", "home-watched-return-state", "root-watched-return-state"}
ACCEPTANCE = ["Q14 acceptance: return to the same public browse URL", "Q14 acceptance: selected title action regains focus",
              "Q14 acceptance: same settled browse position", "Q14 Home acceptance: original action regains focus",
              "Q14 Home acceptance: settled horizontal position", "Q14 Home acceptance: settled vertical position"]
PREREQUISITES = ["fixture prerequisite:", "BFCache prerequisite:", "cold boundary prerequisite:", "cold search prerequisite:",
                 "HTMX prerequisite:", "Home prerequisite:", "Home cold prerequisite:", "destination prerequisite:"]


def required(suite, title):
    if suite == "navigation":
        return {"movies-player-state", "direct-player-state"} if "accessible return link" in title else {"home-watched-return-state" if title.startswith("Home") else "root-watched-return-state"}
    if suite in ("primary", "shows"):
        return BASE
    if suite in ("cold", "bfcache"):
        return BASE | {"cold-boundary-state" if suite == "cold" else "native-cache-boundary-state"}
    if suite == "htmx":
        return (BASE - {"player-state"}) | {"letter-state", "htmx-boundary-state"}
    if suite == "search":
        return {"original-url-state", "before-state", "cold-boundary-state", "returned-state"}
    if suite == "safety":
        return (BASE - {"returned-state"}) | ({"safe-rejection"} if title.startswith("saved return rejects ") else set())
    if suite == "home":
        return HOME | ({"home-player-state"} if "Continue watching" in title else set()) | ({"home-cold-boundary-state"} if title.startswith("cold ") else set())
    return set()


def native_boundary(before, returned, cold):
    original, current = before["state"], returned["state"]
    a, b = original["document"], current["document"]
    if not a["idSHA256"] or not b["idSHA256"] or not b["shows"]:
        return False
    if cold:
        return a["idSHA256"] != b["idSHA256"] and b["shows"][-1]["persisted"] is False and "back_forward" in current["navigation"]
    return (a["idSHA256"] == b["idSHA256"] and b["shows"][-1]["persisted"] is True
            and returned["peer"] == before["peer"])


def boundary(suite, case):
    """Assertions remain authoritative; additionally reject contradictory typed proof."""
    rows = {item["name"]: item["observation"] for item in case["attachments"]}
    if any(value is None for value in rows.values()) or set(rows) != required(suite, case["title"]):
        return False
    if suite == "navigation":
        states = [row["state"] for row in rows.values()]
        return (all(state["profile"] == "local-owner" and state["path"].startswith("/watch/") and not state["values"] for state in states)
                and len({state["path"] for state in states}) == 1)
    if suite == "safety":
        if "safe-rejection" not in rows:
            return True
        rejection = rows["safe-rejection"]
        return (rejection["noBrowseRequests"] is True and case["title"] == f"saved return rejects {rejection['name']} before navigation or continuation"
                and rejection["href"] in rows["before-state"]["state"]["hrefs"])
    before = rows["home-before-state" if suite == "home" else "before-state"]
    returned = rows["home-returned-state" if suite == "home" else "returned-state"]
    a, b = before["state"], returned["state"]
    if a["profile"] != "local-owner" or b["profile"] != a["profile"] or a["path"] != b["path"] or a["values"] != b["values"]:
        return False
    if any(abs(a["scroll"][axis] - b["scroll"][axis]) > 2 for axis in ("x", "y")):
        return False
    if suite != "home" and (a["titles"] != b["titles"] or a["hrefs"] != b["hrefs"] or len(set(b["hrefs"])) != len(b["hrefs"])):
        return False
    if (a["focused"], a["focusedBrowse"]) != (b["focused"], b["focusedBrowse"]):
        return False
    if suite in ("cold", "search"):
        if not native_boundary(before, rows["cold-boundary-state"], True):
            return False
    if suite == "bfcache" and not native_boundary(before, rows["native-cache-boundary-state"], False):
        return False
    if suite == "htmx":
        value = rows["htmx-boundary-state"]
        doc = value["state"]["document"]
        if (doc["idSHA256"] != a["document"]["idSHA256"] or doc["shows"] != a["document"]["shows"]
                or doc["histories"] <= rows["letter-state"]["state"]["document"]["histories"]
                or not any(item["history"] for item in value["peer"][len(rows["letter-state"]["peer"]):])):
            return False
    if suite == "search":
        original = rows["original-url-state"]["state"]
        if original["document"]["idSHA256"] != a["document"]["idSHA256"] or original["values"] == a["values"]:
            return False
    if suite == "home" and "home-cold-boundary-state" in rows:
        value = rows["home-cold-boundary-state"]
        if not native_boundary(before, value, True) or not any(not item["continuation"] and not item["history"] for item in value["peer"][len(before["peer"]):]):
            return False
        if "Movies destination" in case["title"] and any(item["continuation"] for item in returned["peer"][len(before["peer"]):]):
            return False
    return True
