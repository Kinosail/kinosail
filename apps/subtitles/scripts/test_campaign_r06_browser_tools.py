"""Bounded tool identity controls; all tool paths and fingerprint calls are fake."""
import importlib.util
from pathlib import Path
import stat
from types import SimpleNamespace
import unittest
from unittest.mock import patch

script = Path(__file__).with_name("campaign-r06-browser.py")
spec = importlib.util.spec_from_file_location("campaign_r06_browser_owned", script)
driver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(driver)


class MemoryTool:
    sizes = {}

    def __init__(self, name):
        self.name = name

    def resolve(self, strict):
        if strict is not True:
            raise AssertionError("strict-resolution-required")
        return self

    def lstat(self):
        return SimpleNamespace(st_size=self.sizes[self.name], st_mode=stat.S_IFREG | 0o755)


class ToolIdentityControls(unittest.TestCase):
    def setUp(self):
        MemoryTool.sizes = {"go":17142188, "node":149711504, "pnpm":1}
        self.calls = []
        self.stack = [
            patch.object(driver.shutil, "which", side_effect=lambda name:name),
            patch.object(driver, "Path", MemoryTool),
            patch.object(driver, "fingerprint", side_effect=self.pin_tool),
        ]
        for control in self.stack:
            control.start()
            self.addCleanup(control.stop)

    def pin_tool(self, path, limit, executable):
        self.assertIs(executable, True)
        self.calls.append((path.name, limit))
        if not 0 < MemoryTool.sizes[path.name] <= limit:
            raise ValueError("file-shape")
        return {"name":path.name, "bytes":MemoryTool.sizes[path.name]}

    def reject(self, name, size):
        if name != "node":
            MemoryTool.sizes["node"] = 1
        MemoryTool.sizes[name] = size
        with self.assertRaisesRegex(ValueError, "^file-shape$"):
            driver.tools_state()
        self.assertEqual(self.calls[-1][0], name)

    def test_observed_node_size_can_be_pinned(self):
        preflight = {}
        result = driver.tools_state(preflight)
        self.assertEqual(set(result), {"go", "node", "pnpm"})
        self.assertEqual(preflight["node"], {
            "available":True, "bytes":149711504, "mode":0o755, "regular":True,
        })
        self.assertEqual(dict(self.calls), {
            "go":64*1024*1024, "node":192*1024*1024, "pnpm":64*1024*1024,
        })

    def test_node_cap_boundary_can_be_pinned(self):
        MemoryTool.sizes["node"] = 192*1024*1024
        self.assertEqual(driver.tools_state()["node"]["bytes"], 192*1024*1024)

    def test_node_above_cap_is_rejected(self):
        self.reject("node", 192*1024*1024+1)

    def test_go_above_original_cap_is_rejected(self):
        self.reject("go", 64*1024*1024+1)

    def test_pnpm_above_original_cap_is_rejected(self):
        self.reject("pnpm", 64*1024*1024+1)

    def test_unavailable_tool_retains_fixed_safe_diagnostic(self):
        with patch.object(driver.shutil, "which", return_value=None):
            preflight = {}
            with self.assertRaisesRegex(ValueError, "^tool-unavailable$"):
                driver.tools_state(preflight)
        self.assertEqual(preflight, {"go":{"available":False}})
        self.assertEqual(self.calls, [])


if __name__ == "__main__":
    unittest.main()
