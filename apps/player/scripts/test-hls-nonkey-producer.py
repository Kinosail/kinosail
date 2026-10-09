#!/usr/bin/env python3
"""Bounded fresh-reference test of the canonical private non-key producer seam."""
import hashlib
import json
import os
import re
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

def retain_go_output(stdout,stderr,complete):
    stdout,stderr=stdout or b'',stderr or b''
    check(len(stdout)<=2<<20 and len(stderr)<=65536,'producer_test_output_bound')
    events=[]
    for line in stdout.decode(errors='replace').splitlines():
        if not line.startswith('{'):continue
        try:events.append(json.loads(line))
        except json.JSONDecodeError:
            check(not complete,'producer_complete_go_json')
    receipt['goOutputComplete']=complete
    receipt['completeGoEvents' if complete else 'partialGoEvents']=events
    if not complete:
        receipt['partialGoStdout']=stdout.decode(errors='replace')
        receipt['partialGoStderr']=stderr.decode(errors='replace')
    receipt['goStdoutSHA256']=hashlib.sha256(stdout).hexdigest()
    receipt['goStderrSHA256']=hashlib.sha256(stderr).hexdigest()
    receipt['goStderrBytes']=len(stderr)
    compile_lines=[v.get('Output','') for v in events]+stderr.decode(errors='replace').splitlines()
    receipt['safeGoCompilerMessages']=[line.strip()[:1000] for line in compile_lines if re.search(r'hls_remaining_nonkey_producer(?:_packets|_arguments)?_test\.go:[0-9]+:',line)][:64]
    receipt['tests']=[{'test':v.get('Test'),'action':v['Action']} for v in events if v.get('Test') and v['Action'] in ['pass','fail','skip']]
    receipt['safeGoMessages']=[v['Output'].strip() for v in events if v.get('Output') and
        ('pending producer ' in v['Output'] or 'nonkey actual-pending-producer ' in v['Output'])][:64]

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
    formatted_files=[test]+[ROOT/'apps/player/internal/server'/p for p in [
        'hls_remaining_nonkey_producer_packets_test.go','hls_remaining_nonkey_producer_arguments_test.go',
        'hls_copied_startup.go','hls_copied_clock.go','hls_copied_segment_arguments.go','hls.go']]
    unformatted=run(['gofmt','-l',*[str(p) for p in formatted_files]],10,65536)
    receipt['testSourceFormatted']=not unformatted.strip()
    if not receipt['testSourceFormatted']:
        receipt['safeTestFormattingDiff']=run(['gofmt','-d',*[str(p) for p in formatted_files]],10,65536).decode()
    check(receipt['testSourceFormatted'],'producer_test_source_unformatted')
    receipt['currentStage']='actual-canonical-producer'
    env=dict(os.environ,KINOSAIL_COPIED_RECOVERY_MEDIA='1',KINOSAIL_NONKEY_CLI_REFERENCE=str(reference))
    command=['go','-C',str(ROOT/'apps/player'),'test','-count=1','-p=1','-json','./internal/server',
        '-run','^TestRemainingNonKey(ActualPendingProducer|Producer.*)$','-timeout','180s']
    process=subprocess.Popen(command,cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
    try:
        stdout,stderr=process.communicate(timeout=min(240,guard.check(25)))
        retain_go_output(stdout,stderr,True)
        receipt['goReturnCode']=process.returncode
    except subprocess.TimeoutExpired as error:
        receipt['primaryFailureClass']='TimeoutExpired'
        retain_go_output(error.stdout,error.stderr,False)
        raise
    finally:
        primary_failure=sys.exc_info()[1]
        cleanup_failed=False
        with guard.cleanup():
            try:receipt['ownedGoProcessJoin']=join_group(process)
            except Exception:
                receipt['ownedGoCleanupFailureClass']='producer_owned_go_join_failed'
                cleanup_failed=True
        join=receipt.get('ownedGoProcessJoin',{})
        cleanup_failed=cleanup_failed or join.get('confirmedZeroSamples')!=2 or bool(join.get('qualificationFailures'))
        if cleanup_failed:
            receipt['ownedGoCleanupFailureClass']='producer_owned_go_join_failed'
            if primary_failure is None:raise RuntimeError('producer_owned_go_join_failed')
    check(process.returncode==0,'producer_actual_canonical_regression')
    receipt['result']='observed'
    receipt['currentStage']='complete'
except Exception as error:
    receipt['failureClass']=str(error) if isinstance(error,RuntimeError) else type(error).__name__
finally:
    with guard.cleanup():
        receipt['sourceBindingEstablished']=len(sources)==2
        receipt['sourceObservations']=[]
        for path,state in sources:
            try:receipt['sourceObservations'].append({'sourceSHA256':state['sha256'],'unchanged':source_state(path)==state})
            except Exception as error:
                receipt['sourceObservations'].append({'sourceSHA256':state['sha256'],'unchanged':False,'observationFailureClass':type(error).__name__})
        receipt['sourcesUnchanged']=len(sources)==2 and all(v['unchanged'] for v in receipt['sourceObservations'])
        if not receipt['sourcesUnchanged']:
            receipt['result']='failed'
            receipt['sourceFailureClass']='producer_immutable_source_changed' if sources else 'producer_source_binding_unavailable'
        receipt['handledTerminationSignals']=guard.signals
        files=[Path(__file__),ROOT/'apps/player/scripts/hls_nonkey_producer_reference.py',
            ROOT/'apps/player/internal/server/hls_remaining_nonkey_producer_test.go',
            ROOT/'apps/player/internal/server/hls_remaining_nonkey_producer_packets_test.go',
            ROOT/'apps/player/internal/server/hls_remaining_nonkey_producer_arguments_test.go',
            ROOT/'apps/player/internal/server/hls_copied_startup.go',
            ROOT/'apps/player/internal/server/hls_copied_clock.go',
            ROOT/'apps/player/internal/server/hls_copied_segment_arguments.go',
            ROOT/'apps/player/internal/server/hls.go']
        receipt['executedSourceSHA256']={str(p.relative_to(ROOT)):sha(p) for p in files}
        raw=json.dumps(receipt,separators=(',',':'),allow_nan=False)+'\n'
        check(0<len(raw.encode())<=4<<20,'producer_receipt_bound')
        target=RUN/'receipt.json'
        target.write_text(raw)
        (RUN/'SHA256SUMS').write_text(sha(target)+'  receipt.json\n'+''.join(sha(p)+'  '+str(p.relative_to(ROOT))+'\n' for p in files))
        print(json.dumps({k:receipt.get(k) for k in ['revision','tree','result','failureClass','currentStage',
            'testSourceFormatted','safeTestFormattingDiff','goReturnCode','goOutputComplete','tests','safeGoMessages','safeGoCompilerMessages','sourcesUnchanged','sourceObservations','ownedGoProcessJoin','ownedGoCleanupFailureClass',
            'referenceReceiptSHA256','referenceSHA256','productionAcceptance','certificateAcceptance']}|
            {'receiptSHA256':sha(target)}),flush=True)
    guard.__exit__()
raise SystemExit(0 if receipt['result']=='observed' else 1)
