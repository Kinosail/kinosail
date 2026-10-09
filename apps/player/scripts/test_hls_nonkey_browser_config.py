"""Rational handoff controls written before the bounded clock parser."""
import unittest
from hls_nonkey_browser_config import measured_delta

class BrowserRationalHandoff(unittest.TestCase):
    def test_rational_source_clock_preserved(self):
        for text,seconds in [('0',0.0),('1/2',0.5),('9/5',1.8)]:
            self.assertEqual(measured_delta(text),{'rational':text,'seconds':seconds})
    def test_invalid_or_unbounded_clock_rejected(self):
        for value in [None,1,True,'','nan','inf','-1/2','15','1/0','1/'+'9'*65,'2.5','2/3/4']:
            with self.subTest(value=value):
                with self.assertRaises(RuntimeError):
                    measured_delta(value)

if __name__=='__main__':
    unittest.main()
