#!/usr/bin/env python3
"""Only the fixed synthetic ordinary-cold producer receives a diagnostic recipe."""
import hashlib
import json
import os
from pathlib import Path
import sys
from hls_nonkey_browser_config import ordinary_cold_arguments

config_path = Path(os.environ['KINOSAIL_BROWSER_PRODUCER_CONFIG'])
raw = config_path.read_bytes()
if len(raw) > 65536:
    raise SystemExit('diagnostic_config_bound')
config = json.loads(raw)
arguments = sys.argv[1:]
original = list(arguments)
# Indexed/speculative jobs retain the published producer unchanged.
eligible = ordinary_cold_arguments(arguments,config['source'])
if eligible:
    selected = float(arguments[arguments.index('-ss')+1])
    cases = [v for v in config['cases'] if abs(v['request']-selected) <= 0.000001]
    if len(cases) != 1:
        raise SystemExit('diagnostic_request_not_fixed')
    case = cases[0]
    arguments[arguments.index('-ss')+1] = str(case['inputSeek'])
    # Remove only timestamp options whose replacement is measured for this fixture.
    for option in ['-avoid_negative_ts', '-output_ts_offset', '-copypriorss', '-copypriorss:v']:
        while option in arguments:
            number = arguments.index(option)
            del arguments[number:number+2]
    last = len(arguments)-1
    arguments[last:last] = ['-copypriorss:v', '0', '-avoid_negative_ts', 'disabled',
                           '-output_ts_offset', str(-case['delta'])]
    options = 'movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1'
    if '-hls_segment_options' in arguments:
        arguments[arguments.index('-hls_segment_options')+1] = options
    else:
        arguments[-1:-1] = ['-hls_segment_options', options]
# Retain both adapted and bypassed fixed-source HLS workers without private argv.
if '-hls_time' in original and '-i' in original and original[original.index('-i')+1]==config['source']:
    def option(arguments,name,default=None):
        return arguments[arguments.index(name)+1] if name in arguments else default
    evidence={'pid':os.getpid(),'parent':os.getppid(),'adapted':eligible,
        'sourceMatched':True,'indexedJobChanged':False,
        'indexedArgumentPresent':'-seek_timestamp' in original,
        'startNumberPresent':'-start_number' in original,
        'startNumber':option(original,'-start_number','0'),
        'originalHLSTime':option(original,'-hls_time'),
        'actualHLSTime':option(arguments,'-hls_time'),
        'originalArgvSHA256':hashlib.sha256(json.dumps(original).encode()).hexdigest(),
        'actualArgvSHA256':hashlib.sha256(json.dumps(arguments).encode()).hexdigest()}
    if eligible:
        evidence.update(request=selected,inputSeek=case['inputSeek'],sourceIDRPTS=case['sourceIDRPTS'],
            delta=case['delta'],measuredDeltaRational=case['measuredDeltaRational'])
    path=Path(config['invocations'])
    if path.exists() and path.stat().st_size>65536:
        raise SystemExit('diagnostic_invocation_bound')
    with path.open('a') as stream:
        stream.write(json.dumps(evidence)+'\n')
os.execv(config['ffmpeg'], [config['ffmpeg'], *arguments])
