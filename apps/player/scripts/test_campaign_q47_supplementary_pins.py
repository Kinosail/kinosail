"""Pure RAM controls for fixed fictional Q47 supplemental output bytes."""
import hashlib
import unittest
from campaign_q47_admission import NAMES
from campaign_q47_supplementary_pins import MARKER, output_pins, rendered


TEMPLATES = {
    "player": """name: kinosail-player

services:
  kinosail:
    image: ghcr.io/kinosail/kinosail-player:latest
    restart: unless-stopped
    stop_grace_period: 30s
    init: true
    user: "10001:10001"
    pids_limit: 256
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    ports:
      - "38127:38127"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
      KINOSAIL_BACKUP_DIR: /backups
    volumes:
      - player-config:/config
      - player-cache:/cache
      - player-backups:/backups
      - type: bind
        source: "${KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"
        target: /media
        read_only: true
        bind:
          create_host_path: false
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
      - /run:rw,noexec,nosuid,nodev,size=16m
    healthcheck:
      test: ["CMD", "kinosail", "healthcheck"]
      interval: 30s
      timeout: 15s
      retries: 3

volumes:
  player-config:
  player-cache:
  player-backups:
""",
    "subtitles": """name: kinosail-subtitles

services:
  kinosail:
    image: ghcr.io/kinosail/kinosail-subtitles:latest
    restart: unless-stopped
    stop_grace_period: 30s
    init: true
    user: "10001:10001"
    pids_limit: 256
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    ports:
      - "38128:38128"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
      KINOSAIL_BACKUP_DIR: /backups
    volumes:
      - subtitles-config:/config
      - subtitles-cache:/cache
      - subtitles-backups:/backups
      - type: bind
        source: "${KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"
        target: /media
        read_only: false
        bind:
          create_host_path: false
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
      - /run:rw,noexec,nosuid,nodev,size=16m
    healthcheck:
      test: ["CMD", "kinosail", "healthcheck"]
      interval: 30s
      timeout: 15s
      retries: 3

volumes:
  subtitles-config:
  subtitles-cache:
  subtitles-backups:
""",
    "both": """name: kinosail-both

services:
  player:
    image: ghcr.io/kinosail/kinosail-player:latest
    restart: unless-stopped
    stop_grace_period: 30s
    init: true
    user: "10001:10001"
    pids_limit: 256
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    ports:
      - "38127:38127"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
      KINOSAIL_BACKUP_DIR: /backups
    volumes:
      - player-config:/config
      - player-cache:/cache
      - player-backups:/backups
      - type: bind
        source: "${KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"
        target: /media
        read_only: true
        bind:
          create_host_path: false
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
      - /run:rw,noexec,nosuid,nodev,size=16m
    healthcheck:
      test: ["CMD", "kinosail", "healthcheck"]
      interval: 30s
      timeout: 15s
      retries: 3

  subtitles:
    image: ghcr.io/kinosail/kinosail-subtitles:latest
    restart: unless-stopped
    stop_grace_period: 30s
    init: true
    user: "10001:10001"
    pids_limit: 256
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    ports:
      - "38128:38128"
    environment:
      KINOSAIL_DATA_DIR: /config
      KINOSAIL_CACHE_DIR: /cache
      KINOSAIL_MEDIA_DIR: /media
      KINOSAIL_BACKUP_DIR: /backups
    volumes:
      - subtitles-config:/config
      - subtitles-cache:/cache
      - subtitles-backups:/backups
      - type: bind
        source: "${KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"
        target: /media
        read_only: false
        bind:
          create_host_path: false
    tmpfs:
      - /tmp:rw,noexec,nosuid,nodev,size=256m
      - /run:rw,noexec,nosuid,nodev,size=16m
    healthcheck:
      test: ["CMD", "kinosail", "healthcheck"]
      interval: 30s
      timeout: 15s
      retries: 3

volumes:
  player-config:
  player-cache:
  player-backups:
  subtitles-config:
  subtitles-cache:
  subtitles-backups:
""",
}


GOLDEN = {
    "Q47 lost request recovers with retained inputs": {"bytes": 2023, "sha256": "9dceb1e58b9c6e36564dac30651696c92c1df6f5b14b00b763e69c276bf75b5a"},
    "Q47 failed HTTP recovers with retained inputs": {"bytes": 2023, "sha256": "9dceb1e58b9c6e36564dac30651696c92c1df6f5b14b00b763e69c276bf75b5a"},
    "Q47 input change discards late headers": {"bytes": 1025, "sha256": "c1d92e1d81ad0d2ae6a8a77862f24f54326fd4079e401432670eaf712855a699"},
    "Q47 app change discards late body": {"bytes": 1048, "sha256": "9b9c26ac7992d9630779853ddf6a1d64e7509258a46646e180b59a98882faccc"},
    "Q47 Player preserves Compose download and copy": {"bytes": 1023, "sha256": "0cab306815944517b179df11f6bd53abd6a19017e79876fe0fc2b2c6cf54088b"},
    "Q47 Subtitles preserves Compose download and copy": {"bytes": 1048, "sha256": "9b9c26ac7992d9630779853ddf6a1d64e7509258a46646e180b59a98882faccc"},
    "Q47 Both preserves Compose download and copy": {"bytes": 2023, "sha256": "9dceb1e58b9c6e36564dac30651696c92c1df6f5b14b00b763e69c276bf75b5a"},
}
FIXTURE_PINS = [["player",1062,"5c1ad7f847fb7255d1e1a7fca2a9ef244cdc466fb8739b42c5438395eca95776"],["subtitles",1087,"a4599be72882509f250835af354f3df6af6ac305e808659a40b8315ddabb273e"],["both",2101,"0c08d9e2ffacaedbc0c2afc12cb0562500b18deab50655691ccd570c4fce5601"]]


class DictSubclass(dict):
    pass


class StrSubclass(str):
    pass


class SupplementaryPinsTests(unittest.TestCase):
    def test_exact_fictional_templates(self):
        for app, size, sha256 in FIXTURE_PINS:
            with self.subTest(app=app):
                data = TEMPLATES[app].encode("utf-8")
                self.assertEqual(len(data), size)
                self.assertEqual(hashlib.sha256(data).hexdigest(), sha256)

    def test_fixed_inventory_and_golden_bytes(self):
        pins = output_pins(TEMPLATES)
        self.assertEqual(pins, GOLDEN)
        self.assertNotIn(NAMES["contracts"][3], pins)
        for pin in pins.values():
            self.assertEqual(set(pin), {"bytes", "sha256"})
            self.assertIs(type(pin["bytes"]), int)
            self.assertRegex(pin["sha256"], r"^[0-9a-f]{64}$")

    def test_recovery_and_supersession_use_current_fields(self):
        pins = output_pins(TEMPLATES)
        self.assertEqual(pins[NAMES["recovery"][0]], pins[NAMES["contracts"][2]])
        self.assertEqual(pins[NAMES["recovery"][1]], pins[NAMES["contracts"][2]])
        self.assertEqual(pins[NAMES["supersession"][1]], pins[NAMES["contracts"][1]])
        self.assertNotEqual(pins[NAMES["supersession"][0]], pins[NAMES["contracts"][0]])

    def test_strict_three_template_inventory(self):
        invalid = [None, [], "player", DictSubclass(TEMPLATES),
                   {key: value for key, value in TEMPLATES.items() if key != "both"},
                   {**TEMPLATES, "other": TEMPLATES["player"]}]
        for value in invalid:
            with self.subTest(value_type=type(value).__name__):
                with self.assertRaisesRegex(ValueError, "^template_pin_inventory$"):
                    output_pins(value)

    def test_strict_template_type_empty_and_byte_cap(self):
        for app in TEMPLATES:
            for value in (None, b"template", "", StrSubclass(TEMPLATES[app]), "x" * 16385):
                with self.subTest(app=app, value_type=type(value).__name__):
                    with self.assertRaisesRegex(ValueError, "^template_pin_bytes$"):
                        output_pins({**TEMPLATES, app: value})

    def test_exact_byte_limit_and_multibyte_overflow(self):
        for app, template in TEMPLATES.items():
            at_limit = template + " " * (16384 - len(template.encode()))
            self.assertEqual(len(at_limit.encode()), 16384)
            self.assertEqual(set(output_pins({**TEMPLATES, app: at_limit})), set(GOLDEN))
            over_limit = template + "\u00e9" * ((16384 - len(template.encode())) // 2 + 1)
            self.assertLessEqual(len(over_limit), 16384)
            with self.assertRaisesRegex(ValueError, "^template_pin_bytes$"):
                output_pins({**TEMPLATES, app: over_limit})

    def test_exact_media_marker_cardinality(self):
        for app, template in TEMPLATES.items():
            self.assertEqual(template.count(MARKER), 2 if app == "both" else 1)
            for value in (template.replace(MARKER, ""), template.replace(MARKER, MARKER * 2)):
                with self.subTest(app=app):
                    with self.assertRaisesRegex(ValueError, "^template_pin_marker$"):
                        output_pins({**TEMPLATES, app: value})
        with self.assertRaisesRegex(ValueError, "^template_pin_marker$"):
            output_pins({**TEMPLATES, "both": TEMPLATES["both"].replace(MARKER, "", 1)})

    def test_port_replacement_matches_browser_first_match(self):
        for app, first in (("player", "38127"), ("subtitles", "38128"), ("both", "38127")):
            markers = MARKER + "\n" + (MARKER + "\n" if app == "both" else "")
            ports = '- "' + first + ':' + first + '"\n'
            source = markers + ports * 2
            expected = ('source: "/fictional/q47/media"\n' * (2 if app == "both" else 1)
                        + '- "49127:' + first + '"\n' + ports)
            if app == "both":
                source += '- "38128:38128"\n' * 2
                expected += '- "49128:38128"\n- "38128:38128"\n'
            self.assertEqual(rendered(source, app, "/fictional/q47/media", "49127", "49128"),
                             {"bytes": len(expected.encode()), "sha256": hashlib.sha256(expected.encode()).hexdigest()})


if __name__ == "__main__":
    unittest.main()
