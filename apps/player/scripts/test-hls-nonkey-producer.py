#!/usr/bin/env python3
"""Bounded fresh-reference test of the canonical private non-key producer seam."""
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from hls_followon_public import bounded_bytes, check
from hls_timeline_http import sha, source_state
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_remaining_process import join_group

ROOT=Path(__file__).resolve().parents[3]
RUN=ROOT/'.verification/hls-nonkey-producer'/time.strftime('%Y%m%dT%H%M%SZ',time.gmtime())
RUN.mkdir(parents=True)
receipt={'revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
    'tree':subprocess.check_output(['git','rev-parse','HEAD^{tree}'],text=True).strip(),
    'result':'failed','productionAcceptance':False,'certificateAcceptance':False,
    'boundary':'Actual canonical private worker only; no public API, browser, cache or generation admission'}
guard=DiagnosticDeadline(660)
guard.__enter__()
sources=[]
def run(argv,timeout,bound):
    guard.check()
    result=subprocess.run(argv,capture_output=True,timeout=timeout)
    check(len(result.stdout)<=bound,'producer_command_bound')
    check(result.returncode==0,'producer_command_failed')
    return result.stdout
try:
    receipt['currentStage']='strict-reference-cli'
    run([sys.executable,str(ROOT/'apps/player/scripts/test-hls-nonkey-supported-followon.py')],300,2<<20)
    paths=list((ROOT/'.verification/hls-nonkey-mux').glob('seek-followon-*/receipt.json'))
    check(len(paths)==1,'producer_fresh_reference_selection')
    cli=json.loads(bounded_bytes(paths[0],32<<20,'producer_fresh_reference_bound'))
    sources=[(Path(v['path']),v['state']) for v in cli['sources']]
    receipt['referenceReceiptSHA256']=sha(paths[0])
    reference=RUN/'reference.json'
    run([sys.executable,str(ROOT/'apps/player/scripts/hls_nonkey_producer_reference.py'),str(paths[0]),str(reference)],15,65536)
    receipt['referenceSHA256']=sha(reference)
    receipt['currentStage']='test-format-control'
    test=ROOT/'apps/player/internal/server/hls_remaining_nonkey_producer_test.go'
    unformatted=run(['gofmt','-l',str(test)],10,65536)
    receipt['testSourceFormatted']=not unformatted.strip()
    check(receipt['testSourceFormatted'],'producer_test_source_unformatted')
    receipt['currentStage']='actual-canonical-producer'
    env=dict(os.environ,KINOSAIL_COPIED_RECOVERY_MEDIA='1',KINOSAIL_NONKEY_CLI_REFERENCE=str(reference))
    command=['go','-C',str(ROOT/'apps/player'),'test','-count=1','-p=1','-json','./internal/server',
        '-run','^TestRemainingNonKeyActualPendingProducer$','-timeout','180s']
    process=subprocess.Popen(command,cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
    try:
        stdout,stderr=process.communicate(timeout=min(240,guard.check(25)))
        check(len(stdout)<=2<<20 and len(stderr)<=65536,'producer_test_output_bound')
        events=[json.loads(line) for line in stdout.decode().splitlines() if line.startswith('{')]
        receipt['completeGoEvents']=events
        receipt['goStderrSHA256']=__import__('hashlib').sha256(stderr).hexdigest()
        receipt['goReturnCode']=process.returncode
        receipt['tests']=[{'test':v.get('Test'),'action':v['Action']} for v in events if v.get('Test') and v['Action'] in ['pass','fail','skip']]
        receipt['safeGoMessages']=[v['Output'].strip() for v in events if v.get('Output') and
            ('pending producer ' in v['Output'] or 'nonkey actual-pending-producer ' in v['Output'])][:64]
    finally:
        with guard.cleanup():
            receipt['ownedGoProcessJoin']=join_group(process)
        check(receipt['ownedGoProcessJoin'].get('confirmedZeroSamples')==2 and
            not receipt['ownedGoProcessJoin'].get('qualificationFailures'),'producer_owned_go_join')
    check(process.returncode==0,'producer_actual_canonical_regression')
    receipt['result']='observed'
    receipt['currentStage']='complete'
except Exception as error:
    receipt['failureClass']=str(error) if isinstance(error,RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['sourcesUnchanged']=all(source_state(path)==state for path,state in sources)
        if not receipt['sourcesUnchanged']:
            receipt['result']='failed'
            receipt['sourceFailureClass']='producer_immutable_source_changed'
        receipt['handledTerminationSignals']=guard.signals
        files=[Path(__file__),ROOT/'apps/player/scripts/hls_nonkey_producer_reference.py',
            ROOT/'apps/player/internal/server/hls_remaining_nonkey_producer_test.go']
        receipt['executedSourceSHA256']={str(p.relative_to(ROOT)):sha(p) for p in files}
        raw=json.dumps(receipt,separators=(',',':'),allow_nan=False)+'\n'
        check(0<len(raw.encode())<=4<<20,'producer_receipt_bound')
        target=RUN/'receipt.json'
        target.write_text(raw)
        (RUN/'SHA256SUMS').write_text(sha(target)+'  receipt.json\n'+''.join(sha(p)+'  '+str(p.relative_to(ROOT))+'\n' for p in files))
        print(json.dumps({k:receipt.get(k) for k in ['revision','tree','result','failureClass','currentStage',
            'testSourceFormatted','goReturnCode','tests','safeGoMessages','sourcesUnchanged','ownedGoProcessJoin',
            'referenceReceiptSHA256','referenceSHA256','productionAcceptance','certificateAcceptance']}|
            {'receiptSHA256':sha(target)}),flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result']=='observed' else 1)
