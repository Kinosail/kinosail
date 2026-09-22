"""Reusable app selections must be valid for the suite rollup."""

import json
import unittest

from affected import FLAGS
from app_plan import select


class AppPlanTests(unittest.TestCase):
    def test_selected_server_and_native_only_inputs(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["dashboard"] = plan["dashboard_browsers"] = True
        self.assertEqual(select("dashboard", json.dumps(plan)),
                         {"selected": True, "tools": False, "browsers": True, "arm": False, "client": False})
        plan["dashboard"] = plan["dashboard_browsers"] = False
        plan["client"] = True
        self.assertEqual(select("player", json.dumps(plan))["client"], True)
        self.assertFalse(select("player", json.dumps(plan))["selected"])

    def test_invalid_input_is_rejected(self):
        plan = dict.fromkeys((*FLAGS, "deep"), False)
        plan["player"] = True
        raw = json.dumps(plan)
        for app, value in (("../player", raw), ("unknown", raw), ("player", "[]"),
                           ("player", "x" * 16385), ("player", json.dumps(plan | {"extra": True})),
                           ("player", json.dumps(plan | {"player": "true"}))):
            with self.subTest(app=app, value=value[:20]), self.assertRaises(ValueError):
                select(app, value)
        plan["player"] = False
        plan["player_arm"] = True
        with self.assertRaises(ValueError):
            select("player", json.dumps(plan))


if __name__ == "__main__":
    unittest.main()
