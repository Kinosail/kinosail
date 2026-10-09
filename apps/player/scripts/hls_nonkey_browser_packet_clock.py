"""Complete fixed-fixture AAC clock correspondence; no decoder/cache acceptance."""
from hls_nonkey_browser_packet_association import packet_association

def packet_clock(tail, boundary):
    unavailable={'observed':False,'failureClass':'packet_clock_shape','acceptance':False}
    association=packet_association(tail)
    if not association['observed'] or not isinstance(boundary,dict):
        return unavailable
    if not all(isinstance(boundary.get(name),dict)
               and boundary[name].get('timeBase')=='1/48000'
               and boundary[name].get('sampleRate')==48000
               for name in ['sourceStream','publicStream']):
        return unavailable
    source,public=tail['completeSourceRows'],tail['completePublicRows']
    for rows in [source,public]:
        for row in rows:
            if not all(type(row.get(field)) is int and abs(row[field])<=1<<52
                       for field in ['pts','dts','duration']) or row['duration']<=0:
                return unavailable
    known=[]
    for ordinal,matched in enumerate(association['sourceOrdinals']):
        if matched is not None:
            known.append((ordinal,matched,public[ordinal]['pts']-source[matched]['pts']))
    changes=[]
    for previous,current in zip(known,known[1:]):
        if previous[2]!=current[2]:
            changes.append({'publicOrdinal':current[0],'sourceOrdinal':current[1],
                            'previousOffsetTicks':previous[2],'offsetTicks':current[2]})
    adjacent=[]
    for ordinal in range(1,len(public)):
        delta=public[ordinal]['pts']-public[ordinal-1]['pts']-public[ordinal-1]['duration']
        if delta:
            adjacent.append({'publicOrdinal':ordinal,'deltaTicks':delta})
    offsets=[row[2] for row in known]
    return {'observed':True,'timeBase':'1/48000','knownClockPackets':len(known),
            'ambiguousPackets':association['ambiguousPackets'],
            'unknownPackets':association['unknownPackets'],
            'clockOffsetRangeTicks':[min(offsets),max(offsets)] if offsets else None,
            'clockChangeCount':len(changes),'clockChanges':changes[:64],
            'clockChangeOverflow':len(changes)>64,
            'adjacentPTSChangeCount':len(adjacent),'adjacentPTSChanges':adjacent[:64],
            'adjacentPTSChangeOverflow':len(adjacent)>64,
            'sourcePayloadSequenceSHA256':association['sourcePayloadSequenceSHA256'],
            'publicPayloadSequenceSHA256':association['publicPayloadSequenceSHA256'],
            'clockScope':'Edited demux PTS ticks; ambiguous payloads grant no correspondence',
            'acceptance':False}
