"""Reproduce the read-only audit's file/declaration provenance from its exact baseline."""
import argparse,collections,hashlib,json,re,subprocess
from pathlib import Path

def declaration_end(text,start):
 pos=text.index('{',start);depth=0;quote=None;comment=None
 while pos<len(text):
  char=text[pos];pair=text[pos:pos+2]
  if comment=='line':
   if char=='\n':comment=None
  elif comment=='block':
   if pair=='*/':comment=None;pos+=1
  elif quote:
   if char=='\\' and quote!='`':pos+=1
   elif char==quote:quote=None
  elif pair=='//':comment='line';pos+=1
  elif pair=='/*':comment='block';pos+=1
  elif char in '`"\'':quote=char
  elif char=='{':depth+=1
  elif char=='}':
   depth-=1
   if depth==0:return pos+1
  pos+=1
 raise ValueError('Unclosed Go declaration')

def verify(checkout,packet):
 d=json.loads(packet.read_text());base=d['baseline_sha'];identities=set();bodycount=0
 if 'tests' in d:return verify_slice(checkout,packet,d)
 for f in d['files']:
  content=subprocess.check_output(['git','show',f"{base}:{f['path']}"],cwd=checkout)
  assert hashlib.sha256(content).hexdigest()==f['baseline_sha256'],f['path']
  assert hashlib.sha256((checkout/f['path']).read_bytes()).hexdigest()==f['baseline_sha256'],f['path']
  text=content.decode();found={}
  for match in re.finditer(r'^func ((?:Test|Fuzz|Benchmark)\w+)\(',text,re.M):
   body=text[match.start():declaration_end(text,match.start())].encode()
   found[match[1]]=(text[:match.start()].count('\n')+1,hashlib.sha256(body).hexdigest())
  rows=[r for r in d['declarations'] if r['file']==f['path']]
  assert set(found)=={r['name'] for r in rows},f['path']
  for row in rows:
   identity=(row['file'],row['name']);assert identity not in identities;identities.add(identity)
   assert found[row['name']]==(row['line'],row['body_sha256']),identity
   assert row['audit_status']=='evidence_complete',identity
   assert all(row[k] for k in ['credible_failure','production_owner_paths','production_callers','remaining_coverage','coverage_gap','history','validation_command']),identity
   bodycount+=1
 counts=dict(collections.Counter(row['decision'] for row in d['declarations']))
 assert bodycount==len(d['declarations']) and counts==d['counts']['decisions']
 # Check the captured baseline execution receipt if available, without running Go.
 receipts=[]
 for artifact in d['baseline_execution_provenance']['artifacts']:
  path=Path(artifact['path']);present=path.is_file();ok=None
  if present:ok=hashlib.sha256(path.read_bytes()).hexdigest()==artifact['sha256'];assert ok,path
  receipts.append({'path':str(path),'present':present,'checksum_matches':ok})
 return {'verification':'passed','baseline_sha':base,'declaration_count':bodycount,'test_file_count':len(d['files']),'decisions':counts,'baseline_receipts':receipts,'packet_sha256':hashlib.sha256(packet.read_bytes()).hexdigest(),'execution':'Read-only checksum validation. No Go or E2E execution.'}

def verify_slice(checkout,packet,d):
 """Verify explicitly assigned declarations without claiming every sibling was assigned."""
 base=d['baseline_sha'];identities=set();terminal=collections.Counter()
 assert set(d['source_test_files'])=={r['path'] for r in d['tests']}
 for path,file in d['source_test_files'].items():
  content=subprocess.check_output(['git','show',f'{base}:{path}'],cwd=checkout)
  checksum=hashlib.sha256(content).hexdigest()
  assert checksum==file['baseline_sha256'],path
  assert hashlib.sha256((checkout/path).read_bytes()).hexdigest()==checksum,path
  text=content.decode();found={}
  for match in re.finditer(r'^func ((?:Test|Fuzz|Benchmark)\w+)\(',text,re.M):
   body=text[match.start():declaration_end(text,match.start())].encode()
   found[match[1]]=(text[:match.start()].count('\n')+1,hashlib.sha256(body).hexdigest())
  rows=[r for r in d['tests'] if r['path']==path]
  assert set(file['assigned_declarations'])=={r['name'] for r in rows},path
  for row in rows:
   identity=(path,row['name']);assert identity not in identities;identities.add(identity)
   assert found[row['name']]==(row['line'],row['body_sha256']),identity
   assert row['audit_status']==row['review_status']=='evidence_complete',identity
   assert all(row[k] for k in ['credible_failure','production_owner_paths','production_callers','remaining_coverage','coverage_gap','history','validation_command']),identity
   assert row['body_helpers_and_owner_read'] is True,identity
   assert row['baseline_status']==row['baseline_terminal_event']['Action'],identity
   terminal[row['baseline_status']]+=1
 for path,checksum in d['source_owner_hashes'].items():
  content=subprocess.check_output(['git','show',f'{base}:{path}'],cwd=checkout)
  assert hashlib.sha256(content).hexdigest()==checksum,path
 assert len(identities)==d['completed']==d['counts']['completed']==len(d['tests'])
 assert d['pending']==d['counts']['pending']==0
 counts=dict(collections.Counter(r['decision'] for r in d['tests']))
 assert counts==d['counts']['completed_decisions']
 assert dict(terminal)==d['counts']['baseline_terminal_statuses']
 receipts=[]
 for artifact in d['baseline_execution_provenance']['artifacts']:
  path=Path(artifact['path']);present=path.is_file();ok=None
  if present:ok=hashlib.sha256(path.read_bytes()).hexdigest()==artifact['sha256'];assert ok,path
  receipts.append({'path':str(path),'present':present,'checksum_matches':ok})
 return {'verification':'passed','baseline_sha':base,'declaration_count':len(identities),'test_file_count':len(d['source_test_files']),'source_owner_files_verified':len(d['source_owner_hashes']),'decisions':counts,'baseline_terminal_statuses':dict(terminal),'baseline_receipts':receipts,'packet_sha256':hashlib.sha256(packet.read_bytes()).hexdigest(),'execution':'Read-only assigned-declaration/body/owner/receipt checksum validation. No Go or E2E execution.'}

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--checkout',type=Path,required=True);p.add_argument('--packet',type=Path,required=True);a=p.parse_args();print(json.dumps(verify(a.checkout,a.packet),indent=2))
