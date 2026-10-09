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

def matrix_failure(error):
    if isinstance(error, RuntimeError):
        value = str(error)
        if value in ('bounded_diagnostic_deadline', 'bounded_run_deadline'):
            raise error
        if re.fullmatch(r'[a-z][a-z0-9_]{0,79}', value):
            return value
    return type(error).__name__

def fixed_streams(facts):
    rows = facts.get('streams') if type(facts) is dict else None
    if type(rows) is not list or len(rows) != 2:
        raise RuntimeError('refill_clock_stream_slots')
    for number, value in enumerate(rows):
        if type(value) is not dict or type(value.get('index')) is not int or value['index'] != number:
            raise RuntimeError('refill_clock_stream_slots')
    video, audio = rows
    if video.get('codec_type') != 'video' or video.get('codec_name') != 'h264' or video.get('time_base') != '1/16000':
        raise RuntimeError('refill_clock_video_grid')
    if audio.get('codec_type') != 'audio' or audio.get('codec_name') != 'aac' or audio.get('time_base') != '1/48000':
        raise RuntimeError('refill_clock_audio_grid')
    if audio.get('sample_rate') != '48000' or type(audio.get('channels')) is not int or audio['channels'] != 2:
        raise RuntimeError('refill_clock_audio_format')
