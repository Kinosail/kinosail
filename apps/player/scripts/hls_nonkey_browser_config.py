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
