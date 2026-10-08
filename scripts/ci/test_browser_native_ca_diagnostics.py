"""Closed failure attribution around the actual native trust CLI operation."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

HELPER=Path(__file__).resolve().with_name('browser-native-ca.py')

class NativeTrustDiagnostics(unittest.TestCase):
    def setUp(self):
        spec=importlib.util.spec_from_file_location('native_trust_diagnostics',HELPER)
        self.module=importlib.util.module_from_spec(spec);spec.loader.exec_module(self.module)
        if not hasattr(self.module,'cli'):
            # Replay the exact existing CLI catch, rather than fail on an absent new seam.
            block=HELPER.read_text().split("if __name__ == '__main__':\n",1)[1]
            exec('def cli():\n'+block,self.module.__dict__)

    def run_cli(self, cause=None, operation='install'):
        output=io.StringIO()
        with patch.object(self.module.sys,'argv',['helper',operation,'firefox','/private/cert','/private/work','123-456']), patch.object(self.module,operation,side_effect=cause) as called, patch.object(self.module,'remove' if operation=='install' else 'install') as other, contextlib.redirect_stderr(output):
            if cause:
                with self.assertRaises(SystemExit) as exit: self.module.cli()
                self.assertEqual(exit.exception.code,2)
            else: self.module.cli()
            self.assertEqual(called.call_count,1);other.assert_not_called()
        return output.getvalue()

    def test_exact_internal_rejections_have_closed_categories(self):
        for message,category in [('unsupported pinned Firefox cache path','firefox_cache'),('foreign Firefox policy already exists','foreign_policy'),('invalid native trust path','path'),('invalid native trust owner or type','path_owner'),('native trust tool failed','tool_exit'),('native trust tool deadline','tool_deadline'),('native trust tool output overflow','tool_output')]:
            with self.subTest(category=category):
                lines=self.run_cli(ValueError(message)).splitlines()
                self.assertEqual(lines[0],'native fixture trust failed; retain any unverified owned additions')
                self.assertEqual(json.loads(lines[1]),{'event':'native_trust_failure','category':category,'family':'validation'})
                self.assertEqual(len(lines),2)

    def test_unknown_private_malformed_or_oversized_messages_never_leak(self):
        for cause,family in [(ValueError('/private/secret\ncredential'),'validation'),(ValueError('x'*1000000),'validation'),(TypeError('unknown'),'type'),(OSError('/private/tool/output'),'io'),(subprocess.SubprocessError('secret stderr'),'process')]:
            with self.subTest(family=family):
                output=self.run_cli(cause,'remove')
                self.assertLess(len(output.encode()),512)
                self.assertEqual(json.loads(output.splitlines()[1]),{'event':'native_trust_failure','category':'unclassified','family':family})
                self.assertNotIn(str(cause),output)

    def test_success_does_not_emit_a_failure_or_cleanup_other_operation(self):
        self.assertEqual(self.run_cli(),'')
