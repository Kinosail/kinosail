"""Clock phase controls require data derivation and reject ambiguity."""
import unittest
from hls_remaining_nonkey_aac_derived import derive_audio_edit


def row(pts, timestamp, ordinal, count=1024):
    return {'pts': pts, 'timestampSampleNumerator': timestamp,
        'timestampSampleDenominator': 1, 'nativeStartSample': ordinal, 'samples': count}


class DerivedControls(unittest.TestCase):
    def test_mkv_measured_phases_derive_minus_sixteen(self):
        value = derive_audio_edit({'completeRows': [row(11925, 572400, 572416),
            row(12480, 599040, 599040)]}, 11925, 12.5, 27600)
        self.assertEqual(value['derivedMediaTime'], 27584)
        self.assertEqual(value['desiredNativeStartSample'], 600000)
        self.assertEqual(value['deltaSamples'], -16)
        self.assertFalse(value['usedReferencePCMToChooseEdit'])

    def test_mp4_native_origin_is_preserved(self):
        value = derive_audio_edit({'completeRows': [row(571400, 571400, 571408),
            row(599048, 599048, 599056)]}, 571400, 12.5, 28600)
        self.assertEqual(value['desiredNativeStartSample'], 600008)
        self.assertEqual(value['derivedMediaTime'], 28600)
        self.assertEqual(value['deltaSamples'], 0)

    def test_phase_difference_is_not_a_fixed_container_constant(self):
        value = derive_audio_edit({'completeRows': [row(10, 500000, 499984),
            row(20, 599040, 599056)]}, 10, 12.5, 100000)
        self.assertEqual(value['deltaSamples'], 32)
        self.assertEqual(value['desiredNativeStartSample'], 600016)

    def test_ambiguous_or_missing_target_is_rejected(self):
        first = row(10, 500000, 500000)
        for rows in [[first], [first, row(20, 599040, 599040), row(30, 599500, 599500)]]:
            with self.assertRaises(RuntimeError):
                derive_audio_edit({'completeRows': rows}, 10, 12.5, 100000)

    def test_duplicate_first_identity_is_rejected(self):
        rows = [row(10, 500000, 500000), row(10, 501024, 501024), row(20, 599040, 599040)]
        with self.assertRaises(RuntimeError):
            derive_audio_edit({'completeRows': rows}, 10, 12.5, 100000)

    def test_nonintegral_target_or_unbounded_delta_is_rejected(self):
        rows = [row(10, 500000, 500000), row(20, 599040, 599040)]
        for offset, current in [(12.500001, 100000), (12.5, 99900)]:
            with self.assertRaises(RuntimeError):
                derive_audio_edit({'completeRows': rows}, 10, offset, current)


if __name__ == '__main__':
    unittest.main()
