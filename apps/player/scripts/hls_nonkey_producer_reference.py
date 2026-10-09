#!/usr/bin/env python3
"""Fresh qualified CLI packet oracle for the private canonical-producer regression."""
from fractions import Fraction
import json
from pathlib import Path
import re
import subprocess
import sys
from hls_followon_public import bounded_bytes, check
from hls_timeline_http import source_state

receipt_path=Path(sys.argv[1])
target=Path(sys.argv[2])
receipt=json.loads(bounded_bytes(receipt_path,32<<20,'producer_reference_receipt_bound'))
revision=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
tree=subprocess.check_output(['git','rev-parse','HEAD^{tree}'],text=True).strip()
check(receipt['revision']==revision and receipt['tree']==tree and receipt['result']=='observed'
      and receipt['priorProducerGateRemainsEmpty'] and not receipt['productionAcceptance'],
      'producer_reference_exact_source_binding')
result={'revision':revision,'tree':tree,'qualifiedIndependentReference':True,'cases':[]}
for container in ['mkv','mp4']:
    sources=[v for v in receipt['sources'] if v['container']==container]
    cases=[v for v in receipt['cases'] if v['container']==container and
           v['requestedRelativeSeconds']==12.5 and v['label']=='shift-no-prior']
    check(len(sources)==len(cases)==1,'producer_reference_unique_case')
    source,row=sources[0],cases[0]
    path=Path(source['path'])
    check(source_state(path)==source['state']==source['finalSourceState'],
          'producer_reference_immutable_source')
    observed=row['observations']
    check(row['consumerCapabilityQualified'] and row['firstIDRMatchesRequired'] and
          row['videoCompletePayloadTail'] and observed['audioPayloadTail']['wholePublicPacketTail'] and
          row['requiredPrecedingIDR']['containsIDR'],'producer_reference_complete_gate')
    requested=Fraction(row['requestedSourcePTS'])
    micros=requested*1000000
    check(micros.denominator==1 and 0<micros<20000000,'producer_reference_physical_requested_clock')
    expected=[p for p in observed['publicPacketRows'] if p['stream_index'] in [0,1]]
    check(0<len(expected)<=4096 and all(re.fullmatch(r'SHA256:[a-f0-9]{64}',p['data_hash']) for p in expected),
          'producer_reference_complete_packet_identity')
    result['cases'].append({'container':container,'source':str(path),
        'sourceSHA256':source['state']['sha256'],'requestedMicros':int(micros),
        'requiredIDRPTS':row['requiredPrecedingIDR']['pts_time'],
        'requiredIDRPayloadSHA256':row['requiredPrecedingIDR']['payloadSHA256'],
        'expectedVideo':[p for p in expected if p['stream_index']==0],
        'expectedAudio':[p for p in expected if p['stream_index']==1]})
raw=json.dumps(result,separators=(',',':'),allow_nan=False)+'\n'
check(0<len(raw.encode())<=4<<20,'producer_reference_output_bound')
target.write_text(raw)
target.chmod(0o600)
print(json.dumps({'revision':revision,'tree':tree,'qualifiedIndependentReference':True,
    'cases':[{'container':v['container'],'requestedMicros':v['requestedMicros'],
      'videoPackets':len(v['expectedVideo']),'audioPackets':len(v['expectedAudio'])} for v in result['cases']],
    'productionAcceptance':False}))
