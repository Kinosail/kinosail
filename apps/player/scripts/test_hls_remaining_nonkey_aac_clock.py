"""Malformed clock evidence must not qualify complete PCM or native clock."""
import hashlib
import unittest
from hls_remaining_nonkey_aac_clock import frame_md5, filter_clock


def document(data, samples=4, pts=0):
    digest = hashlib.md5(data, usedforsecurity=False).hexdigest()
    return ('#tb 0: 1/48000\n#media_type 0: audio\n#sample_rate 0: 48000\n'
            + f'0, {pts}, {pts}, {samples}, {samples * 4}, {digest}\n')


class ClockControls(unittest.TestCase):
    def test_complete_bytes_and_negative_clock_retained(self):
        data = b'\x01\x00\x02\x00' * 4
        value = frame_md5(document(data, pts=-8), data)
        self.assertEqual(value['samples'], 4)
        self.assertEqual(value['residualSamples'], [-8])
        self.assertTrue(value['allOutputPacketPCMBytesBound'])

    def test_changed_or_omitted_pcm_is_rejected(self):
        data = b'\x01\x00\x02\x00' * 4
        for changed in [data[:-4], data + b'\x00' * 4, b'\x00' * len(data)]:
            with self.assertRaises(RuntimeError):
                frame_md5(document(data), changed)

    def test_wrong_format_and_size_rejected(self):
        data = b'\x00' * 16
        for text in [document(data).replace('1/48000', '1/1000'),
                     document(data).replace('4, 16,', '4, 15,')]:
            with self.assertRaises(RuntimeError):
                frame_md5(text, data)

    def test_filter_clock_has_independent_framing(self):
        line = '[Parsed_ashowinfo_0 @ fixture] n:0 pts:0 pts_time:0 fmt:fltp channels:2 chlayout:stereo rate:48000 nb_samples:1024 checksum:ABCDEF01'
        value = filter_clock(line)
        self.assertEqual(value['samples'], 1024)
        self.assertFalse(value['timeBaseInferred'])
        self.assertEqual(value['completeRows'][0]['checksum'], 'ABCDEF01')

    def test_filter_missing_or_noncontiguous_is_rejected(self):
        line = '[Parsed_ashowinfo_0 @ fixture] n:0 pts:0 pts_time:0 fmt:fltp channels:2 rate:48000 nb_samples:1024 checksum:ABCDEF01'
        for text in [line.replace('n:0', 'n:1'), line.replace('pts_time:0', 'pts_time:N/A'),
                     line.replace('rate:48000', 'rate:44100'), 'unrelated line']:
            with self.assertRaises((RuntimeError, ValueError)):
                filter_clock(text)


if __name__ == '__main__':
    unittest.main()
