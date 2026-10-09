"""Raw MSE clock failure matrix written before implementation.
Gap: ffprobe timestamps alone cannot establish the Chromium edit/tfdt/trun map.
Keep every raw sample; reject unknown edit rate, bad count/clock and a guessed shift.
"""
import copy
import unittest
from hls_nonkey_browser_color_clock import mse_clock_facts

class ClockTests(unittest.TestCase):
    def facts(self):
        samples=[{'pts':0,'dts':-2,'duration':1},{'pts':1,'dts':-1,'duration':1}]
        packets=[{'stream_index':0,'pts_time':'-0.5','dts_time':'-1.5','duration_time':'0.5'},
                 {'stream_index':0,'pts_time':'0','dts_time':'-1','duration_time':'0.5'}]
        return {'initialization':{'tracks':[{'trackID':1,'handler':'vide','mediaTimescale':2,
            'edits':[{'mediaTime':1,'duration':0,'rateInteger':1,'rateFraction':0}]}]},
            'physicalFragments':[{'tracks':[{'trackID':1,'samples':samples}]}],
            'publicPacketRows':packets}
    def test_complete_raw_map_not_reinterpreted_or_trimmed(self):
        value=self.facts();before=copy.deepcopy(value)
        result=mse_clock_facts(value,12.5)
        self.assertEqual(result['firstClipPTSSeconds'],-0.5)
        self.assertEqual(result['firstSourcePTSSeconds'],12)
        self.assertEqual(result['sampleCount'],2)
        self.assertTrue(result['qualified'])
        self.assertEqual(value,before)
    def test_unknown_geometry_signedness_or_clock_never_guessed(self):
        for change in ['edit','rate','scale','count','composition','duration','nan','request']:
            value=self.facts();request=12.5
            if change=='edit':value['initialization']['tracks'][0]['edits']=[]
            if change=='rate':value['initialization']['tracks'][0]['edits'][0]['rateInteger']=0
            if change=='scale':value['initialization']['tracks'][0]['mediaTimescale']=0
            if change=='count':value['physicalFragments'][0]['tracks'][0]['samples'].pop()
            if change=='composition':value['physicalFragments'][0]['tracks'][0]['samples'][0]['pts']=2**32
            if change=='duration':value['physicalFragments'][0]['tracks'][0]['samples'][0]['duration']=0
            if change=='nan':value['publicPacketRows'][0]['pts_time']='nan'
            if change=='request':request=13
            with self.subTest(change=change),self.assertRaises(RuntimeError):
                mse_clock_facts(value,request)
if __name__=='__main__':unittest.main()
