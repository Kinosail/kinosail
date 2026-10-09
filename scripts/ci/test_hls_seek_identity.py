import importlib.util
from pathlib import Path
import unittest

ROOT=Path(__file__).resolve().parents[2]
path=ROOT/'apps/player/scripts/hls_seek_identity.py'
spec=importlib.util.spec_from_file_location('hls_seek_identity',path)
module=importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

def pixels(index):
    data=bytearray([128]*(240*64))
    def square(x,y,value):
        for row in range(y-2,y+3):
            for column in range(x-2,x+3):data[row*240+column]=value
    for bit in range(10):
        value=235 if index&(1<<bit) else 16
        square(24+16*bit,24,value);square(24+16*bit,56,251-value)
    square(200,24,16);square(232,24,235)
    return bytes(data)

class SourceIdentityTests(unittest.TestCase):
    def test_each_source_frame_has_one_exact_identity(self):
        self.assertEqual([module.marker_frame(pixels(i)) for i in range(768)],list(range(768)))
        self.assertEqual(module.qualify_sequence(list(range(300,768)),list(range(300,768))),468)
    def test_preroll_missing_duplicate_reordered_and_later_target_are_rejected(self):
        for rows in [list(range(288,768)),list(range(301,768)),[300,300]+list(range(302,768)),
                     [301,300]+list(range(302,768)),list(range(288,300))+list(range(300,768))]:
            with self.subTest(count=len(rows)),self.assertRaises(RuntimeError):module.qualify_sequence(rows,list(range(300,768)))
    def test_ambiguous_corrupt_complement_guard_and_out_of_fixture_reject(self):
        def bad_guard(data):
            for row in range(22,27):data[row*240+198:row*240+203]=bytes([235])*5
        for mutate in [lambda b:b.__setitem__(slice(22*240,27*240),bytes([128])*(5*240)),
                       lambda b:b.__setitem__(slice(54*240,59*240),bytes([16])*(5*240)),bad_guard]:
            data=bytearray(pixels(300));mutate(data)
            with self.assertRaises(RuntimeError):module.marker_frame(bytes(data))
        with self.assertRaises(RuntimeError):module.marker_frame(pixels(900))
    def test_missing_malformed_oversized_and_boolean_identity_reject(self):
        for raw in [None,'private',b'',pixels(300)[:-1],pixels(300)+b'x']:
            with self.subTest(kind=type(raw).__name__),self.assertRaises(RuntimeError):module.marker_frame(raw)
        for rows in [[],[True]+list(range(301,768)),list(range(300,768))+[768],[-1]+list(range(301,768))]:
            with self.assertRaises(RuntimeError):module.qualify_sequence(rows,list(range(300,768)))

if __name__=='__main__':unittest.main()
