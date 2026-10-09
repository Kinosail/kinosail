"""Rational handoff controls written before the bounded clock parser."""
import unittest
from hls_nonkey_browser_config import measured_delta, ordinary_cold_arguments

class BrowserRationalHandoff(unittest.TestCase):
    def test_rational_source_clock_preserved(self):
        for text,seconds in [('0',0.0),('1/2',0.5),('9/5',1.8)]:
            self.assertEqual(measured_delta(text),{'rational':text,'seconds':seconds})
    def test_invalid_or_unbounded_clock_rejected(self):
        for value in [None,1,True,'','nan','inf','-1/2','15','1/0','1/'+'9'*65,'2.5','2/3/4']:
            with self.subTest(value=value):
                with self.assertRaises(RuntimeError):
                    measured_delta(value)

class BrowserOrdinaryProducer(unittest.TestCase):
    def arguments(self):
        return ['-ss','12.5','-i','fixed.mp4','-hls_time','2','-f','hls','out.m3u8']
    def test_omitted_start_number_is_initial_zero(self):
        self.assertTrue(ordinary_cold_arguments(self.arguments(),'fixed.mp4'))
        self.assertTrue(ordinary_cold_arguments(self.arguments()+['-start_number','0'],'fixed.mp4'))
    def test_indexed_refill_or_other_source_is_unchanged(self):
        for extra in [['-seek_timestamp','1'],['-start_number','1'],['-start_number','-1']]:
            with self.subTest(extra=extra):
                self.assertFalse(ordinary_cold_arguments(self.arguments()+extra,'fixed.mp4'))
        self.assertFalse(ordinary_cold_arguments(self.arguments(),'other.mp4'))
        self.assertFalse(ordinary_cold_arguments(['-ss','12.5','-i','fixed.mp4','-hls_time','0.1'],'fixed.mp4'))
    def test_malformed_or_duplicate_options_are_unchanged(self):
        for extra in [['-ss'],['-start_number'],['-i','fixed.mp4'],['-hls_time','2'],['-ss','12']]:
            with self.subTest(extra=extra):
                self.assertFalse(ordinary_cold_arguments(self.arguments()+extra,'fixed.mp4'))

if __name__=='__main__':
    unittest.main()
