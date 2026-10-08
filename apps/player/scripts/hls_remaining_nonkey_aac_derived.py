"""Derive generated AAC edits from complete native ordinal/packet clocks."""
from fractions import Fraction


def derive_audio_edit(clock, first_packet_pts, offset, current_edit):
    requested = Fraction(str(offset)) * 48000
    if requested.denominator != 1 or requested < 0 or type(current_edit) is not int:
        raise RuntimeError('aac_derived_requested_clock')
    rows = clock['completeRows']
    if not 0 < len(rows) <= 4096:
        raise RuntimeError('aac_derived_native_row_bound')
    first = [r for r in rows if r['pts'] == first_packet_pts]
    if len(first) != 1:
        raise RuntimeError('aac_derived_first_packet_identity')
    def timestamp(row):
        return Fraction(row['timestampSampleNumerator'], row['timestampSampleDenominator'])
    targets = [r for r in rows if timestamp(r) <= requested < timestamp(r) + r['samples']]
    if len(targets) != 1:
        raise RuntimeError('aac_derived_target_clock_ambiguous')
    target = targets[0]
    wanted = target['nativeStartSample'] + requested - timestamp(target)
    if wanted.denominator != 1:
        raise RuntimeError('aac_derived_fractional_native_sample')
    media_time = int(wanted) - first[0]['nativeStartSample']
    delta = media_time - current_edit
    if media_time < 32 or not -32 <= delta <= 32:
        raise RuntimeError('aac_derived_generated_edit_bound')
    return {'requestedTimestampSamples': int(requested), 'desiredNativeStartSample': int(wanted),
        'firstCopiedNativeFrame': first[0], 'targetNativeFrame': target,
        'originalMediaTime': current_edit, 'derivedMediaTime': media_time, 'deltaSamples': delta,
        'usedReferencePCMToChooseEdit': False, 'productionAcceptance': False,
        'boundary': 'Fixed 48k generated-fixture clock model; no general source-origin/codec certification'}
