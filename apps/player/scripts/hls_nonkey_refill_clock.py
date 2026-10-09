"""Exact fixed48k packet origins; diagnostic only, no producer/cache eligibility."""
import re

def integer(value):
    if type(value) is not int or not -(1 << 52) <= value <= 1 << 52:
        raise RuntimeError('refill_clock_integer')
    return value

def rescale_us(value, scale):
    integer(value)
    if type(scale) is not int or not 0 < scale <= 1000000:
        raise RuntimeError('refill_clock_scale')
    amount = abs(value) * scale
    result = (amount + 500000) // 1000000
    return -result if value < 0 else result

def audio_rows(rows):
    if type(rows) is not list or not 0 < len(rows) <= 4096:
        raise RuntimeError('refill_clock_packet_bound')
    selected = [v for v in rows if type(v) is dict and v.get('stream_index') == 1]
    if not selected:
        raise RuntimeError('refill_clock_audio_missing')
    for value in selected:
        integer(value.get('pts'))
        if re.fullmatch(r'SHA256:[a-f0-9]{64}', value.get('data_hash', '')) is None:
            raise RuntimeError('refill_clock_payload_hash')
    return selected

def audio_origin(source_rows, public_rows, raw_first, scale):
    if type(scale) is not int or scale != 48000 or integer(raw_first) < 0:
        raise RuntimeError('refill_clock_fixed_format')
    source, public = audio_rows(source_rows), audio_rows(public_rows)
    matches = [v for v in source if v['data_hash'] == public[0]['data_hash']]
    if len(matches) != 1:
        raise RuntimeError('refill_clock_first_identity')
    return integer(raw_first - matches[0]['pts'])

def refill_shift(origin, seek_us, mux_us, scale):
    return integer(integer(origin) + rescale_us(seek_us, scale) - rescale_us(mux_us, scale))
