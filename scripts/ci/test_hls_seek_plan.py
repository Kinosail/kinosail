"""Real fixture-plan response admission; no server or encoder execution."""
import copy
import json
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'apps/player/scripts'))
from hls_followon_public import seek_plan

class SeekPlanTests(unittest.TestCase):
    def test_public_seek_contract_documents_bounds_and_prior_recipe_correlation(self):
        document=json.loads((ROOT/'apps/player/internal/server/api_openapi.json').read_text())
        parameters=document['paths']['/api/v1/items/{id}/playback']['get']['parameters']
        names=[p['name'] for p in parameters]
        self.assertEqual(names.count('position'),1)
        self.assertEqual(names.count('recipe'),1)
        position=next(p for p in parameters if p['name']=='position')
        self.assertEqual(position['schema'],{'type':'number','minimum':0,'maximum':604800,'multipleOf':0.1})
        self.assertIn('less than the media duration',position['description'])
        recipe=next(p for p in parameters if p['name']=='recipe')
        self.assertEqual(recipe['schema']['maxLength'],2048)
        self.assertIn('position',recipe['description'])
        self.assertIn('zero offset',recipe['description'])

    def fixture(self):
        return {'compatible': '/hls/0123456789abcdef/p/t-a0-s0-none-t0-b0-z640x360/index.m3u8',
            'compatiblePlan': {'allowed': True, 'mode': 'transcode', 'audioIndex': 0},
            'compatibleLabel': 'Transcoding video', 'duration': 32.021,
            'media': {'fileVersion': '6:123'}, 'qualities': [{'label': '360p'}, {'label': '180p'}]}

    def attempt(self, result=None, raw=None, item='0123456789abcdef', position=12.5, mode='transcode'):
        calls = []
        self.calls = calls
        class API:
            def http(self, path):
                calls.append(path)
                return 200, raw if raw is not None else json.dumps(result).encode(), {}
        value = seek_plan(API(), item, position, '6:123', 32.021, mode)
        return calls, value

    def test_actual_response_preserves_truthful_mode_source_duration_and_qualities(self):
        calls, value = self.attempt(self.fixture())
        self.assertEqual(calls, ['/api/v1/items/0123456789abcdef/playback?videoCodecs=h264&audioCodecs=aac&position=12.5'])
        self.assertEqual(value['mode'], 'transcode')
        self.assertEqual(value['qualities'], ['360p', '180p'])
        self.assertIn('/p/t-', value['source'])

    def test_certified_key_response_keeps_remux_without_optional_quality_array(self):
        result=self.fixture();result.pop('qualities')
        result['compatible']='/hls/0123456789abcdef/p/r-a0-s0-none-t0-b0/index.m3u8'
        result['compatiblePlan']['mode']='remux';result['compatibleLabel']='Remux'
        _, value=self.attempt(result, position=12, mode='remux')
        self.assertEqual(value['mode'],'remux')
        self.assertEqual(value['qualities'],[])

    def test_invalid_authority_or_position_is_rejected_before_any_http_effect(self):
        for item, position in [('unknown', 12.5), ('0'*17, 12.5), ('0123456789abcdef', True),
            ('0123456789abcdef', float('nan')), ('0123456789abcdef', -1),
            ('0123456789abcdef', 12.55), ('0123456789abcdef', 32.021)]:
            with self.subTest(item=item, position=position):
                with self.assertRaises(RuntimeError):self.attempt(self.fixture(), item=item, position=position)
                self.assertEqual(self.calls, [])

    def test_wrong_missing_ambiguous_or_oversized_response_never_qualifies_delivery(self):
        edits = [lambda r:r.pop('compatible'), lambda r:r.update(compatible='https://foreign.invalid/private'),
            lambda r:r.update(compatible=r['compatible'].replace('t-a0', 'r-a0')),
            lambda r:r['compatiblePlan'].update(mode='unknown'), lambda r:r['compatiblePlan'].update(allowed=1),
            lambda r:r['compatiblePlan'].update(audioIndex=True), lambda r:r.update(duration=True),
            lambda r:r.update(duration=31), lambda r:r['media'].update(fileVersion='other'),
            lambda r:r.update(compatibleLabel='Remux'), lambda r:r.update(qualities=[]),
            lambda r:r.update(qualities=[{'label':'unknown'}]), lambda r:r.update(qualities=[{'label':'360p'}]*9),
            lambda r:r.update(qualities=[{'label':'360p'}]*2)]
        for edit in edits:
            value=copy.deepcopy(self.fixture());edit(value)
            with self.subTest(edit=edit), self.assertRaises(RuntimeError):self.attempt(value)
        for raw in [b'', b'\xff', b'{"duration":32,"duration":32}', b'x'*(2*1024*1024+1)]:
            with self.subTest(size=len(raw)), self.assertRaises(RuntimeError):self.attempt(raw=raw)

if __name__ == '__main__': unittest.main()
