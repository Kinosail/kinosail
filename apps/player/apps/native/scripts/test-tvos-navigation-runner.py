"""Protect evidence admission when a task-owned child refuses to stop.

Native UI journeys cannot inject an unjoinable operating-system process. Load
only the process settlement function so this isolated check never starts Xcode.
"""
import ast
from pathlib import Path
import signal
import subprocess
import unittest
from unittest.mock import Mock


class NavigationRunnerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        source = Path(__file__).with_name("run-tvos-navigation-qa.py").read_text()
        function = next(node for node in ast.parse(source).body
                        if isinstance(node, ast.FunctionDef) and node.name == "settle")
        namespace = {"signal": signal, "subprocess": subprocess}
        exec(compile(ast.Module(body=[function], type_ignores=[]), "runner-settlement", "exec"), namespace)
        cls.settle = staticmethod(namespace["settle"])

    def test_completed_child_needs_no_signal(self):
        child = Mock()
        child.poll.return_value = 0
        self.settle(child)
        child.send_signal.assert_not_called()
        child.terminate.assert_not_called()
        child.kill.assert_not_called()

    def test_unjoinable_child_is_a_failure(self):
        child = Mock()
        child.poll.return_value = None
        child.wait.side_effect = subprocess.TimeoutExpired("fixture-child", 1)
        with self.assertRaises(RuntimeError):
            self.settle(child)
        child.kill.assert_called_once()


if __name__ == "__main__":
    unittest.main()
