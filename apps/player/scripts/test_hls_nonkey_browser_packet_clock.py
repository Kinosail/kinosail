"""Clock observer controls: wrong units, missing rows, ambiguity and interior jumps."""
import unittest
from hls_nonkey_browser_packet_clock import packet_clock

def row(number, pts, duration=1024):
    return {'data_hash': 'SHA256:'+format(number, '064x'), 'pts': pts,
            'dts': pts, 'duration': duration}

def boundary():
    return {'sourceStream': {'timeBase':'1/48000','sampleRate':48000},
            'publicStream': {'timeBase':'1/48000','sampleRate':48000}}

class PacketClockTest(unittest.TestCase):
    def test_all_unique_packets_and_interior_changes_are_retained(self):
        source=[row(1,10000),row(2,11024),row(3,12048),row(4,13072)]
        public=[row(1,0),row(2,1024),row(3,1048),row(4,2072)]
        result=packet_clock({'completeSourceRows':source,'completePublicRows':public},boundary())
        self.assertTrue(result['observed'])
        self.assertFalse(result['acceptance'])
        self.assertEqual(result['knownClockPackets'],4)
        self.assertEqual(result['clockOffsetRangeTicks'],[-11000,-10000])
        self.assertEqual(result['clockChangeCount'],1)
        self.assertEqual(result['clockChanges'][0],{'publicOrdinal':2,'sourceOrdinal':2,
            'previousOffsetTicks':-10000,'offsetTicks':-11000})
        self.assertEqual(result['adjacentPTSChanges'][0]['deltaTicks'],-1000)
        self.assertEqual(result['adjacentPTSChangeCount'],1)

    def test_ambiguous_and_unknown_payloads_are_explicit(self):
        source=[row(1,100),row(1,1124),row(2,2148)]
        public=[row(1,0),row(7,1024),row(2,2048)]
        result=packet_clock({'completeSourceRows':source,'completePublicRows':public},boundary())
        self.assertEqual(result['knownClockPackets'],1)
        self.assertEqual(result['ambiguousPackets'],1)
        self.assertEqual(result['unknownPackets'],1)
        self.assertFalse(result['acceptance'])

    def test_bad_units_and_integer_shapes_fail_closed(self):
        tail={'completeSourceRows':[row(1,100)],'completePublicRows':[row(1,0)]}
        for units in [{}, {'sourceStream':{'timeBase':'1/1000','sampleRate':48000},
                           'publicStream':boundary()['publicStream']}]:
            self.assertFalse(packet_clock(tail,units)['observed'])
        for field,value in [('pts',True),('dts',None),('duration',0),('pts',1<<53)]:
            damaged={'completeSourceRows':[row(1,100)],'completePublicRows':[row(1,0)]}
            damaged['completePublicRows'][0][field]=value
            self.assertFalse(packet_clock(damaged,boundary())['observed'])

    def test_full_counts_survive_projection_overflow(self):
        source=[row(n+1,n*1024+10000) for n in range(70)]
        public=[row(n+1,n*1023) for n in range(70)]
        result=packet_clock({'completeSourceRows':source,'completePublicRows':public},boundary())
        self.assertEqual(result['clockChangeCount'],69)
        self.assertEqual(len(result['clockChanges']),64)
        self.assertTrue(result['clockChangeOverflow'])
        self.assertEqual(result['adjacentPTSChangeCount'],69)
        self.assertTrue(result['adjacentPTSChangeOverflow'])
        self.assertFalse(result['acceptance'])

    def test_unsupported_payload_shape_is_unavailable(self):
        self.assertFalse(packet_clock({},boundary())['observed'])

if __name__=='__main__':
    unittest.main()
