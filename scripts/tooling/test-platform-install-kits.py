#!/usr/bin/env python3
"""Check importable container kits without pulling images or starting services."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import unittest
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[2]


def compose_command():
    if shutil.which("docker-compose"):
        return ["docker-compose"]
    if shutil.which("docker") and subprocess.run(
        ["docker", "compose", "version"], capture_output=True, check=False, timeout=10,
    ).returncode == 0:
        return ["docker", "compose"]
    raise RuntimeError("Docker Compose is required to validate platform install kits")


class PlatformInstallKitsTest(unittest.TestCase):
    def compose(self, app):
        path = ROOT / "apps" / app / "packaging" / "platform-compose.yaml"
        self.assertIn("create_host_path: false", path.read_text())
        media = ROOT / "apps" / app / "packaging"
        result = subprocess.run(
            [*compose_command(), "-f", str(path), "config", "--format", "json"],
            capture_output=True, text=True, check=True, timeout=30,
            env=os.environ | {"KINOSAIL_MEDIA_PATH": str(media)},
        )
        config = json.loads(result.stdout)
        self.assertEqual(config["name"], f"kinosail-{app}")
        return config["services"]["kinosail"]

    def test_missing_media_path_rejects_before_deployment(self):
        env = os.environ.copy()
        env.pop("KINOSAIL_MEDIA_PATH", None)
        for app in ("player", "subtitles", "both"):
            with self.subTest(app=app):
                path = ROOT / "apps" / ("player" if app == "both" else app) / "packaging" / f"platform-compose{'-both' if app == 'both' else ''}.yaml"
                result = subprocess.run(
                    [*compose_command(), "-f", str(path), "config", "--quiet"],
                    capture_output=True, text=True, check=False, timeout=30, env=env,
                )
                self.assertNotEqual(result.returncode, 0)

    def test_both_compose_keeps_each_app_contract(self):
        path = ROOT / "apps/player/packaging/platform-compose-both.yaml"
        result = subprocess.run(
            [*compose_command(), "-f", str(path), "config", "--format", "json"],
            capture_output=True, text=True, check=True, timeout=30,
            env=os.environ | {"KINOSAIL_MEDIA_PATH": str(ROOT / "apps/player/packaging")},
        )
        config = json.loads(result.stdout)
        self.assertEqual(config["name"], "kinosail-both")
        self.assertEqual(set(config["services"]), {"player", "subtitles"})
        self.assertEqual(len(config["volumes"]), 6)
        for app, port, read_only in (("player", 38127, True), ("subtitles", 38128, False)):
            service = config["services"][app]
            standalone = self.compose(app)
            self.assertEqual(service["image"], f"ghcr.io/kinosail/kinosail-{app}:latest")
            self.assertEqual(service["user"], "10001:10001")
            self.assertTrue(service["read_only"])
            self.assertTrue(service["init"])
            self.assertEqual(service["restart"], "unless-stopped")
            self.assertEqual(service["pids_limit"], 256)
            self.assertEqual(service["cap_drop"], ["ALL"])
            self.assertIn("no-new-privileges:true", service["security_opt"])
            self.assertEqual(service["healthcheck"], standalone["healthcheck"])
            self.assertEqual(service["environment"], standalone["environment"])
            self.assertEqual(service["tmpfs"], standalone["tmpfs"])
            self.assertEqual(service["ports"][0]["target"], port)
            self.assertEqual(service["ports"][0]["published"], str(port))
            mounts = {volume["target"]: volume for volume in service["volumes"]}
            self.assertEqual(set(mounts), {"/config", "/cache", "/backups", "/media"})
            self.assertEqual(mounts["/media"]["source"], str(ROOT / "apps/player/packaging"))
            self.assertEqual(mounts["/media"].get("read_only", False), read_only)
            self.assertFalse(mounts["/media"].get("bind", {}).get("create_host_path", False))
            for target, name in (("/config", "config"), ("/cache", "cache"), ("/backups", "backups")):
                self.assertTrue(mounts[target]["source"].endswith(f"{app}-{name}"))

    def test_compose_imports_keep_media_and_state_separate(self):
        for app, port, access in (("player", 38127, True), ("subtitles", 38128, False)):
            with self.subTest(app=app):
                service = self.compose(app)
                self.assertEqual(service["image"], f"ghcr.io/kinosail/kinosail-{app}:latest")
                self.assertEqual(service["user"], "10001:10001")
                self.assertTrue(service["read_only"])
                self.assertEqual(service["cap_drop"], ["ALL"])
                self.assertIn("no-new-privileges:true", service["security_opt"])
                self.assertFalse(service.get("privileged", False))
                self.assertEqual(service["ports"][0]["target"], port)
                self.assertEqual(service["ports"][0]["published"], str(port))
                mounts = {volume["target"]: volume for volume in service["volumes"]}
                self.assertEqual(set(mounts), {"/config", "/cache", "/backups", "/media"})
                self.assertEqual(mounts["/media"]["type"], "bind")
                self.assertEqual(mounts["/media"].get("read_only", False), access)
                self.assertFalse(mounts["/media"].get("bind", {}).get("create_host_path", False))
                self.assertEqual(mounts["/media"]["source"], str(ROOT / "apps" / app / "packaging"))
                for target in ("/config", "/cache", "/backups"):
                    self.assertEqual(mounts[target]["type"], "volume")
                env = service["environment"]
                self.assertEqual(env["KINOSAIL_MEDIA_DIR"], "/media")
                self.assertEqual(env["KINOSAIL_DATA_DIR"], "/config")
                self.assertEqual(env["KINOSAIL_CACHE_DIR"], "/cache")
                self.assertEqual(env["KINOSAIL_BACKUP_DIR"], "/backups")

    def test_unraid_templates_expose_required_paths_without_media_default(self):
        for app, port, mode in (("player", "38127", "ro"), ("subtitles", "38128", "rw")):
            with self.subTest(app=app):
                path = ROOT / "apps" / app / "packaging" / "unraid.xml"
                root = ET.parse(path).getroot()
                self.assertEqual(root.tag, "Container")
                self.assertEqual(root.findtext("Repository"), f"ghcr.io/kinosail/kinosail-{app}:latest")
                self.assertEqual(root.findtext("Privileged"), "false")
                self.assertEqual(root.findtext("Network"), "bridge")
                self.assertEqual(root.findtext("TemplateURL"), f"https://raw.githubusercontent.com/Kinosail/kinosail-unraid-templates/main/templates/kinosail-{app}.xml")
                self.assertEqual(root.findtext("Icon"), "https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/internal/server/static/icon-512.png")
                self.assertEqual(root.findtext("License"), "PolyForm Perimeter License 1.0.1")
                self.assertIn(f"[PORT:{port}]", root.findtext("WebUI"))
                extras = root.findtext("ExtraParams")
                for flag in ("--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges:true", "--user=10001:10001", "--init"):
                    self.assertIn(flag, extras)
                configs = {item.get("Target"): item for item in root.findall("Config")}
                self.assertEqual(configs["/media"].get("Mode"), mode)
                self.assertEqual(configs["/media"].get("Default"), "")
                self.assertEqual(configs["/media"].get("Required"), "true")
                for target in ("/config", "/cache", "/backups"):
                    self.assertEqual(configs[target].get("Type"), "Path")
                self.assertEqual(configs[port].get("Type"), "Port")
                self.assertEqual(configs[port].get("Default"), port)

    def test_zimaos_catalog_offers_player_subtitles_and_both(self):
        for choice, apps in (("Player", {"player"}), ("Subtitles", {"subtitles"}), ("Both", {"player", "subtitles"})):
            with self.subTest(choice=choice):
                path = ROOT / "catalogs" / "zimaos" / choice / "docker-compose.yml"
                result = subprocess.run(
                    [*compose_command(), "-f", str(path), "config", "--format", "json"],
                    capture_output=True, text=True, check=True, timeout=30,
                    env=os.environ | {"AppID": f"kinosail-{choice.lower()}"},
                )
                config = json.loads(result.stdout)
                self.assertEqual(set(config["services"]), apps)
                self.assertEqual(config["x-casaos"]["main"], "player" if "player" in apps else "subtitles")
                self.assertEqual(config["x-casaos"]["version"], "0.0.1")
                for app in apps:
                    service = config["services"][app]
                    self.assertEqual(service["image"], f"ghcr.io/kinosail/kinosail-{app}:latest")
                    self.assertEqual(service["user"], "10001:10001")
                    self.assertTrue(service["read_only"])
                    mounts = {volume["target"]: volume for volume in service["volumes"]}
                    self.assertEqual(set(mounts), {"/config", "/cache", "/backups", "/media"})
                    self.assertEqual(mounts["/media"]["source"], "/DATA/Media")
                    self.assertEqual(mounts["/media"].get("read_only", False), app == "player")
                    for target in ("/config", "/cache", "/backups"):
                        self.assertEqual(mounts[target]["type"], "volume")
                self.assertEqual(len(config["volumes"]), 3 * len(apps))


if __name__ == "__main__":
    unittest.main()
