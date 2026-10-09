"""Measured normalized clocks must control edits while raw quantization stays visible."""
import unittest
from hls_remaining_nonkey_aac_normalized import derive_normalized_audio_edit


def inputs(first_pts, first_ordinal, target_pts, target_ordinal, normalized_target):
    rows = [{'pts': first_pts, 'nativeStartSample': first_ordinal, 'samples': 1024},
            {'pts': target_pts, 'nativeStartSample': target_ordinal, 'samples': 1024}]
    filtered = [{'pts': first_ordinal, 'samples': 1024},
                {'pts': normalized_target, 'samples': 1024}]
    value = {'result': 'observed', 'fullSourceClockQualified': True,
             'sourcePCM': {'sha256': 'fixture'}, 'generatedMediaLogSHA256': 'measurement',
             'preTrimUserFilterClock': {'completeRows': filtered}}
    return {'completeRows': rows}, value


class NormalizedControls(unittest.TestCase):
    def test_mkv_further_seek_uses_measured_clock(self):
        clock, measured = inputs(11925, 572416, 13483, 647168, 647168)
        result = derive_normalized_audio_edit(clock, measured, 11925, 13.5, 75600)
        self.assertEqual(result['deltaSamples'], -16)
        self.assertEqual(result['desiredNativeStartSample'], 648000)
        self.assertFalse(result['usedReferencePCMToChooseEdit'])

    def test_mkv_already_exact_seek_requires_zero_change(self):
        clock, measured = inputs(17920, 860160, 18197, 873472, 873472)
        result = derive_normalized_audio_edit(clock, measured, 17920, 18.2, 13440)
        self.assertEqual(result['deltaSamples'], 0)
        self.assertEqual(result['desiredNativeStartSample'], 873600)

    def test_mp4_native_origin_is_preserved(self):
        clock, measured = inputs(571400, 571408, 599048, 599056, 599048)
        result = derive_normalized_audio_edit(clock, measured, 571400, 12.5, 28600)
        self.assertEqual(result['deltaSamples'], 0)
        self.assertEqual(result['desiredNativeStartSample'], 600008)

    def test_missing_or_ambiguous_normalized_target_rejected(self):
        clock, measured = inputs(10, 500000, 20, 599040, 500000)
        with self.assertRaises(RuntimeError):
            derive_normalized_audio_edit(clock, measured, 10, 12.5, 100000)
        measured['preTrimUserFilterClock']['completeRows'] = [
            {'pts': 599040, 'samples': 1024}, {'pts': 599500, 'samples': 1024}]
        with self.assertRaises(RuntimeError):
            derive_normalized_audio_edit(clock, measured, 10, 12.5, 100000)

    def test_unqualified_or_mismatched_measurement_rejected(self):
        clock, measured = inputs(10, 500000, 20, 599040, 599040)
        measured['fullSourceClockQualified'] = False
        with self.assertRaises(RuntimeError):
            derive_normalized_audio_edit(clock, measured, 10, 12.5, 100000)
        measured['fullSourceClockQualified'] = True
        measured['preTrimUserFilterClock']['completeRows'][0]['samples'] = 1000
        with self.assertRaises(RuntimeError):
            derive_normalized_audio_edit(clock, measured, 10, 12.5, 100000)


if __name__ == '__main__':
    unittest.main()
