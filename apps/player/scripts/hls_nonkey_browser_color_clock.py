"""Version-pinned Chromium ELST projection of complete retained raw video samples."""
from fractions import Fraction
import math

def mse_clock_facts(observed, request):
    def check(value):
        if not value: raise RuntimeError('browser_raw_mse_clock')
    check(type(request) in (int,float) and request==12.5)
    tracks=observed['initialization']['tracks']
    videos=[v for v in tracks if v['handler']=='vide']
    check(len(videos)==1 and len({v['trackID'] for v in tracks})==len(tracks))
    video=videos[0];scale=video['mediaTimescale']
    check(type(scale) is int and 0<scale<=10**9)
    edits=video['edits']
    check(len(edits)==1 and all(type(v['mediaTime']) is int and v['mediaTime']>=0
        and v['rateInteger']==1 and v['rateFraction']==0 for v in edits))
    media=edits[0]['mediaTime']
    check(media is not None)
    samples=[]
    for fragment in observed['physicalFragments']:
        rows=[v for v in fragment['tracks'] if v['trackID']==video['trackID']]
        check(len(rows)==1)
        samples.extend(rows[0]['samples'])
    packets=[v for v in observed['publicPacketRows'] if v['stream_index']==0]
    check(0<len(samples)==len(packets)<=4096)
    projections=[]
    for sample,packet in zip(samples,packets):
        check(all(type(sample[k]) is int and abs(sample[k])<2**53 for k in ['pts','dts','duration'])
            and sample['dts']>=0 and sample['duration']>0)
        mapped={'pts':Fraction(sample['pts']-media,scale), 'dts':Fraction(sample['dts']-media,scale)}
        mapped['duration']=Fraction(sample['duration'],scale)
        for key,value in mapped.items():
            actual=float(packet[key+'_time'])
            check(math.isfinite(actual) and abs(actual-float(value))<=0.0000011)
        projections.append(mapped)
    first=projections[0]['pts']
    check(first==Fraction(-1,2) and first+Fraction(str(request))==12)
    return {'qualified':True,'trackID':video['trackID'],'mediaTimescale':scale,'editMediaTime':media,
        'firstChromiumRawDTSSeconds':float(Fraction(samples[0]['dts'],scale)),
        'firstFFprobeLogicalDTSSeconds':float(projections[0]['dts']),
        'sampleCount':len(samples),'firstClipPTSSeconds':float(first),'firstSourcePTSSeconds':float(first+Fraction(str(request))),
        'timestampOffset':request,'allRawSamplesRetained':True,'everyFFprobeLogicalVideoPacketClockMatched':True,'chromiumDTSRemainsUnedited':True,
        'interpretation':'Chromium153 first single nonnegative ELST subtracts CTS only; FFprobe logical DTS reported separately; component-only'}
