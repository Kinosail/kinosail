"""Expected bytes for fixed disposable Q47 secondary cases; no runtime IO."""
import hashlib
import json
from campaign_q47_admission import NAMES
MARKER = 'source: "$' + '{KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"'
def rendered(template, app, media, port, second_port):
    if type(template) is not str or not 0 < len(template.encode()) <= 16384:
        raise ValueError("template_pin_bytes")
    if template.count(MARKER) != (2 if app == "both" else 1):
        raise ValueError("template_pin_marker")
    first = "38128" if app == "subtitles" else "38127"
    value = template.replace(MARKER, "source: " + json.dumps(media))
    value = value.replace('- "' + first + ":" + first + '"', '- "' + port + ":" + first + '"', 1)
    if app == "both":
        value = value.replace('- "38128:38128"', '- "' + second_port + ':38128"', 1)
    data = value.encode()
    return {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
def output_pins(templates):
    if type(templates) is not dict or set(templates) != {"player", "subtitles", "both"}:
        raise ValueError("template_pin_inventory")
    defaults = {app: rendered(templates[app], app, "/fictional/q47/media", "49128" if app == "subtitles" else "49127", "49128") for app in ("player", "subtitles", "both")}
    result = {name: defaults["both"] for name in NAMES["recovery"]}
    result[NAMES["supersession"][0]] = rendered(templates["player"], "player", "/fictional/q47/changed", "50127", "38128")
    result[NAMES["supersession"][1]] = defaults["subtitles"]
    result.update({name: defaults[app] for name, app in zip(NAMES["contracts"][:3], ("player", "subtitles", "both"), strict=True)})
    return result
