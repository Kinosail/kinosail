"""Bounded rational clocks from the independent fresh CLI receipt."""
from fractions import Fraction
import math
import re

def measured_delta(value):
    if type(value) is not str or len(value)>64 or re.fullmatch(r'[0-9]{1,20}(?:/[1-9][0-9]{0,19})?',value) is None:
        raise RuntimeError('browser_rational_clock_shape')
    rational=Fraction(value)
    if not 0<=rational<15:
        raise RuntimeError('browser_rational_clock_bound')
    seconds=float(rational)
    if not math.isfinite(seconds):
        raise RuntimeError('browser_rational_clock_finite')
    return {'rational':value,'seconds':seconds}

def ordinary_cold_arguments(arguments, source):
    """Only the fixed two-second initial recipe may receive the diagnostic shift."""
    if '-seek_timestamp' in arguments:
        return False
    def value(option, default=None):
        count=arguments.count(option)
        if count==0:
            return default
        position=arguments.index(option)
        return arguments[position+1] if count==1 and position+1<len(arguments) else None
    return (value('-hls_time')=='2' and value('-start_number','0')=='0'
            and value('-i')==source and value('-ss') is not None)
