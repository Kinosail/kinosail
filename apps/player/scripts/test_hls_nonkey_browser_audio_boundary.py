"""Written-first raw AAC boundary controls; no discard-map or missing-sample inference."""
import copy
import hashlib
import struct
import unittest
from hls_nonkey_browser_audio_boundary import source_audio_boundary, retained_audio_boundary, retained_source_boundary_bytes

def box(kind,body):
    return struct.pack('>I4s',len(body)+8,kind)+body
def full(kind,body):
    return box(kind,bytes(4)+body)
def source_fixture():
    clock=bytes(8)+struct.pack('>II',48000,4072)
    header=bytes(8)+struct.pack('>III',2,0,1000)
    edit=full(b'elst',struct.pack('>IIihh',1,1000,1008,1,0))
    stts=full(b'stts',struct.pack('>IIIII',2,3,1024,1,1000))
    track=box(b'trak',full(b'tkhd',header)+box(b'edts',edit)+box(b'mdia',
        full(b'mdhd',clock)+full(b'hdlr',bytes(4)+b'soun')+box(b'minf',box(b'stbl',stts))))
    movie=box(b'moov',full(b'mvhd',bytes(8)+struct.pack('>II',1000,1000))+track)
    return box(b'ftyp',b'isom')+box(b'mdat',b'abc')+movie

def packet(n):
    return {'pts':n,'dts':n,'duration':1024,'pts_time':'0.000000','dts_time':'0.000000',
        'duration_time':'0.021333','size':'12','flags':'K_','data_hash':'SHA256:'+'a'*64,
        'side_data_list':[{'side_data_type':'Skip Samples','skip_samples':1008,'discard_padding':16,'skip_reason':0,'discard_reason':0}]}

class BoundaryTests(unittest.TestCase):
    def observations(self):
        stream={'sample_rate':'48000','channels':2,'codec_name':'aac','time_base':'1/48000','start_time':'-0.500000','duration':'1.000000'}
        pcm={'stream':stream,'samples':4072,'completeEOFAccounted':True}
        return {'sourcePacketRows':[dict(packet(n),stream_index=1) for n in [0,1024,2048,3072]],
            'publicPacketRows':[dict(packet(n),stream_index=1) for n in [0,1024,2048,3072]],
            'nativePCM':{'source':pcm,'public':copy.deepcopy(pcm)},
            'initialization':{'movieTimescale':1000,'tracks':[{'trackID':2,'handler':'soun','mediaTimescale':48000,
                'edits':[{'duration':1000,'mediaTime':1008,'rateInteger':1,'rateFraction':0}]}]},
            'physicalFragments':[{'tracks':[{'trackID':2,'samples':[{'pts':n,'dts':n,'duration':1024} for n in [0,1024,2048,3072]]}]}]}
    def test_source_raw_container_units_stts_duration_and_edits_are_preserved(self):
        data=source_fixture();digest=hashlib.sha256(data).hexdigest()
        value=source_audio_boundary(data,digest)
        self.assertEqual(value['sourceSHA256'],digest)
        self.assertEqual(value['movie']['timescale'],1000)
        self.assertEqual(value['audio']['mediaTimescale'],48000)
        self.assertEqual(value['audio']['mdhdDuration'],4072)
        self.assertEqual(value['audio']['edits'][0]['mediaTime'],1008)
        self.assertEqual(value['audio']['sttsSamples'],4)
        self.assertEqual(value['audio']['sttsDuration'],4072)
        self.assertEqual(value['audio']['lastPhysicalSamples'][-1]['duration'],1000)
        self.assertFalse(value['missingSampleLocationEstablished'])
    def test_raw_shape_or_source_hash_failure_is_retained_without_aborting_original_evidence(self):
        data=source_fixture()
        for bad,digest in [(data,'0'*64),(data[:-1],hashlib.sha256(data[:-1]).hexdigest()),(bytes(),hashlib.sha256(bytes()).hexdigest())]:
            value=retained_source_boundary_bytes(bad,digest)
            self.assertFalse(value['observed']);self.assertIn('failureClass',value)
    def test_full_packet_edges_side_data_streams_and_raw_public_clocks_are_distinct(self):
        facts=self.observations();before=copy.deepcopy(facts)
        result=retained_audio_boundary(facts)
        self.assertTrue(result['observed']);self.assertEqual(facts,before)
        self.assertEqual(len(result['sourcePackets']['firstThree']),3)
        self.assertEqual(len(result['publicPackets']['lastThree']),3)
        self.assertEqual(result['sourcePackets']['firstThree'][0]['skipSamples'][0]['discard_padding'],16)
        self.assertEqual(result['publicRaw']['trackID'],2)
        self.assertEqual(result['publicRaw']['lastPhysicalSamples'][-1]['dts'],3072)
        self.assertEqual(result['publicRaw']['mediaTimescale'],48000)
        self.assertFalse(result['missingSampleLocationEstablished'])
    def test_missing_packet_fields_are_explicit_and_malformed_or_unmapped_values_are_held(self):
        facts=self.observations();del facts['sourcePacketRows'][0]['duration']
        result=retained_audio_boundary(facts)
        self.assertTrue(result['observed']);self.assertIsNone(result['sourcePackets']['firstThree'][0]['duration'])
        for mutation in ['hash','decimal','side','track','raw']:
            facts=self.observations()
            if mutation=='hash':facts['sourcePacketRows'][0]['data_hash']='private'
            if mutation=='decimal':facts['sourcePacketRows'][0]['pts_time']='nan'
            if mutation=='side':facts['sourcePacketRows'][0]['side_data_list'][0]['side_data_type']='private'
            if mutation=='track':facts['initialization']['tracks'][0]['handler']='vide'
            if mutation=='raw':facts['physicalFragments'][0]['tracks'][0]['samples'][0]['duration']=0
            with self.subTest(mutation=mutation):
                self.assertFalse(retained_audio_boundary(facts)['observed'])
if __name__=='__main__':unittest.main()
