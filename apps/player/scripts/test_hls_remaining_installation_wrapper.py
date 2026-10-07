"""Generated wrapper passthrough paths absent from the fixed public refill proof."""
import ast
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


class InstallationWrapperPassthrough(unittest.TestCase):
    def wrapper(self, directory):
        driver = Path(__file__).with_name('test-hls-remaining.py')
        tree = ast.parse(driver.read_text())
        expression = next(n for n in ast.walk(tree) if isinstance(n, ast.Call) and isinstance(n.func, ast.Attribute)
            and n.func.attr == 'write_text' and isinstance(n.func.value, ast.Name) and n.func.value.id == 'wrapper')
        source = directory / 'media/Fixture.flac'
        source.parent.mkdir()
        source.write_bytes(b'synthetic wrapper-only source; never media execution')
        wrapper = directory / 'paced-ffmpeg'
        scope = {'directory': directory, 'source': source, 'wrapper': wrapper, 'pacing': 0.75,
            'real': '/owned/pinned/ffmpeg', 'invocation': directory / 'pacing-invocations.jsonl',
            'Path': Path, '__file__': str(driver), 'installation': True}
        eval(compile(ast.Expression(expression), 'wrapper-static-control', 'eval'), scope)
        return wrapper.read_text(), source

    def test_initial_and_unrelated_commands_are_only_paced(self):
        with tempfile.TemporaryDirectory(prefix='kinosail-aac-wrapper-') as temporary:
            directory = Path(temporary)
            program, source = self.wrapper(directory)
            for start, selected in [('0', str(source)), ('2', str(source)), ('4', str(directory / 'unrelated.flac'))]:
                arguments = ['-hls_time', '2', '-i', selected, '-start_number', start, 'initial.m3u8']
                expected = ['/owned/pinned/ffmpeg', '-hls_time', '2', '-readrate', '0.75', '-i', selected,
                    '-start_number', start, 'initial.m3u8']
                with self.subTest(start=start, selected=selected), patch.object(sys, 'argv', ['wrapper', *arguments]), patch('os.execv') as launch:
                    exec(compile(program, 'wrapper-passthrough-control', 'exec'), {})
                    launch.assert_called_once_with('/owned/pinned/ffmpeg', expected)
                self.assertFalse((directory / 'installed-recipe-private.json').exists())
                self.assertFalse((directory / 'installation-certificate.json').exists())

    def test_matching_start_with_wrong_tuple_never_executes(self):
        with tempfile.TemporaryDirectory(prefix='kinosail-aac-wrapper-') as temporary:
            directory = Path(temporary)
            program, source = self.wrapper(directory)
            eligibility = {'audio': {'codec': 'flac', 'sampleRate': 48000, 'channels': 2, 'index': 0},
                'sourceSHA256': hashlib.sha256(source.read_bytes()).hexdigest()}
            (directory / 'installation-eligibility-private.json').write_text(json.dumps(eligibility))
            target = directory / 'cache/0123456789abcdef-plan-a-a0/.seek-4/audio/index.m3u8'
            arguments = ['-hls_time', '2', '-i', str(source), '-start_number', '4', str(target)]
            with patch.object(sys, 'argv', ['wrapper', *arguments]), patch('os.execv') as launch:
                with self.assertRaisesRegex(RuntimeError, '^installation_closed_actual_refill$'):
                    exec(compile(program, 'wrapper-closed-control', 'exec'), {})
                launch.assert_not_called()
            self.assertFalse((directory / 'installed-recipe-private.json').exists())
            self.assertFalse((directory / 'installation-certificate.json').exists())


if __name__ == '__main__':
    unittest.main()
