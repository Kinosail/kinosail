from pathlib import Path
import collections,hashlib,json,re,subprocess,sys

import argparse
parser=argparse.ArgumentParser(description='Read-only exact Git tree test audit inventory; no runtime coverage claim.')
parser.add_argument('sha',help='Exact integrated Git SHA to review')
parser.add_argument('--repo',type=Path,default=Path.cwd(),help='Checkout containing the reviewed commit and owner ledgers')
parser.add_argument('--output',type=Path,required=True,help='JSON receipt output path')
args=parser.parse_args()
repo=args.repo.resolve()
head=subprocess.check_output(['git','rev-parse',args.sha+'^{commit}'],cwd=repo,text=True).strip()
base='876b771a7dd0aef5e65957fcee87add0312535e8'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def at(path):return subprocess.check_output(['git','show',head+':'+path],cwd=repo)
paths=subprocess.check_output(['git','ls-tree','-r','--name-only',head,'--','apps/player','apps/subtitles','packages'],cwd=repo,text=True).splitlines()
testfiles=[p for p in paths if p.endswith('_test.go')]
inventory=[];byfile={};byname=collections.defaultdict(list)
for path in testfiles:
 raw=at(path);source=raw.decode();declarations=[]
 for m in re.finditer(r'^func ((?:Test|Benchmark|Fuzz)\w+)\(',source,re.M):
  row={'file':path,'name':m[1],'line':source[:m.start()].count('\n')+1};inventory.append(row);declarations.append(row);byname[m[1]].append(row)
 byfile[path]={'sha256':sha(raw),'declarations':declarations}
specs={'player-backend.json':'tests','subtitles-shared.json':'declarations_inventory','shared-identity.json':'declarations','player-nonserver.json':'tests'}
receipts=[];deletions=[];retained_missing=[];pending=[];candidates=[];references=[];seen={}
for fname,key in specs.items():
 ledger_path='engineering/qa/2026-10-03-test-overhaul/'+fname;raw=at(ledger_path);ledger=json.loads(raw);receipts.append({'file':ledger_path,'sha256':sha(raw)})
 for row in ledger[key]:
  path=row.get('file',row.get('path'));name=row.get('name',row.get('test'));decision=row.get('decision')
  if not path or not path.endswith('_test.go') or not name:continue
  current_name=row.get('renamed_to') or name
  entry={'ledger':fname,'file':path,'name':name,'current_name':current_name,'decision':decision};present=any(r['name']==current_name for r in byfile.get(path,{}).get('declarations',[]));entry['current_declaration_present']=present
  pair=(path,current_name)
  if pair in seen:
   if seen[pair]['decision']!=decision:candidates.append({'issue':'overlapping ledger decisions disagree','first':seen[pair],'second':entry})
  else:seen[pair]=entry
  if decision=='D':deletions.append(entry)
  elif decision in ('R','F') and not present:retained_missing.append(entry)
  elif decision is None:pending.append(entry)
  elif decision=='C':candidates.append({'issue':'C candidate requires final owner resolution','entry':entry})
  if decision=='D':
   proof={k:row.get(k) for k in ('remaining_proof','remaining_coverage','remaining_proof_or_gap','coverage_gap','coverage_limit','gap') if row.get(k)}
   for keeper in sorted(set(re.findall(r'\b(?:Test|Benchmark|Fuzz)[A-Z]\w*',json.dumps(proof)))):
    if keeper==name:continue
    references.append({'removed':entry,'named_reference':keeper,'current_matches':byname.get(keeper,[]),'proof':proof})
report={'schema':'kinosail-test-audit-integrity-v1','reviewer':'frontend_native','source_sha':head,'baseline_sha':base,'scope':'Independent read-only current Go declaration/removal/keeper inventory across tracked Player/Subtitles/packages tests','purpose_and_failure_analysis':['A keeper removed by another ledger can falsely justify a deletion; compare named proof references against exact current Git tree.','A D declaration may still exist or an R/F declaration may disappear; compare actual anchored Go declarations by exact path/name.','Overlapping owner ledgers may disagree; preserve and surface both, not choose silently.','A proof reference may describe old baseline, dependency source, intended deletion or a generic name in another package; missing-name reports are candidates for semantic review, not automatic blockers.','Pending/C and newly added tests are reported honestly; this inventory does not claim full semantic review or execution coverage.'],'test_files':len(testfiles),'current_declarations':len(inventory),'ledger_receipts':receipts,'decision_counts':dict(collections.Counter(e['decision'] for e in seen.values())),'D_still_present':[e for e in deletions if e['current_declaration_present']],'R_F_missing':retained_missing,'pending':pending,'candidate_or_overlap_resolution':candidates,'named_D_proof_references':references,'missing_named_D_proof_references':[r for r in references if not r['current_matches']],'unledgered_current_declarations':[r for r in inventory if (r['file'],r['name']) not in seen],'tracked_source_file_receipts':[{'file':p,'sha256':r['sha256'],'declaration_count':len(r['declarations'])} for p,r in byfile.items()],'current_go_inventory':inventory,'limits':['Function-name resolution is by all tracked current Go packages unless exact path given; app twins are not automatically equivalent.','Bench/Fuzz declarations are inventoried but not claimed ordinary Go test execution.','No builds, browser, native device or TLS bypass run.','Go proof string extraction does not verify semantic equivalence or true-vs-synthetic E2E execution.','No test/source/workflow files changed.'],'reproduce':{'command':'python3 '+Path(__file__).name+' '+head+' --repo <checkout> --output <receipt.json>','generator_sha256':sha(Path(__file__).read_bytes())}}
out=args.output;out.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({'path':str(out),'sha256':sha(out.read_bytes()),'source_sha':head,'files':len(testfiles),'current_declarations':len(inventory),'D_still_present':len(report['D_still_present']),'R_F_missing':len(retained_missing),'pending':len(pending),'C_or_overlap':len(candidates),'missing_named_D_proof_refs':len(report['missing_named_D_proof_references']),'unledgered_current_declarations':len(report['unledgered_current_declarations'])},indent=2))
