"""Actual-public video and AVC metadata failure matrix, written before helpers."""
import hashlib
import struct
import unittest
from hls_nonkey_browser_video import public_video_suffix, avc_configuration_metadata

def packet(n):
    return {'stream_index':0,'data_hash':'SHA256:'+format(n,'064x'),'pts_time':str(n/24),
        'dts_time':str((n-2)/24),'duration_time':str(1/24),'flags':'K' if n==2 else '_'}

def atom(kind,data):
    return struct.pack('>I4s',len(data)+8,kind)+data

def movie(extra=b''):
    entry=atom(b'avc1',bytes(78)+atom(b'avcC',bytes([1,100,0,31,255,224,0]))+extra)
    data=atom(b'stsd',bytes(4)+struct.pack('>I',1)+entry)
    for kind in [b'stbl',b'minf',b'mdia',b'trak',b'moov']:data=atom(kind,data)
    return data

class PublicBrowserVideo(unittest.TestCase):
    def test_complete_actual_packet_suffix_requires_every_payload_through_eof(self):
        source=[packet(n) for n in range(8)]
        required={'pts_time':source[2]['pts_time'],'payloadSHA256':source[2]['data_hash'][7:]}
        self.assertTrue(public_video_suffix(source,source[2:],required)['qualified'])
        for public in [source[2:-1],source[2:4]+source[5:],source[2:]+[source[-1]],[dict(source[3],flags='K')]+source[4:]]:
            self.assertFalse(public_video_suffix(source,public,required)['qualified'])
    def test_untrusted_video_packet_identity_and_required_key_are_rejected(self):
        source=[packet(n) for n in range(8)]
        required={'pts_time':source[2]['pts_time'],'payloadSHA256':source[2]['data_hash'][7:]}
        for bad in [dict(source[2],data_hash='bad'),dict(source[2],pts_time='nan'),
                    dict(source[2],flags='_'),dict(source[2],duration_time='0')]:
            with self.assertRaises(RuntimeError):public_video_suffix(source,[bad]+source[3:],required)
    def test_avc_configuration_and_explicit_or_absent_color_are_sealed(self):
        plain=avc_configuration_metadata(movie())
        self.assertIsNone(plain['color'])
        color=avc_configuration_metadata(movie(atom(b'colr',b'nclx'+struct.pack('>HHHB',1,1,1,128))))
        self.assertEqual(color['color'],{'kind':'nclx','primaries':1,'transfer':1,'matrix':1,'fullRange':True})
        self.assertEqual(plain['avcConfigurationSHA256'],color['avcConfigurationSHA256'])
        self.assertEqual(plain['avcConfigurationSHA256'],hashlib.sha256(bytes([1,100,0,31,255,224,0])).hexdigest())
    def test_malformed_or_duplicate_avc_and_color_extents_are_rejected(self):
        for data in [b'',movie()[:-1],movie(atom(b'colr',b'nclx')),movie(atom(b'avcC',bytes(7))),
                     movie(atom(b'colr',b'nclx'+struct.pack('>HHHB',1,1,1,1)))]:
            with self.assertRaises(RuntimeError):avc_configuration_metadata(data)

if __name__=='__main__':unittest.main()
