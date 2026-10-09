"""Actual-public video and AVC metadata failure matrix, written before helpers."""
import hashlib
import struct
import unittest
from unittest.mock import patch
from hls_nonkey_browser_video import public_video_suffix, avc_configuration_metadata, actual_video_evidence

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
    def test_failed_actual_suffix_keeps_bounded_measurement_before_rejection(self):
        result={}
        with patch('hls_nonkey_browser_video.public_video_suffix',return_value={'qualified':False,'publicVideoPackets':3}):
            with self.assertRaises(RuntimeError):actual_video_evidence('source','public',[],[],{},result)
        self.assertEqual(result['packetSuffix'],{'qualified':False,'publicVideoPackets':3})
        self.assertEqual(result['currentStage'],'actual-public-video-packets')
    def test_failed_configuration_keeps_every_validated_metadata_stage(self):
        result={}
        configs=[{'avcConfigurationSHA256':'a'*64,'color':None},{'avcConfigurationSHA256':'b'*64,'color':None}]
        streams=[{'extradata_hash':'SHA256:'+'a'*64},{'extradata_hash':'SHA256:'+'b'*64}]
        with patch('hls_nonkey_browser_video.public_video_suffix',return_value={'qualified':True}), patch('hls_nonkey_browser_video.bounded_bytes',return_value=b'fixture'), patch('hls_nonkey_browser_video.avc_configuration_metadata',side_effect=configs), patch('hls_nonkey_browser_video.video_stream',side_effect=streams):
            with self.assertRaises(RuntimeError):actual_video_evidence('source','public',[],[],{},result)
        self.assertFalse(result['configurationIdentity'])
        self.assertEqual(result['sourceAVC'],configs[0])
        self.assertEqual(result['publicAVC'],configs[1])
        self.assertEqual(result['sourceVideoStream'],streams[0])
        self.assertEqual(result['publicVideoStream'],streams[1])
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
