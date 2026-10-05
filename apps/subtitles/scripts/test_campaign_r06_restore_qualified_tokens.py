"""Failure-first compatibility for the admitted unadmitted Main source pair.

The diagnostic remains unadmitted formatter evidence, not compiler/runtime proof.
These controls use fixed source bytes only; no IO, process or network is needed.
"""
import base64
import hashlib
import unittest

from campaign_r06_restore_tokens import brace_kind, equivalent

RAW_MAIN = base64.b64decode(
    b"cGFja2FnZSBtYWluCgppbXBvcnQgKAoJImNvbnRleHQiCgkiZW5jb2RpbmcvanNvbiIKCSJlcnJvcnMiCgkiZmxhZyIKCSJuZXQvaHR0cCIKCSJvcyIKCSJv"
    b"cy9zaWduYWwiCgkicGF0aC9maWxlcGF0aCIKCSJzeW5jIgoJInN5c2NhbGwiCgkidGVzdGluZyIKCSJ0aW1lIgoKCSJnaXRodWIuY29tL01pa2VPNy9raW5v"
    b"c2FpbC1zdWJ0aXRsZXMvaW50ZXJuYWwvc2VydmVyIgopCgp2YXIgKAoJcmVzdG9yZVNlcnZlID0gZmxhZy5Cb29sKCJyMDYtcmVzdG9yZS1zZXJ2ZSIsIGZh"
    b"bHNlLCAic2VydmUgdGhlIGRpc3Bvc2FibGUgUmVzdG9yZSBmaXh0dXJlIikKCXJlc3RvcmVSb290ID0gZmxhZy5TdHJpbmcoInIwNi1yZXN0b3JlLXJvb3Qi"
    b"LCAiIiwgImVtcHR5IHByaXZhdGUgZGlzcG9zYWJsZSBkaXJlY3RvcnkiKQoJcmVzdG9yZU1vZGUgPSBmbGFnLlN0cmluZygicjA2LXJlc3RvcmUtZmF1bHQi"
    b"LCAiIiwgImhlYWRlcnMgb3IgaW5zcGVjdC1ib2R5IikKKQoKdHlwZSByZXN0b3JlU25hcHNob3Qgc3RydWN0IHsKCVByb3RvY29sIHN0cmluZyBganNvbjoi"
    b"cHJvdG9jb2wiYAoJU2V0dXBTYXZlQXR0ZW1wdHMgaW50IGBqc29uOiJzZXR1cFNhdmVBdHRlbXB0cyJgCglSZXN0b3JlQXR0ZW1wdHMgaW50IGBqc29uOiJy"
    b"ZXN0b3JlQXR0ZW1wdHMiYAoJUHJlcGFyZUF0dGVtcHRzIGludCBganNvbjoicHJlcGFyZUF0dGVtcHRzImAKCVJlc3BvbnNlU3RhdHVzIGludCBganNvbjoi"
    b"cmVzcG9uc2VTdGF0dXMiYAoJSW5zcGVjdGlvblJlc3BvbnNlU3RhdHVzIGludCBganNvbjoiaW5zcGVjdGlvblJlc3BvbnNlU3RhdHVzImAKCUFjdGl2ZUhv"
    b"bGRzIGludCBganNvbjoiYWN0aXZlSG9sZHMiYAoJQnJvd3NlckNvbXBsZXRlZFJlY2VpcHRSZWFkcyBpbnQgYGpzb246ImJyb3dzZXJDb21wbGV0ZWRSZWNl"
    b"aXB0UmVhZHMiYAoJQnJvd3NlclJlc3RvcmVkSW5zcGVjdGlvbnMgaW50IGBqc29uOiJicm93c2VyUmVzdG9yZWRJbnNwZWN0aW9ucyJgCglBY3R1YWxSZXN0"
    b"b3JlZCBib29sIGBqc29uOiJhY3R1YWxSZXN0b3JlZCJgCglIaXN0b3J5T25jZSBib29sIGBqc29uOiJoaXN0b3J5T25jZSJgCglSZWNvdmVyeVN3YXBwZWQg"
    b"Ym9vbCBganNvbjoicmVjb3ZlcnlTd2FwcGVkImAKCUluc3BlY3Rpb25NYXRjaGVzIGJvb2wgYGpzb246Imluc3BlY3Rpb25NYXRjaGVzImAKCVJlY2VpcHRD"
    b"b21wbGV0ZWQgYm9vbCBganNvbjoicmVjZWlwdENvbXBsZXRlZCJgCglSZWNlaXB0U3VjY2VlZGVkIGJvb2wgYGpzb246InJlY2VpcHRTdWNjZWVkZWQiYAoJ"
    b"SG9sZEVsaWdpYmxlIGJvb2wgYGpzb246ImhvbGRFbGlnaWJsZSJgCglIZWFkZXJzUmVsZWFzZWQgYm9vbCBganNvbjoiaGVhZGVyc1JlbGVhc2VkImAKCUJv"
    b"ZHlSZWxlYXNlZCBib29sIGBqc29uOiJib2R5UmVsZWFzZWQiYAoJUmVzdG9yZVJlc3BvbnNlRGVsaXZlcmVkIGJvb2wgYGpzb246InJlc3RvcmVSZXNwb25z"
    b"ZURlbGl2ZXJlZCJgCglJbnNwZWN0aW9uUmVzcG9uc2VEZWxpdmVyZWQgYm9vbCBganNvbjoiaW5zcGVjdGlvblJlc3BvbnNlRGVsaXZlcmVkImAKCVJlc3Rv"
    b"cmVDbGllbnRDYW5jZWxsZWQgYm9vbCBganNvbjoicmVzdG9yZUNsaWVudENhbmNlbGxlZCJgCglJbnNwZWN0aW9uQ2xpZW50Q2FuY2VsbGVkIGJvb2wgYGpz"
    b"b246Imluc3BlY3Rpb25DbGllbnRDYW5jZWxsZWQiYAoJSG9sZEV4cGlyZWQgYm9vbCBganNvbjoiaG9sZEV4cGlyZWQiYAoJQm91bmRhcnlGYWlsZWQgYm9v"
    b"bCBganNvbjoiYm91bmRhcnlGYWlsZWQiYAp9Cgp0eXBlIHJlc3RvcmVSaWcgc3RydWN0IHsKCXRhcmdldCAqcmVzdG9yZVRhcmdldAoJYXBwIGh0dHAuSGFu"
    b"ZGxlcgoJZmlsZXMgKm9zLlJvb3QKCWN0eCBjb250ZXh0LkNvbnRleHQKCWNhbmNlbCBjb250ZXh0LkNhbmNlbEZ1bmMKCW1vZGUsIGl0ZW0gc3RyaW5nCglj"
    b"dXJyZW50QmVmb3JlIFtdYnl0ZQoJbXUgc3luYy5NdXRleAoJb3duZWQgc3luYy5XYWl0R3JvdXAKCXN0YXRlIHJlc3RvcmVTbmFwc2hvdAoJcmVzdG9yZUhl"
    b"YWRlcnMsIGluc3BlY3Rpb25IZWFkZXJzIGh0dHAuSGVhZGVyCglyZXN0b3JlQm9keSwgaW5zcGVjdGlvbkJvZHkgW11ieXRlCgllbGlnaWJsZSwgcmVsZWFz"
    b"ZWQgY2hhbiBzdHJ1Y3R7fQoJcmVsZWFzZU9uY2UsIGVsaWdpYmxlT25jZSwgc3RvcE9uY2Ugc3luYy5PbmNlCglpbnNwZWN0aW9uSGVsZCwgY2xlYW51cEZh"
    b"aWxlZCBib29sCn0KCmZ1bmMgVGVzdE1haW4obSAqdGVzdGluZy5NKSB7CglmbGFnLlBhcnNlKCkKCWlmICpyZXN0b3JlU2VydmUgewoJCW9zLkV4aXQocnVu"
    b"UmVzdG9yZUZpeHR1cmUoKSkKCX0KCW9zLkV4aXQobS5SdW4oKSkKfQoKZnVuYyBuZXdSZXN0b3JlUmlnKHRhcmdldCAqcmVzdG9yZVRhcmdldCwgZGlyZWN0"
    b"b3J5LCBtb2RlIHN0cmluZykgKCpyZXN0b3JlUmlnLCBlcnJvcikgewoJaWYgZGlyZWN0b3J5ID09ICIiIHx8ICFmaWxlcGF0aC5Jc0FicyhkaXJlY3Rvcnkp"
    b"IHsKCQlyZXR1cm4gbmlsLCBlcnJvcnMuTmV3KCJSZXN0b3JlIGZpeHR1cmUgcm9vdCB1bmF2YWlsYWJsZSIpCgl9CglpZiBtb2RlICE9ICJub25lIiAmJiBt"
    b"b2RlICE9ICJoZWFkZXJzIiAmJiBtb2RlICE9ICJpbnNwZWN0LWJvZHkiIHsKCQlyZXR1cm4gbmlsLCBlcnJvcnMuTmV3KCJSZXN0b3JlIGZpeHR1cmUgZmF1"
    b"bHQgdW5hdmFpbGFibGUiKQoJfQoJZmlsZXMsIGVyciA6PSBwcmVwYXJlUmVzdG9yZUZpbGVzKGRpcmVjdG9yeSkKCWlmIGVyciAhPSBuaWwgeyByZXR1cm4g"
    b"bmlsLCBlcnIgfQoJY3R4LCBjYW5jZWwgOj0gY29udGV4dC5XaXRoQ2FuY2VsKGNvbnRleHQuQmFja2dyb3VuZCgpKQoJZiA6PSAmcmVzdG9yZVJpZ3t0YXJn"
    b"ZXQ6IHRhcmdldCwgZmlsZXM6IGZpbGVzLCBjdHg6IGN0eCwgY2FuY2VsOiBjYW5jZWwsIG1vZGU6IG1vZGUsCgkJZWxpZ2libGU6IG1ha2UoY2hhbiBzdHJ1"
    b"Y3R7fSksIHJlbGVhc2VkOiBtYWtlKGNoYW4gc3RydWN0e30pLCBzdGF0ZTogcmVzdG9yZVNuYXBzaG90e1Byb3RvY29sOiAidW5yZWFjaGVkIn19Cgl0b29s"
    b"IDo9IGZpbGVwYXRoLkpvaW4oZGlyZWN0b3J5LCAidW5hdmFpbGFibGUtbWVkaWEtdG9vbCIpCglmLmFwcCA9IHNlcnZlci5OZXcoc2VydmVyLkNvbmZpZ3tM"
    b"aWZlY3ljbGU6IGN0eCwgU3VidGl0bGVBcHA6IHRydWUsIFJlcXVpcmVBdXRoOiB0cnVlLCBBdXRoVVJMOiB0YXJnZXQub3JpZ2luLAoJCU1lZGlhRGlyOiBm"
    b"aWxlcGF0aC5Kb2luKGRpcmVjdG9yeSwgIm1lZGlhIiksIERhdGFEaXI6IGZpbGVwYXRoLkpvaW4oZGlyZWN0b3J5LCAiZGF0YSIpLAoJCUNhY2hlRGlyOiBm"
    b"aWxlcGF0aC5Kb2luKGRpcmVjdG9yeSwgImNhY2hlIiksIEZGbXBlZzogdG9vbCwgRkZwcm9iZTogdG9vbCwgRlBDYWxjOiB0b29sfSkKCXRhcmdldC50bHMu"
    b"Q29uZmlnLkhhbmRsZXIgPSBodHRwLkhhbmRsZXJGdW5jKGZ1bmModyBodHRwLlJlc3BvbnNlV3JpdGVyLCByICpodHRwLlJlcXVlc3QpIHsgZi5zZXJ2ZSh0"
    b"YXJnZXQsIHcsIHIpIH0pCgl0YXJnZXQudGxzLlN0YXJ0VExTKCkKCXJldHVybiBmLCBuaWwKfQoKZnVuYyAoZiAqcmVzdG9yZVJpZykgZmFpbEJvdW5kYXJ5"
    b"KCkgeyBmLm11LkxvY2soKTsgZi5zdGF0ZS5Cb3VuZGFyeUZhaWxlZCA9IHRydWU7IGYubXUuVW5sb2NrKCkgfQpmdW5jIChmICpyZXN0b3JlUmlnKSBzbmFw"
    b"c2hvdCgpIHJlc3RvcmVTbmFwc2hvdCB7IGYubXUuTG9jaygpOyBkZWZlciBmLm11LlVubG9jaygpOyByZXR1cm4gZi5zdGF0ZSB9CmZ1bmMgKGYgKnJlc3Rv"
    b"cmVSaWcpIHJlbGVhc2UoKSB7IGYucmVsZWFzZU9uY2UuRG8oZnVuYygpIHsgY2xvc2UoZi5yZWxlYXNlZCkgfSkgfQpmdW5jIChmICpyZXN0b3JlUmlnKSBz"
    b"dG9wKCkgYm9vbCB7CglmLnN0b3BPbmNlLkRvKGZ1bmMoKSB7CgkJZi5jYW5jZWwoKTsgZi5yZWxlYXNlKCkKCQluZXR3b3JrT0sgOj0gZi50YXJnZXQuc3Rv"
    b"cCgpCgkJZi5vd25lZC5XYWl0KCkKCQlpZiBlcnIgOj0gZi5maWxlcy5DbG9zZSgpOyBlcnIgIT0gbmlsIHx8ICFuZXR3b3JrT0sgewoJCQlmLm11LkxvY2so"
    b"KTsgZi5zdGF0ZS5Cb3VuZGFyeUZhaWxlZCwgZi5jbGVhbnVwRmFpbGVkID0gdHJ1ZSwgdHJ1ZTsgZi5tdS5VbmxvY2soKQoJCX0KCX0pCglmLm11LkxvY2so"
    b"KTsgZGVmZXIgZi5tdS5VbmxvY2soKQoJcmV0dXJuICFmLmNsZWFudXBGYWlsZWQKfQoKZnVuYyBydW5SZXN0b3JlRml4dHVyZSgpIChleGl0IGludCkgewoJ"
    b"dGFyZ2V0LCBlcnIgOj0gbmV3T3duZWRSZXN0b3JlVGFyZ2V0KCkKCWlmIGVyciAhPSBuaWwgeyByZXR1cm4gMiB9CglkZWZlciBmdW5jKCkgeyBpZiAhdGFy"
    b"Z2V0LnN0b3AoKSB7IGV4aXQgPSAyIH0gfSgpCglmLCBlcnIgOj0gbmV3UmVzdG9yZVJpZyh0YXJnZXQsICpyZXN0b3JlUm9vdCwgKnJlc3RvcmVNb2RlKQoJ"
    b"aWYgZXJyICE9IG5pbCB7IHJldHVybiAyIH0KCWRlZmVyIGZ1bmMoKSB7IGlmICFmLnN0b3AoKSB7IGV4aXQgPSAyIH0gfSgpCglpZiBqc29uLk5ld0VuY29k"
    b"ZXIob3MuU3Rkb3V0KS5FbmNvZGUoc3RydWN0IHsgS2luZCwgT3JpZ2luIHN0cmluZyB9eyJyMDYtcmVzdG9yZS1maXh0dXJlLXYxIiwgdGFyZ2V0Lm9yaWdp"
    b"bn0pICE9IG5pbCB7IHJldHVybiAyIH0KCXN0b3BwaW5nIDo9IG1ha2UoY2hhbiBvcy5TaWduYWwsIDEpCglzaWduYWwuTm90aWZ5KHN0b3BwaW5nLCBzeXNj"
    b"YWxsLlNJR1RFUk0sIHN5c2NhbGwuU0lHSU5UKQoJZGVmZXIgc2lnbmFsLlN0b3Aoc3RvcHBpbmcpCglzZWxlY3QgeyBjYXNlIDwtc3RvcHBpbmc6IGNhc2Ug"
    b"PC10aW1lLkFmdGVyKDEyMCp0aW1lLlNlY29uZCk6IH0KCXJldHVybiAwCn0K"
)

DIAGNOSTIC_MAIN = base64.b64decode(
    b"cGFja2FnZSBtYWluCgppbXBvcnQgKAoJImNvbnRleHQiCgkiZW5jb2RpbmcvanNvbiIKCSJlcnJvcnMiCgkiZmxhZyIKCSJuZXQvaHR0cCIKCSJvcyIKCSJv"
    b"cy9zaWduYWwiCgkicGF0aC9maWxlcGF0aCIKCSJzeW5jIgoJInN5c2NhbGwiCgkidGVzdGluZyIKCSJ0aW1lIgoKCSJnaXRodWIuY29tL01pa2VPNy9raW5v"
    b"c2FpbC1zdWJ0aXRsZXMvaW50ZXJuYWwvc2VydmVyIgopCgp2YXIgKAoJcmVzdG9yZVNlcnZlID0gZmxhZy5Cb29sKCJyMDYtcmVzdG9yZS1zZXJ2ZSIsIGZh"
    b"bHNlLCAic2VydmUgdGhlIGRpc3Bvc2FibGUgUmVzdG9yZSBmaXh0dXJlIikKCXJlc3RvcmVSb290ICA9IGZsYWcuU3RyaW5nKCJyMDYtcmVzdG9yZS1yb290"
    b"IiwgIiIsICJlbXB0eSBwcml2YXRlIGRpc3Bvc2FibGUgZGlyZWN0b3J5IikKCXJlc3RvcmVNb2RlICA9IGZsYWcuU3RyaW5nKCJyMDYtcmVzdG9yZS1mYXVs"
    b"dCIsICIiLCAiaGVhZGVycyBvciBpbnNwZWN0LWJvZHkiKQopCgp0eXBlIHJlc3RvcmVTbmFwc2hvdCBzdHJ1Y3QgewoJUHJvdG9jb2wgICAgICAgICAgICAg"
    b"ICAgICAgICBzdHJpbmcgYGpzb246InByb3RvY29sImAKCVNldHVwU2F2ZUF0dGVtcHRzICAgICAgICAgICAgaW50ICAgIGBqc29uOiJzZXR1cFNhdmVBdHRl"
    b"bXB0cyJgCglSZXN0b3JlQXR0ZW1wdHMgICAgICAgICAgICAgIGludCAgICBganNvbjoicmVzdG9yZUF0dGVtcHRzImAKCVByZXBhcmVBdHRlbXB0cyAgICAg"
    b"ICAgICAgICAgaW50ICAgIGBqc29uOiJwcmVwYXJlQXR0ZW1wdHMiYAoJUmVzcG9uc2VTdGF0dXMgICAgICAgICAgICAgICBpbnQgICAgYGpzb246InJlc3Bv"
    b"bnNlU3RhdHVzImAKCUluc3BlY3Rpb25SZXNwb25zZVN0YXR1cyAgICAgaW50ICAgIGBqc29uOiJpbnNwZWN0aW9uUmVzcG9uc2VTdGF0dXMiYAoJQWN0aXZl"
    b"SG9sZHMgICAgICAgICAgICAgICAgICBpbnQgICAgYGpzb246ImFjdGl2ZUhvbGRzImAKCUJyb3dzZXJDb21wbGV0ZWRSZWNlaXB0UmVhZHMgaW50ICAgIGBq"
    b"c29uOiJicm93c2VyQ29tcGxldGVkUmVjZWlwdFJlYWRzImAKCUJyb3dzZXJSZXN0b3JlZEluc3BlY3Rpb25zICAgaW50ICAgIGBqc29uOiJicm93c2VyUmVz"
    b"dG9yZWRJbnNwZWN0aW9ucyJgCglBY3R1YWxSZXN0b3JlZCAgICAgICAgICAgICAgIGJvb2wgICBganNvbjoiYWN0dWFsUmVzdG9yZWQiYAoJSGlzdG9yeU9u"
    b"Y2UgICAgICAgICAgICAgICAgICBib29sICAgYGpzb246Imhpc3RvcnlPbmNlImAKCVJlY292ZXJ5U3dhcHBlZCAgICAgICAgICAgICAgYm9vbCAgIGBqc29u"
    b"OiJyZWNvdmVyeVN3YXBwZWQiYAoJSW5zcGVjdGlvbk1hdGNoZXMgICAgICAgICAgICBib29sICAgYGpzb246Imluc3BlY3Rpb25NYXRjaGVzImAKCVJlY2Vp"
    b"cHRDb21wbGV0ZWQgICAgICAgICAgICAgYm9vbCAgIGBqc29uOiJyZWNlaXB0Q29tcGxldGVkImAKCVJlY2VpcHRTdWNjZWVkZWQgICAgICAgICAgICAgYm9v"
    b"bCAgIGBqc29uOiJyZWNlaXB0U3VjY2VlZGVkImAKCUhvbGRFbGlnaWJsZSAgICAgICAgICAgICAgICAgYm9vbCAgIGBqc29uOiJob2xkRWxpZ2libGUiYAoJ"
    b"SGVhZGVyc1JlbGVhc2VkICAgICAgICAgICAgICBib29sICAgYGpzb246ImhlYWRlcnNSZWxlYXNlZCJgCglCb2R5UmVsZWFzZWQgICAgICAgICAgICAgICAg"
    b"IGJvb2wgICBganNvbjoiYm9keVJlbGVhc2VkImAKCVJlc3RvcmVSZXNwb25zZURlbGl2ZXJlZCAgICAgYm9vbCAgIGBqc29uOiJyZXN0b3JlUmVzcG9uc2VE"
    b"ZWxpdmVyZWQiYAoJSW5zcGVjdGlvblJlc3BvbnNlRGVsaXZlcmVkICBib29sICAgYGpzb246Imluc3BlY3Rpb25SZXNwb25zZURlbGl2ZXJlZCJgCglSZXN0"
    b"b3JlQ2xpZW50Q2FuY2VsbGVkICAgICAgIGJvb2wgICBganNvbjoicmVzdG9yZUNsaWVudENhbmNlbGxlZCJgCglJbnNwZWN0aW9uQ2xpZW50Q2FuY2VsbGVk"
    b"ICAgIGJvb2wgICBganNvbjoiaW5zcGVjdGlvbkNsaWVudENhbmNlbGxlZCJgCglIb2xkRXhwaXJlZCAgICAgICAgICAgICAgICAgIGJvb2wgICBganNvbjoi"
    b"aG9sZEV4cGlyZWQiYAoJQm91bmRhcnlGYWlsZWQgICAgICAgICAgICAgICBib29sICAgYGpzb246ImJvdW5kYXJ5RmFpbGVkImAKfQoKdHlwZSByZXN0b3Jl"
    b"UmlnIHN0cnVjdCB7Cgl0YXJnZXQgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAqcmVzdG9yZVRhcmdldAoJYXBwICAgICAgICAgICAgICAgICAgICAg"
    b"ICAgICAgICAgICAgaHR0cC5IYW5kbGVyCglmaWxlcyAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAqb3MuUm9vdAoJY3R4ICAgICAgICAgICAgICAg"
    b"ICAgICAgICAgICAgICAgICAgY29udGV4dC5Db250ZXh0CgljYW5jZWwgICAgICAgICAgICAgICAgICAgICAgICAgICAgICBjb250ZXh0LkNhbmNlbEZ1bmMK"
    b"CW1vZGUsIGl0ZW0gICAgICAgICAgICAgICAgICAgICAgICAgIHN0cmluZwoJY3VycmVudEJlZm9yZSAgICAgICAgICAgICAgICAgICAgICAgW11ieXRlCglt"
    b"dSAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICBzeW5jLk11dGV4Cglvd25lZCAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICBzeW5jLldh"
    b"aXRHcm91cAoJc3RhdGUgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgcmVzdG9yZVNuYXBzaG90CglyZXN0b3JlSGVhZGVycywgaW5zcGVjdGlvbkhl"
    b"YWRlcnMgICBodHRwLkhlYWRlcgoJcmVzdG9yZUJvZHksIGluc3BlY3Rpb25Cb2R5ICAgICAgICAgW11ieXRlCgllbGlnaWJsZSwgcmVsZWFzZWQgICAgICAg"
    b"ICAgICAgICAgICBjaGFuIHN0cnVjdHt9CglyZWxlYXNlT25jZSwgZWxpZ2libGVPbmNlLCBzdG9wT25jZSBzeW5jLk9uY2UKCWluc3BlY3Rpb25IZWxkLCBj"
    b"bGVhbnVwRmFpbGVkICAgICAgIGJvb2wKfQoKZnVuYyBUZXN0TWFpbihtICp0ZXN0aW5nLk0pIHsKCWZsYWcuUGFyc2UoKQoJaWYgKnJlc3RvcmVTZXJ2ZSB7"
    b"CgkJb3MuRXhpdChydW5SZXN0b3JlRml4dHVyZSgpKQoJfQoJb3MuRXhpdChtLlJ1bigpKQp9CgpmdW5jIG5ld1Jlc3RvcmVSaWcodGFyZ2V0ICpyZXN0b3Jl"
    b"VGFyZ2V0LCBkaXJlY3RvcnksIG1vZGUgc3RyaW5nKSAoKnJlc3RvcmVSaWcsIGVycm9yKSB7CglpZiBkaXJlY3RvcnkgPT0gIiIgfHwgIWZpbGVwYXRoLklz"
    b"QWJzKGRpcmVjdG9yeSkgewoJCXJldHVybiBuaWwsIGVycm9ycy5OZXcoIlJlc3RvcmUgZml4dHVyZSByb290IHVuYXZhaWxhYmxlIikKCX0KCWlmIG1vZGUg"
    b"IT0gIm5vbmUiICYmIG1vZGUgIT0gImhlYWRlcnMiICYmIG1vZGUgIT0gImluc3BlY3QtYm9keSIgewoJCXJldHVybiBuaWwsIGVycm9ycy5OZXcoIlJlc3Rv"
    b"cmUgZml4dHVyZSBmYXVsdCB1bmF2YWlsYWJsZSIpCgl9CglmaWxlcywgZXJyIDo9IHByZXBhcmVSZXN0b3JlRmlsZXMoZGlyZWN0b3J5KQoJaWYgZXJyICE9"
    b"IG5pbCB7CgkJcmV0dXJuIG5pbCwgZXJyCgl9CgljdHgsIGNhbmNlbCA6PSBjb250ZXh0LldpdGhDYW5jZWwoY29udGV4dC5CYWNrZ3JvdW5kKCkpCglmIDo9"
    b"ICZyZXN0b3JlUmlnewoJCXRhcmdldDogdGFyZ2V0LCBmaWxlczogZmlsZXMsIGN0eDogY3R4LCBjYW5jZWw6IGNhbmNlbCwgbW9kZTogbW9kZSwKCQllbGln"
    b"aWJsZTogbWFrZShjaGFuIHN0cnVjdHt9KSwgcmVsZWFzZWQ6IG1ha2UoY2hhbiBzdHJ1Y3R7fSksIHN0YXRlOiByZXN0b3JlU25hcHNob3R7UHJvdG9jb2w6"
    b"ICJ1bnJlYWNoZWQifSwKCX0KCXRvb2wgOj0gZmlsZXBhdGguSm9pbihkaXJlY3RvcnksICJ1bmF2YWlsYWJsZS1tZWRpYS10b29sIikKCWYuYXBwID0gc2Vy"
    b"dmVyLk5ldyhzZXJ2ZXIuQ29uZmlnewoJCUxpZmVjeWNsZTogY3R4LCBTdWJ0aXRsZUFwcDogdHJ1ZSwgUmVxdWlyZUF1dGg6IHRydWUsIEF1dGhVUkw6IHRh"
    b"cmdldC5vcmlnaW4sCgkJTWVkaWFEaXI6IGZpbGVwYXRoLkpvaW4oZGlyZWN0b3J5LCAibWVkaWEiKSwgRGF0YURpcjogZmlsZXBhdGguSm9pbihkaXJlY3Rv"
    b"cnksICJkYXRhIiksCgkJQ2FjaGVEaXI6IGZpbGVwYXRoLkpvaW4oZGlyZWN0b3J5LCAiY2FjaGUiKSwgRkZtcGVnOiB0b29sLCBGRnByb2JlOiB0b29sLCBG"
    b"UENhbGM6IHRvb2wsCgl9KQoJdGFyZ2V0LnRscy5Db25maWcuSGFuZGxlciA9IGh0dHAuSGFuZGxlckZ1bmMoZnVuYyh3IGh0dHAuUmVzcG9uc2VXcml0ZXIs"
    b"IHIgKmh0dHAuUmVxdWVzdCkgeyBmLnNlcnZlKHRhcmdldCwgdywgcikgfSkKCXRhcmdldC50bHMuU3RhcnRUTFMoKQoJcmV0dXJuIGYsIG5pbAp9CgpmdW5j"
    b"IChmICpyZXN0b3JlUmlnKSBmYWlsQm91bmRhcnkoKSAgICAgICAgICAgICB7IGYubXUuTG9jaygpOyBmLnN0YXRlLkJvdW5kYXJ5RmFpbGVkID0gdHJ1ZTsg"
    b"Zi5tdS5VbmxvY2soKSB9CmZ1bmMgKGYgKnJlc3RvcmVSaWcpIHNuYXBzaG90KCkgcmVzdG9yZVNuYXBzaG90IHsgZi5tdS5Mb2NrKCk7IGRlZmVyIGYubXUu"
    b"VW5sb2NrKCk7IHJldHVybiBmLnN0YXRlIH0KZnVuYyAoZiAqcmVzdG9yZVJpZykgcmVsZWFzZSgpICAgICAgICAgICAgICAgICAgeyBmLnJlbGVhc2VPbmNl"
    b"LkRvKGZ1bmMoKSB7IGNsb3NlKGYucmVsZWFzZWQpIH0pIH0KZnVuYyAoZiAqcmVzdG9yZVJpZykgc3RvcCgpIGJvb2wgewoJZi5zdG9wT25jZS5EbyhmdW5j"
    b"KCkgewoJCWYuY2FuY2VsKCkKCQlmLnJlbGVhc2UoKQoJCW5ldHdvcmtPSyA6PSBmLnRhcmdldC5zdG9wKCkKCQlmLm93bmVkLldhaXQoKQoJCWlmIGVyciA6"
    b"PSBmLmZpbGVzLkNsb3NlKCk7IGVyciAhPSBuaWwgfHwgIW5ldHdvcmtPSyB7CgkJCWYubXUuTG9jaygpCgkJCWYuc3RhdGUuQm91bmRhcnlGYWlsZWQsIGYu"
    b"Y2xlYW51cEZhaWxlZCA9IHRydWUsIHRydWUKCQkJZi5tdS5VbmxvY2soKQoJCX0KCX0pCglmLm11LkxvY2soKQoJZGVmZXIgZi5tdS5VbmxvY2soKQoJcmV0"
    b"dXJuICFmLmNsZWFudXBGYWlsZWQKfQoKZnVuYyBydW5SZXN0b3JlRml4dHVyZSgpIChleGl0IGludCkgewoJdGFyZ2V0LCBlcnIgOj0gbmV3T3duZWRSZXN0"
    b"b3JlVGFyZ2V0KCkKCWlmIGVyciAhPSBuaWwgewoJCXJldHVybiAyCgl9CglkZWZlciBmdW5jKCkgewoJCWlmICF0YXJnZXQuc3RvcCgpIHsKCQkJZXhpdCA9"
    b"IDIKCQl9Cgl9KCkKCWYsIGVyciA6PSBuZXdSZXN0b3JlUmlnKHRhcmdldCwgKnJlc3RvcmVSb290LCAqcmVzdG9yZU1vZGUpCglpZiBlcnIgIT0gbmlsIHsK"
    b"CQlyZXR1cm4gMgoJfQoJZGVmZXIgZnVuYygpIHsKCQlpZiAhZi5zdG9wKCkgewoJCQlleGl0ID0gMgoJCX0KCX0oKQoJaWYganNvbi5OZXdFbmNvZGVyKG9z"
    b"LlN0ZG91dCkuRW5jb2RlKHN0cnVjdHsgS2luZCwgT3JpZ2luIHN0cmluZyB9eyJyMDYtcmVzdG9yZS1maXh0dXJlLXYxIiwgdGFyZ2V0Lm9yaWdpbn0pICE9"
    b"IG5pbCB7CgkJcmV0dXJuIDIKCX0KCXN0b3BwaW5nIDo9IG1ha2UoY2hhbiBvcy5TaWduYWwsIDEpCglzaWduYWwuTm90aWZ5KHN0b3BwaW5nLCBzeXNjYWxs"
    b"LlNJR1RFUk0sIHN5c2NhbGwuU0lHSU5UKQoJZGVmZXIgc2lnbmFsLlN0b3Aoc3RvcHBpbmcpCglzZWxlY3QgewoJY2FzZSA8LXN0b3BwaW5nOgoJY2FzZSA8"
    b"LXRpbWUuQWZ0ZXIoMTIwICogdGltZS5TZWNvbmQpOgoJfQoJcmV0dXJuIDAKfQo="
)

RAW_BLOB = "5b71ecc1bc3e94be6b516595bea89ac487f88500"
DIAGNOSTIC_BLOB = "21b445622772471c1af71fb7c20cd28a1273aca2"
RAW_SHA256 = "b808cdca52ea70579a5550a1c049e9f14e5213efafcef85ea8575630b7354f49"
DIAGNOSTIC_SHA256 = "9ddf2e4a5136dc0cf1f6954e324d001b78a51f0fc79459405ce357d6721e6a89"
SERVER_IMPORT = b'import "github.com/MikeO7/kinosail-subtitles/internal/server"\n'


def go_blob(data):
    return hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()


def source(expression):
    return b"package main\n" + SERVER_IMPORT + b"func main() { use(" + expression + b") }\n"


class QualifiedConfigControls(unittest.TestCase):
    def test_actual_raw_and_diagnostic_identity(self):
        self.assertEqual((len(RAW_MAIN), go_blob(RAW_MAIN), hashlib.sha256(RAW_MAIN).hexdigest()),
                         (4725, RAW_BLOB, RAW_SHA256))
        self.assertEqual((len(DIAGNOSTIC_MAIN), go_blob(DIAGNOSTIC_MAIN),
                          hashlib.sha256(DIAGNOSTIC_MAIN).hexdigest()),
                         (5447, DIAGNOSTIC_BLOB, DIAGNOSTIC_SHA256))

    def test_actual_main_pair_is_format_equivalent(self):
        self.assertTrue(equivalent(RAW_MAIN, DIAGNOSTIC_MAIN))

    def test_exact_server_config_final_comma(self):
        self.assertTrue(equivalent(source(b"server.Config{FPCalc: tool}"),
                                   source(b"server.Config{FPCalc: tool,}")))

    def test_selector_comments_remain_exact(self):
        original = source(b"server /* binding */ . Config{FPCalc: tool}")
        self.assertTrue(equivalent(original, source(b"server /* binding */ . Config{FPCalc: tool,}")))
        self.assertFalse(equivalent(original, source(b"server /* changed */ . Config{FPCalc: tool,}")))

    def test_unknown_packages_are_rejected(self):
        for qualifier in (b"foreign", b"serverX", b"Server"):
            with self.subTest(qualifier=qualifier):
                self.assertFalse(equivalent(source(qualifier + b".Config{FPCalc: tool}"),
                                            source(qualifier + b".Config{FPCalc: tool,}")))

    def test_unknown_server_types_are_rejected(self):
        for name in (b"OtherConfig", b"config", b"Settings"):
            with self.subTest(name=name):
                self.assertFalse(equivalent(source(b"server." + name + b"{FPCalc: tool}"),
                                            source(b"server." + name + b"{FPCalc: tool,}")))

    def test_longer_selectors_are_rejected(self):
        for prefix in (b"foreign.", b"foreign /* owner */ ."):
            with self.subTest(prefix=prefix):
                self.assertFalse(equivalent(source(prefix + b"server.Config{FPCalc: tool}"),
                                            source(prefix + b"server.Config{FPCalc: tool,}")))

    def test_block_commas_are_rejected(self):
        original = b"package main\nfunc work() { if ready { consume() } }\n"
        changed = b"package main\nfunc work() { if ready { consume(), } }\n"
        self.assertFalse(equivalent(original, changed))

    def test_internal_commas_are_preserved(self):
        original = source(b"server.Config{First: one, Second: two}")
        self.assertFalse(equivalent(original, source(b"server.Config{First: one,, Second: two,}")))
        self.assertFalse(equivalent(original, source(b"server.Config{First: one Second: two,}")))

    def test_field_names_are_preserved(self):
        self.assertFalse(equivalent(source(b"server.Config{FPCalc: tool}"),
                                    source(b"server.Config{FFmpeg: tool,}")))

    def test_field_values_are_preserved(self):
        self.assertFalse(equivalent(source(b"server.Config{FPCalc: tool}"),
                                    source(b"server.Config{FPCalc: other,}")))

    def test_field_order_is_preserved(self):
        self.assertFalse(equivalent(source(b"server.Config{First: one, Second: two}"),
                                    source(b"server.Config{Second: two, First: one,}")))

    def test_literal_spellings_are_preserved(self):
        for original, changed in ((b'"one"', b'"two"'), (b"'a'", b"'b'"),
                                  (b"`one`", b'"one"'), (b"0600", b"0o600")):
            with self.subTest(original=original):
                self.assertFalse(equivalent(source(b"server.Config{Value: " + original + b"}"),
                                            source(b"server.Config{Value: " + changed + b",}")))

    def test_import_bindings_are_preserved(self):
        original = source(b"server.Config{FPCalc: tool}")
        for replacement in (b'import alternate "github.com/MikeO7/kinosail-subtitles/internal/server"\n',
                            b'import "example.invalid/server"\n'):
            with self.subTest(replacement=replacement):
                changed = source(b"server.Config{FPCalc: tool,}").replace(SERVER_IMPORT, replacement)
                self.assertFalse(equivalent(original, changed))

    def test_grouped_type_declarations_stay_rejected(self):
        original = b"package main\ntype First struct{}\ntype Second struct{}\n"
        changed = b"package main\ntype (First struct{}; Second struct{})\n"
        self.assertFalse(equivalent(original, changed))

    def test_brace_kind_api_and_other_classifications_stay_exact(self):
        self.assertEqual(brace_kind(("word", "Config"), set(), None, None), "block")
        self.assertEqual(brace_kind(("word", "struct"), set(), None, None), "type")
        self.assertEqual(brace_kind(("word", "restoreRig"), {"restoreRig"}, None, None), "list")
        self.assertEqual(brace_kind(("op", ","), set(), None, "list"), "list")


if __name__ == "__main__":
    unittest.main()
