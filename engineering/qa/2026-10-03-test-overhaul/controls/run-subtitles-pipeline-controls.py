#!/usr/bin/env python3
"""Repeat existing-test guard controls from the recorded checkout, restoring files."""
import argparse, hashlib, json, os, pathlib, platform, subprocess, time
parser=argparse.ArgumentParser();parser.add_argument('--repo',type=pathlib.Path,default=pathlib.Path(__file__).resolve().parents[4]);parser.add_argument('--output',type=pathlib.Path,required=True);args=parser.parse_args()
root=args.repo.resolve();out=args.output.resolve();out.mkdir(parents=True,exist_ok=False)
env=os.environ.copy();env['GOCACHE']='/tmp/kinosail-testing-go-cache'
def git(*cmd):return subprocess.check_output(['git','-C',str(root),*cmd])
def digest(data):return hashlib.sha256(data).hexdigest()
sha=git('rev-parse','HEAD').decode().strip();assert not git('diff','--name-only'), 'Start from a committed source tree'
base='ac4224661';server='apps/subtitles/internal/server/'
files={key:server+name for key,name in {'candidate_prod':'subsource_candidate.go','candidate_test':'subsource_provider_internal_test.go','preserve_prod':'subtitle_sync_piecewise.go','preserve_test':'subtitle_sync_behavior_internal_test.go'}.items()}
originals={key:(root/path).read_bytes() for key,path in files.items()}
old={key:git('show',base+':'+files[key]) for key in ['candidate_test','preserve_test']}
source=originals['candidate_prod'].decode()
block='''\tif !validSubSourceCandidateInput(movie, subtitle, language) {
\t\treturn subSourceCandidate{}, false
\t}
\trelease := strings.Join(subtitle.ReleaseInfo, " ")
\tif !validSubSourceRelease(item, subtitle, release) {
\t\treturn subSourceCandidate{}, false
\t}'''
assert source.count(block)==1
candidate=source.replace(block,'\trelease := strings.Join(subtitle.ReleaseInfo, " ")').encode()
assert source.count('count <= 50 && ')==1
response=source.replace('count <= 50 && ','').encode()
preserve_source=originals['preserve_prod'].decode();needle='''\tif candidate.Preserve {
\t\tif changed {''';assert preserve_source.count(needle)==1
preserve=preserve_source.replace(needle,'''\tif candidate.Preserve {
\t\tif false && changed {''').encode()
manifest={'schema':1,'sha':sha,'old_test_source_sha':git('rev-parse',base).decode().strip(),'command':['python3',str(pathlib.Path(__file__).resolve()),'--repo',str(root),'--output',str(out)],'environment':{'platform':platform.platform(),'machine':platform.machine(),'python':platform.python_version(),'GOCACHE':env['GOCACHE'],'go_version':subprocess.check_output(['go','version'],env=env).decode().strip(),'go_env':json.loads(subprocess.check_output(['go','env','-json','GOOS','GOARCH','CGO_ENABLED','GOVERSION','GOMOD'],cwd=root/'apps/subtitles',env=env))},'source_checksums':{files[k]:digest(v) for k,v in originals.items()},'boundary':'Existing isolated deterministic provider/timing controls; not genuine E2E or real speech extraction. Production guard mutations are transient and restored byte for byte.','phases':[]}
phases=[('candidate-old',{'candidate_prod':candidate,'candidate_test':old['candidate_test']},'TestSubSourceRejectsUnsafeOrAmbiguousResults',0),('candidate-repaired',{'candidate_prod':candidate},'TestSubSourceRejectsUnsafeOrAmbiguousResults',1),('response-limit-old',{'candidate_prod':response,'candidate_test':old['candidate_test']},'TestSubSourceRejectsUnsafeOrAmbiguousResults',0),('response-limit-repaired',{'candidate_prod':response},'TestSubSourceRejectsUnsafeOrAmbiguousResults',1),('preserve-old',{'preserve_prod':preserve,'preserve_test':old['preserve_test']},'TestSynchronizeCandidateRejectsPreservedSubtitle',0),('preserve-repaired',{'preserve_prod':preserve},'TestSynchronizeCandidateRejectsPreservedSubtitle',1),('restored-full-package',{},None,0)]
try:
 for name,changes,test,expected in phases:
  for k,data in originals.items():(root/files[k]).write_bytes(data)
  for k,data in changes.items():(root/files[k]).write_bytes(data)
  patch=git('diff','--binary');(out/(name+'.patch')).write_bytes(patch)
  cmd=['go','test','-json','-count=1']
  if test:cmd+=['-run','^'+test+'$']
  cmd+=['./internal/server'];start=time.time()
  with open(out/(name+'.jsonl'),'wb') as stdout,open(out/(name+'.stderr'),'wb') as stderr:
   result=subprocess.run(cmd,cwd=root/'apps/subtitles',env=env,stdout=stdout,stderr=stderr)
  events=[json.loads(line) for line in (out/(name+'.jsonl')).read_text().splitlines()]
  row={'phase':name,'sha':sha,'cwd':str(root/'apps/subtitles'),'command':cmd,'environment':{'GOCACHE':env['GOCACHE']},'exit_code':result.returncode,'expected_exit_code':expected,'seconds':round(time.time()-start,3),'files':{files[k]:digest((root/files[k]).read_bytes()) for k in originals},'patch_sha256':digest(patch),'test_results':[e for e in events if e.get('Action') in ['pass','fail','skip']],'logs':[name+'.jsonl',name+'.stderr',name+'.patch']};manifest['phases'].append(row)
  (out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(name,result.returncode,'expected',expected,flush=True)
  assert result.returncode==expected,(name,result.returncode,expected)
  if expected:
   assert any(e.get('Action')=='fail' and e.get('Test')==test for e in events),(name,'intended existing test did not fail')
finally:
 for k,data in originals.items():(root/files[k]).write_bytes(data)
 manifest['restored_source_checksums']={files[k]:digest((root/files[k]).read_bytes()) for k in originals}
 manifest['source_restored_exactly']=manifest['restored_source_checksums']==manifest['source_checksums']
 manifest['final_git_diff']=git('diff','--name-only').decode()
 (out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
 with open(out/'SHA256SUMS','w') as checks:
  for path in sorted(out.iterdir()):
   if path.is_file() and path.name!='SHA256SUMS':checks.write(digest(path.read_bytes())+'  '+path.name+'\n')
