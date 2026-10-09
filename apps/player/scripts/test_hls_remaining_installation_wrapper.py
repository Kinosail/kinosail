"""Generated wrapper passthrough paths absent from the fixed public refill proof."""
import hashlib
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).parent))
from hls_remaining_capture import capture_codec


class InstallationWrapperPassthrough(unittest.TestCase):
    def wrapper(self, directory):
        source = directory / 'media/Fixture.flac'
        source.parent.mkdir()
        source.write_bytes(b'synthetic wrapper-only source; never media execution')
        wrapper = directory / 'paced-ffmpeg'
        executable = directory/'sentinel'; executable.write_bytes(b'not-a-codec')
        with patch('shutil.which', return_value=str(executable)):
            capture_codec(directory, source, 0.75, True, {}, {})
        return wrapper.read_text(), source

    def test_initial_commands_are_only_paced_and_foreign_source_cannot_execute(self):
        with tempfile.TemporaryDirectory(prefix='kinosail-aac-wrapper-') as temporary:
            directory = Path(temporary)
            program, source = self.wrapper(directory)
            for start, selected in [('0', str(source)), ('2', str(source))]:
                arguments = ['-hls_time', '2', '-i', selected, '-start_number', start, 'initial.m3u8']
                expected = [str(directory/'sentinel'), '-hls_time', '2', '-readrate', '0.75', '-i', selected,
                    '-start_number', start, 'initial.m3u8']
                with self.subTest(start=start, selected=selected), patch.object(sys, 'argv', ['wrapper', *arguments]), patch('os.execv') as launch:
                    exec(compile(program, 'wrapper-passthrough-control', 'exec'), {})
                    launch.assert_called_once_with(str(directory/'sentinel'), expected)
                self.assertFalse((directory / 'installed-recipe-private.json').exists())
                self.assertFalse((directory / 'installation-certificate.json').exists())
            log = (directory/'pacing-invocations.jsonl').read_bytes()
            with patch.object(sys, 'argv', ['wrapper', '-hls_time', '2', '-i', '/foreign.flac', '-start_number', '4']), patch('os.execv') as launch:
                with self.assertRaisesRegex(RuntimeError, '^refill_capture_source$'):
                    exec(compile(program, 'wrapper-foreign-control', 'exec'), {})
                launch.assert_not_called()
            self.assertEqual((directory/'pacing-invocations.jsonl').read_bytes(), log)

    def test_matching_start_with_wrong_tuple_never_executes(self):
        with tempfile.TemporaryDirectory(prefix='kinosail-aac-wrapper-') as temporary:
            directory = Path(temporary)
            program, source = self.wrapper(directory)
            eligibility = {'audio': {'codec': 'flac', 'sampleRate': 48000, 'channels': 2, 'index': 0},
                'sourceSHA256': hashlib.sha256(source.read_bytes()).hexdigest()}
            (directory / 'installation-eligibility-private.json').write_text(json.dumps(eligibility))
            target = directory / 'cache/0123456789abcdef-plan-a-a0/.seek-4/audio/index.m3u8'
            arguments = ['-hls_time', '2', '-i', str(source), '-map', '0:a:0', '-start_number', '4',
                '-hls_segment_filename', str(target.parents[2]/'audio/segment-%05d.m4s'), str(target)]
            with patch.object(sys, 'argv', ['wrapper', *arguments]), patch('os.execv') as launch:
                with self.assertRaisesRegex(RuntimeError, '^installation_closed_actual_refill$'):
                    exec(compile(program, 'wrapper-closed-control', 'exec'), {})
                launch.assert_not_called()
            self.assertFalse((directory / 'installed-recipe-private.json').exists())
            self.assertFalse((directory / 'installation-certificate.json').exists())


if __name__ == '__main__':
    unittest.main()
