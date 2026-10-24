import json
import sys
import time
import urllib.parse
import urllib.request

base = sys.argv[1]


def call(method, path):
    request = urllib.request.Request(base + path, method=method, headers={"Accept": "application/json"})
    with urllib.request.urlopen(request, timeout=30) as response:
        raw = response.read()
        return json.loads(raw) if raw else None


for _ in range(120):
    try:
        if call("GET", "/identity")["MediaContainer"].get("startState") is None:
            break
    except Exception:
        pass
    time.sleep(2)
else:
    raise SystemExit("Plex did not start")

names = {d["title"] for d in call("GET", "/library/sections")["MediaContainer"].get("Directory", [])}
for name, kind, agent, scanner, path in (("Movies", "movie", "tv.plex.agents.movie", "Plex Movie", "/data/movies"),
                                         ("Shows", "show", "tv.plex.agents.series", "Plex TV Series", "/data/shows")):
    if name not in names:
        query = urllib.parse.urlencode({"name": name, "type": kind, "agent": agent, "scanner": scanner, "language": "en-US", "location": path})
        call("POST", "/library/sections?" + query)

for attempt in range(90):
    sections = call("GET", "/library/sections")["MediaContainer"].get("Directory", [])
    counts = {}
    for section in sections:
        rows = call("GET", f"/library/sections/{section['key']}/all?includeGuids=1")["MediaContainer"].get("Metadata", [])
        counts[section["type"]] = rows
    movies = counts.get("movie", [])
    shows = counts.get("show", [])
    matched = [m for m in movies if m.get("Guid")]
    episodes = call("GET", f"/library/metadata/{shows[0]['ratingKey']}/allLeaves")["MediaContainer"].get("Metadata", []) if shows else []
    if len(matched) == 3 and len(episodes) == 3:
        print(f"plex ready: {len(movies)} movies, {len(shows)} show, {len(episodes)} episodes")
        break
    if attempt % 15 == 14:
        for section in sections:
            call("GET", f"/library/sections/{section['key']}/refresh")
    time.sleep(2)
else:
    raise SystemExit("Plex metadata did not finish")
