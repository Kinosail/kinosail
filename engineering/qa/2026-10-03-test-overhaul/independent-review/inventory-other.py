from pathlib import Path
import re,json,subprocess,hashlib,collections,sys
import argparse
parser=argparse.ArgumentParser(description='Read-only exact Git tree test audit inventory; no runtime coverage claim.')
parser.add_argument('sha',help='Exact integrated Git SHA to review')
parser.add_argument('--repo',type=Path,default=Path.cwd(),help='Checkout containing the reviewed commit and owner ledgers')
parser.add_argument('--output',type=Path,required=True,help='JSON receipt output path')
args=parser.parse_args()
repo=args.repo.resolve()
head=subprocess.check_output(['git','rev-parse',args.sha+'^{commit}'],cwd=repo,text=True).strip()
base='876b771a7dd0aef5e65957fcee87add0312535e8'
def digest(b):return hashlib.sha256(b).hexdigest()
cache={}
def blob(sha,p):
 key=(sha,p)
 if key not in cache:
  x=subprocess.run(['git','show',sha+':'+p],cwd=repo,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
  cache[key]=None if x.returncode else x.stdout
 return cache[key]
def names(p,source):
 if source is None:return []
 if p.endswith('.sh'):return ['script scenario']
 s=source.decode();ext=Path(p).suffix
 if ext in ('.ts','.mjs','.js'):
  return [re.sub(r'\\([\'"`\\])',r'\1',m['name']) for m in re.finditer(r'\btest(?:\.\w+)*\(\s*(?P<quote>[\'"`])(?P<name>(?:\\.|(?!(?P=quote)).)*)(?P=quote)',s,re.S)]
 if ext in ('.swift','.kt'):return re.findall(r'\bfunc?\s+`?([\w ]+)`?\s*\(',s)
 if ext=='.py':return re.findall(r'^\s*def\s+(test_\w+)\(',s,re.M)
 if ext=='.go':return re.findall(r'^func\s+((?:Test|Benchmark|Fuzz)\w+)\(',s,re.M)
 return ['script scenario']
receipts=[];rows=[]
for filename,key in [('frontend-native.json','declarations'),('frontend-player-fixture-e2e.json','declarations'),('frontend-subtitles-fixture-e2e.json','declarations'),('tooling.json','tests')]:
 p='engineering/qa/2026-10-03-test-overhaul/'+filename;b=blob(head,p);x=json.loads(b);receipts.append({'file':p,'sha256':digest(b)})
 scopes=[(filename,x[key])]
 if filename=='frontend-native.json':scopes.append((filename+':android_audit',x['android_audit']['declarations']))
 for label,decls in scopes:
  for r in decls:
   p=r.get('file',r.get('path'));n=r.get('name',r.get('test'));original=blob(base,p);current=blob(head,p);bn=names(p,original);cn=names(p,current)
   def match(n,ns):
    if n=='script scenario':return True if ns or original is not None else False
    if n in ns:return True
    for template in ns:
     pieces=re.split(r'\$\{[^}]*\}',template)
     if len(pieces)>1 and re.fullmatch('.*?'.join(re.escape(v) for v in pieces),n):return True
    return False
   before=match(n,bn);after=current is not None and match(n,cn)
   same=original is not None and original==current
   mode='exact_or_source_template' if before else 'unchanged_full_source' if same else 'unresolved'
   if same and not before:before=after=True
   rows.append({'ledger':label,'file':p,'name':n,'decision':r.get('decision'),'baseline_parsed_declaration_present':before,'current_parsed_declaration_present':after,'presence_mode':mode,'current_file_present':current is not None,'baseline_file_sha256':digest(original) if original is not None else None,'current_file_sha256':digest(current) if current is not None else None,'remaining_proof':r.get('remaining_proof',r.get('remaining_proof_or_gap'))})
explicit={
'packages/webassets/theme.test.mjs':['invalid appearance changes do not alter the page or persisted preference'],
'apps/player/apps/android/app/src/test/java/com/kinosail/player/core/CastButtonThemeTest.kt':['playerThemeCanCreateCastButton'],
'apps/player/e2e/player-subtitle-fullscreen.spec.ts':['native fullscreen fallback follows Safari entry and exit events'],
'apps/player/e2e/service-worker-storage-contract.spec.ts':[],
'apps/player/e2e/title-jump-first-paint.spec.ts':[],
'apps/subtitles/e2e/subtitle-inspector-races.spec.ts':[],
'apps/subtitles/e2e/subtitle-inspector-layout.spec.ts':[],
'apps/subtitles/e2e/subtitle-defaults-tests.ts':[],
'apps/player/scripts/test-native-contract.py':[],
'apps/subtitles/scripts/test-native-contract.py':[],
}
keepers=[]
for p,ns in explicit.items():
 b=blob(head,p);ks=names(p,b);keepers.append({'file':p,'file_present':b is not None,'sha256':digest(b) if b else None,'requested_names':ns,'parsed_names':ks,'requested_names_missing':[n for n in ns if n not in ks]})
report={'schema':'kinosail-test-audit-integrity-v1','reviewer':'frontend_native','source_sha':head,'baseline_sha':base,'scope':'Read-only physical declaration inventory for frontend/native Android and tooling ledgers; no runtime/semantic equivalence claim','rows':rows,'counts':dict(collections.Counter(r['decision'] for r in rows)),'D_still_present':[r for r in rows if r['decision']=='D' and r['current_parsed_declaration_present']],'R_F_missing':[r for r in rows if r['decision'] in ('R','F') and r['baseline_parsed_declaration_present'] and not r['current_parsed_declaration_present']],'unresolved_candidates':[r for r in rows if r['decision']=='C'],'baseline_name_not_resolved':[r for r in rows if not r['baseline_parsed_declaration_present']],'explicit_keeper_receipts':keepers,'ledger_receipts':receipts,'limits':['Regex inventory only; generated case names match retained source templates; unchanged full source proves physical retention when source names are parameter values. Swift/Kotlin presence uses named function.','Script scenarios are full executable files, not parsed inner test cases.','Baseline-name unresolved entries need explicit actual source comparison; they are not silently marked preserved.','No builds/browser/device/TLS bypass.','Native platform runtime preservation is not inferred from source presence.'],'reproduce':{'command':'python3 '+Path(__file__).name+' '+head+' --repo <checkout> --output <receipt.json>','generator_sha256':digest(Path(__file__).read_bytes())}}
out=args.output;out.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'path':str(out),'sha256':digest(out.read_bytes()),'source_sha':head,'rows':len(rows),'counts':report['counts'],'D_still_present':len(report['D_still_present']),'R_F_missing':len(report['R_F_missing']),'baseline_name_not_resolved':len(report['baseline_name_not_resolved']),'explicit_keeper_missing':[r for r in keepers if not r['file_present'] or r['requested_names_missing']]}))
