"""Complete payload ordinal association for the fixed public AAC failure.
No source clock, cache certificate or decoder acceptance is granted."""
import hashlib
import json
import re

def packet_association(tail):
    unavailable = {'observed': False, 'failureClass': 'packet_association_shape',
                   'acceptance': False}
    if not isinstance(tail, dict):
        return unavailable
    source = tail.get('completeSourceRows')
    public = tail.get('completePublicRows')
    if not all(isinstance(rows, list) and 0 < len(rows) <= 4096
               for rows in [source, public]):
        return unavailable
    if not all(isinstance(row, dict) and isinstance(row.get('data_hash'), str)
               and re.fullmatch(r'SHA256:[a-f0-9]{64}', row['data_hash'])
               for rows in [source, public] for row in rows):
        return unavailable
    source_hashes = [row['data_hash'] for row in source]
    public_hashes = [row['data_hash'] for row in public]
    positions = {}
    for ordinal, digest in enumerate(source_hashes):
        positions.setdefault(digest, []).append(ordinal)
    ordinals = []
    ambiguous = unknown = 0
    for digest in public_hashes:
        matches = positions.get(digest, [])
        unknown += len(matches) == 0
        ambiguous += len(matches) > 1
        ordinals.append(matches[0] if len(matches) == 1 else None)
    transitions = []
    for ordinal in range(1, len(ordinals)):
        previous, current = ordinals[ordinal-1], ordinals[ordinal]
        if previous is not None and current is not None and current-previous != 1:
            transitions.append({'publicOrdinal': ordinal, 'previousSourceOrdinal': previous,
                                'sourceOrdinal': current, 'delta': current-previous})
    def binding(values):
        return hashlib.sha256(json.dumps(values, separators=(',', ':')).encode()).hexdigest()
    return {'observed': True, 'sourcePackets': len(source), 'publicPackets': len(public),
            'sourcePayloadSequenceSHA256': binding(source_hashes),
            'publicPayloadSequenceSHA256': binding(public_hashes),
            'sourceOrdinals': ordinals, 'ordinalRowsTrimmed': 0,
            'ambiguousPackets': ambiguous, 'unknownPackets': unknown,
            'transitionCount': len(transitions), 'transitions': transitions[:64],
            'transitionOverflow': len(transitions) > 64,
            'associationScope': 'Payload association only; no source-clock or decoder discard rule',
            'acceptance': False}
