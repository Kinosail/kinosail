import json,unittest
from hls_presented_fixture import proof_enabled, source_frames, coded_filter, fixture_command

class PresentedFixtureTests(unittest.TestCase):
 def payload(self):
  return {'frames':[{'best_effort_timestamp_time':format(i/24,'.6f')} for i in range(768)]}
 def test_optional_flag_is_closed_before_any_operation(self):
  for value in [None,'0','1']:
   self.assertEqual(proof_enabled({} if value is None else {'KINOSAIL_HLS_PRESENTATION_PROOF':value}),value=='1')
  for value in ['', 'true','01','1\n','2',1,'1'*4096]:
   actions=[]
   with self.assertRaises(ValueError):
    proof_enabled({'KINOSAIL_HLS_PRESENTATION_PROOF':value});actions.append('build')
   self.assertEqual(actions,[])
 def test_exact_source_frame_map_independently_binds_target_and_preroll(self):
  frames=source_frames(json.dumps(self.payload()).encode())
  self.assertEqual(len(frames),768);self.assertEqual(frames[300],12.5);self.assertEqual(frames[288],12)
 def test_millisecond_source_clock_keeps_unique_frame_indices(self):
  value=self.payload()
  for i,frame in enumerate(value['frames']):frame['best_effort_timestamp_time']=format(round(i/24,3),'.3f')
  frames=source_frames(json.dumps(value).encode())
  self.assertEqual(frames[300],12.5);self.assertEqual(len(set(frames)),768)
 def test_unknown_duplicate_nonfinite_and_conflicting_maps_reject_before_use(self):
  values=[]
  for mutate in [lambda v:v.update(extra=1),lambda v:v['frames'].pop(),
    lambda v:v['frames'][300].update(best_effort_timestamp_time='NaN'),
    lambda v:v['frames'][300].update(best_effort_timestamp_time='12.0'),
    lambda v:v['frames'][300].update(extra=1)]:
   v=self.payload();mutate(v);values.append(json.dumps(v).encode())
  values.extend([b'{}',b'[]',b'\xff',b'x'*65537,b'{"frames":[],"frames":[]}'])
  for raw in values:
   actions=[]
   with self.assertRaises(ValueError):source_frames(raw);actions.append('open')
   self.assertEqual(actions,[])
 def test_marker_recipe_owns_frame_index_without_changing_source_clock(self):
  value=coded_filter()
  self.assertIn('N',value);self.assertIn('lum(X,Y)',value)
  self.assertNotIn('setpts',value);self.assertNotIn('fps=',value)
 def test_coded_fixture_fixes_eight_bit_filter_and_encoder_format(self):
  command=fixture_command('/owned/HLS Presented.mkv')
  self.assertEqual(command[command.index('-vf')+1],'format=yuv420p,'+coded_filter())
  self.assertEqual(command[command.index('-pix_fmt')+1],'yuv420p')
  self.assertEqual(command[command.index('-frames:v')+1],'768')
  self.assertEqual(command[command.index('-g')+1],'48')
  self.assertEqual(command[-1],'/owned/HLS Presented.mkv')
if __name__=='__main__':unittest.main()
