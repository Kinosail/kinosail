import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1]
product = sys.argv[2]
auth = 'MediaBrowser Client="KinoTest", Device="Mac", DeviceId="kinotest-setup", Version="1.0"'


def call(method, path, body=None, token=None, ok=(200, 204)):
    data = None if body is None else json.dumps(body).encode()
    request = urllib.request.Request(base + path, data=data, method=method)
    header = auth + (f', Token="{token}"' if token else "")
    request.add_header("Authorization", header)
    request.add_header("X-Emby-Authorization", header)
    if token:
        request.add_header("X-Emby-Token", token)
    if data is not None:
        request.add_header("Content-Type", "application/json")
    for attempt in range(90):
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                raw = response.read()
                return json.loads(raw) if raw else None
        except urllib.error.HTTPError as error:
            if error.code in ok:
                return None
            if error.code == 503 and attempt < 89:
                time.sleep(2)
                continue
            raise RuntimeError(f"{method} {path} -> {error.code}")


for _ in range(120):
    try:
        info = call("GET", "/System/Info/Public")
        if info:
            break
    except Exception:
        pass
    time.sleep(1)
credentials = {"Username": "kino", "Pw": "kino-test-password", "Password": "kino-test-password"}
try:
    login = call("POST", "/Users/AuthenticateByName", credentials)
except RuntimeError:
    login = None
if not login:
    call("POST", "/Startup/Configuration", {"UICulture": "en-US", "MetadataCountryCode": "US", "PreferredMetadataLanguage": "en"})
    call("GET", "/Startup/User")
    call("POST", "/Startup/User", {"Name": "kino", "Password": "kino-test-password"})
    call("POST", "/Startup/RemoteAccess", {"EnableRemoteAccess": True, "EnableAutomaticPortMapping": False})
    call("POST", "/Startup/Complete")
    login = call("POST", "/Users/AuthenticateByName", credentials)
token = login["AccessToken"]
existing = call("GET", "/Library/VirtualFolders", token=token) or []
names = {folder["Name"] for folder in existing}
for name, kind, path in (("Movies", "movies", "/media/movies"), ("Shows", "tvshows", "/media/shows")):
    if name in names:
        continue
    fetchers = ["TheMovieDb", "TheTVDB"] if kind == "tvshows" else ["TheMovieDb"]
    types = ["Series", "Season", "Episode"] if kind == "tvshows" else ["Movie"]
    options = {"PathInfos": [{"Path": path}]}
    if product == "emby":
        options["TypeOptions"] = [{"Type": t, "MetadataFetchers": fetchers, "ImageFetchers": fetchers} for t in types]
    call("POST", f"/Library/VirtualFolders?name={name}&collectionType={kind}&refreshLibrary=true",
         {"LibraryOptions": options}, token=token)
call("POST", "/Library/Refresh", token=token)
user = login["User"]["Id"]
for _ in range(120):
    items = (call("GET", f"/Users/{user}/Items?Recursive=true&IncludeItemTypes=Movie,Series,Episode&Fields=ProviderIds",
                  token=token) or {}).get("Items", [])
    movies = [i for i in items if i["Type"] == "Movie"]
    episodes = [i for i in items if i["Type"] == "Episode"]
    series = [i for i in items if i["Type"] == "Series"]
    listed = series and len((call("GET", f"/Shows/{series[0]['Id']}/Episodes?UserId={user}", token=token) or {}).get("Items", [])) == 3
    named = len(movies) == 3 and len(episodes) == 3 and all(i.get("ProviderIds", {}).get("Tmdb") for i in movies) \
        and not any(i["Name"].startswith("Pioneer One - ") for i in episodes)
    if named and listed:
        break
    if named and _ % 6 == 5:
        call("POST", "/Library/Refresh", token=token)
    time.sleep(5)
else:
    raise RuntimeError(f"{product} metadata is incomplete; check that the container can reach TheMovieDb")
print("ready", product, "user", login["User"]["Id"])
