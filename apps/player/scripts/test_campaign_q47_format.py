"""Pure admission controls only; no formatter, filesystem, network or app runs."""
import base64
import hashlib
import unittest

from campaign_q47_format import GO_FILES, OUTPUT_CAP, format_record

FIXTURE = base64.b64decode(
    "Ly9nbzpidWlsZCBxNDdwcm9vZgoKcGFja2FnZSBtYWluCgppbXBvcnQgKAoJImNvbnRleHQiCgkiY3J5cHRvL3NoYTI1NiIK"
    "CSJlbmNvZGluZy9qc29uIgoJImVycm9ycyIKCSJmbXQiCgkiaW8iCgkibG9nIgoJIm1hcHMiCgkibmV0IgoJIm5ldC9odHRw"
    "IgoJIm9zIgoJIm9zL3NpZ25hbCIKCSJwYXRoL2ZpbGVwYXRoIgoJInN5bmMiCgkic3lzY2FsbCIKCSJ0aW1lIgopCgpjb25z"
    "dCBob2xkTGltaXQgPSAyNSAqIHRpbWUuU2Vjb25kCnZhciBtb2RlcyA9IG1hcFtzdHJpbmddYm9vbHsKCSJ2YWxpZCI6IHRy"
    "dWUsICJoZWFkZXJzIjogdHJ1ZSwgImJvZHkiOiB0cnVlLCAibG9zcyI6IHRydWUsCgkibGF0ZV9oZWFkZXJzIjogdHJ1ZSwg"
    "ImxhdGVfYm9keSI6IHRydWUsICJodHRwX2Vycm9yIjogdHJ1ZSwKfQp2YXIgYXBwcyA9IG1hcFtzdHJpbmddYm9vbHsicGxh"
    "eWVyIjogdHJ1ZSwgInN1YnRpdGxlcyI6IHRydWUsICJib3RoIjogdHJ1ZX0KdHlwZSBjb250cm9sIHN0cnVjdCB7CglPcGVy"
    "YXRpb24gc3RyaW5nIGBqc29uOiJvcGVyYXRpb24iYAoJTW9kZSAgICAgIHN0cmluZyBganNvbjoibW9kZSJgCglBcHAgICAg"
    "ICAgc3RyaW5nIGBqc29uOiJhcHAiYAp9CnR5cGUgd2l0bmVzcyBzdHJ1Y3QgewoJTW9kZSAgICAgICAgICAgc3RyaW5nICAg"
    "ICAgICAgIGBqc29uOiJtb2RlImAKCUFwcCAgICAgICAgICAgIHN0cmluZyAgICAgICAgICBganNvbjoiYXBwImAKCVRlbXBs"
    "YXRlU0hBMjU2IHN0cmluZyAgICAgICAgICBganNvbjoidGVtcGxhdGVTSEEyNTYiYAoJQ291bnRzICAgICAgICAgbWFwW3N0"
    "cmluZ11pbnQgIGBqc29uOiJjb3VudHMiYAoJQ2hlY2tzICAgICAgICAgbWFwW3N0cmluZ11ib29sIGBqc29uOiJjaGVja3Mi"
    "YAp9CnR5cGUgcGVlciBzdHJ1Y3QgewoJbXUgICAgICAgIHN5bmMuTXV0ZXgKCWZpbGVzICAgICBodHRwLkhhbmRsZXIKCXRl"
    "bXBsYXRlcyBtYXBbc3RyaW5nXVtdYnl0ZQoJcmVsZWFzZSAgIGNoYW4gc3RydWN0e30KCXN0YXRlICAgICB3aXRuZXNzCn0K"
    "CmZ1bmMgZGllKGNvZGUgc3RyaW5nKSB7CglmbXQuRnByaW50bG4ob3MuU3RkZXJyLCAicTQ3IGZpeHR1cmU6ICIrY29kZSkK"
    "CW9zLkV4aXQoMikKfQpmdW5jIG9wZW5TaXRlKHNpdGUgc3RyaW5nKSAoKm9zLlJvb3QsIGVycm9yKSB7Cgl0ZW1wb3Jhcnkg"
    "Oj0gb3MuR2V0ZW52KCJSVU5ORVJfVEVNUCIpCglpZiBsZW4oc2l0ZSkgPiA0MDk2IHx8ICFmaWxlcGF0aC5Jc0FicyhzaXRl"
    "KSB8fCBzaXRlICE9IGZpbGVwYXRoLkNsZWFuKHNpdGUpIHx8CgkJIWZpbGVwYXRoLklzQWJzKHRlbXBvcmFyeSkgfHwgdGVt"
    "cG9yYXJ5ID09ICIvIiB7CgkJcmV0dXJuIG5pbCwgZXJyb3JzLk5ldygic2l0ZSBib3VuZGFyeSIpCgl9CglyZWxhdGl2ZSwg"
    "ZXJyIDo9IGZpbGVwYXRoLlJlbCh0ZW1wb3JhcnksIHNpdGUpCglpZiBlcnIgIT0gbmlsIHx8IHJlbGF0aXZlID09ICIuIiB8"
    "fCAhZmlsZXBhdGguSXNMb2NhbChyZWxhdGl2ZSkgewoJCXJldHVybiBuaWwsIGVycm9ycy5OZXcoInNpdGUgY29udGFpbm1l"
    "bnQiKQoJfQoJcGFyZW50LCBlcnIgOj0gb3MuT3BlblJvb3QodGVtcG9yYXJ5KQoJaWYgZXJyICE9IG5pbCB7CgkJcmV0dXJu"
    "IG5pbCwgZXJyCgl9Cglyb290LCBvcGVuRXJyIDo9IHBhcmVudC5PcGVuUm9vdChyZWxhdGl2ZSkKCXJldHVybiByb290LCBl"
    "cnJvcnMuSm9pbihvcGVuRXJyLCBwYXJlbnQuQ2xvc2UoKSkKfQpmdW5jIHJlYWRQdWJsaXNoZWQocm9vdCAqb3MuUm9vdCwg"
    "bmFtZSBzdHJpbmcpIChbXWJ5dGUsIGVycm9yKSB7CglmaWxlLCBlcnIgOj0gcm9vdC5PcGVuKG5hbWUpCglpZiBlcnIgIT0g"
    "bmlsIHsKCQlyZXR1cm4gbmlsLCBlcnIKCX0KCWRhdGEsIHJlYWRFcnIgOj0gaW8uUmVhZEFsbChpby5MaW1pdFJlYWRlcihm"
    "aWxlLCAxNjM4NSkpCglpZiBlcnJvcnMuSm9pbihyZWFkRXJyLCBmaWxlLkNsb3NlKCkpICE9IG5pbCB8fCBsZW4oZGF0YSkg"
    "PCAzMiB8fCBsZW4oZGF0YSkgPiAxNjM4NCB7CgkJcmV0dXJuIG5pbCwgZXJyb3JzLk5ldygidGVtcGxhdGUgdW5hdmFpbGFi"
    "bGUiKQoJfQoJcmV0dXJuIGRhdGEsIG5pbAp9CmZ1bmMgbmV3UGVlcihyb290ICpvcy5Sb290KSAoKnBlZXIsIGVycm9yKSB7"
    "CglwIDo9ICZwZWVye2ZpbGVzOiBodHRwLkZpbGVTZXJ2ZXJGUyhyb290LkZTKCkpLCB0ZW1wbGF0ZXM6IG1hcFtzdHJpbmdd"
    "W11ieXRle319Cglmb3IgYXBwIDo9IHJhbmdlIGFwcHMgewoJCWRhdGEsIGVyciA6PSByZWFkUHVibGlzaGVkKHJvb3QsICJh"
    "c3NldHMvaW5zdGFsbC8iK2FwcCsiLnlhbWwiKQoJCWlmIGVyciAhPSBuaWwgewoJCQlyZXR1cm4gbmlsLCBlcnJvcnMuTmV3"
    "KCJ0ZW1wbGF0ZSBib3VuZCIpCgkJfQoJCXAudGVtcGxhdGVzW2FwcF0gPSBkYXRhCgl9CglwLnJlc2V0KCJ2YWxpZCIsICJw"
    "bGF5ZXIiKQoJcmV0dXJuIHAsIG5pbAp9CmZ1bmMgKHAgKnBlZXIpIHJlc2V0KG1vZGUsIGFwcCBzdHJpbmcpIHsKCXAucmVs"
    "ZWFzZSA9IG1ha2UoY2hhbiBzdHJ1Y3R7fSkKCXAuc3RhdGUgPSB3aXRuZXNze01vZGU6IG1vZGUsIEFwcDogYXBwLAoJCVRl"
    "bXBsYXRlU0hBMjU2OiBmbXQuU3ByaW50ZigiJXgiLCBzaGEyNTYuU3VtMjU2KHAudGVtcGxhdGVzW2FwcF0pKSwKCQlDb3Vu"
    "dHM6IG1hcFtzdHJpbmddaW50eyJyZXF1ZXN0cyI6IDAsICJhY3RpdmUiOiAwLCAiaG9sZHMiOiAwLAoJCQkiY2FuY2VsZWQi"
    "OiAwLCAiY29tcGxldGVkIjogMCwgImZhaWx1cmVzIjogMCwgImV4cGlyZWQiOiAwLCAidGVtcGxhdGVCeXRlcyI6IGxlbihw"
    "LnRlbXBsYXRlc1thcHBdKX0sCgkJQ2hlY2tzOiBtYXBbc3RyaW5nXWJvb2x7ImNvb2tpZVNlZW4iOiBmYWxzZSwgImF1dGhv"
    "cml6YXRpb25TZWVuIjogZmFsc2UsICJib2R5U2VlbiI6IGZhbHNlLAoJCQkicXVlcnlTZWVuIjogZmFsc2UsICJib2R5UHJl"
    "Zml4U2VudCI6IGZhbHNlfX0KfQpmdW5jIChwICpwZWVyKSBzbmFwc2hvdCgpIHdpdG5lc3MgewoJcC5tdS5Mb2NrKCkKCWRl"
    "ZmVyIHAubXUuVW5sb2NrKCkKCWNvcHkgOj0gcC5zdGF0ZQoJY29weS5Db3VudHMgPSBtYXBzLkNsb25lKGNvcHkuQ291bnRz"
    "KQoJY29weS5DaGVja3MgPSBtYXBzLkNsb25lKGNvcHkuQ2hlY2tzKQoJcmV0dXJuIGNvcHkKfQpmdW5jIChwICpwZWVyKSBj"
    "b250cm9sbGVyKHcgaHR0cC5SZXNwb25zZVdyaXRlciwgciAqaHR0cC5SZXF1ZXN0KSB7CglpZiByLk1ldGhvZCAhPSBodHRw"
    "Lk1ldGhvZFBvc3QgfHwgci5IZWFkZXIuR2V0KCJYLVE0Ny1Db250cm9sIikgIT0gIjEiIHsKCQlodHRwLkVycm9yKHcsICJj"
    "b250cm9sIHJlamVjdGVkIiwgaHR0cC5TdGF0dXNCYWRSZXF1ZXN0KQoJCXJldHVybgoJfQoJci5Cb2R5ID0gaHR0cC5NYXhC"
    "eXRlc1JlYWRlcih3LCByLkJvZHksIDUxMikKCWRlY29kZXIgOj0ganNvbi5OZXdEZWNvZGVyKHIuQm9keSkKCWRlY29kZXIu"
    "RGlzYWxsb3dVbmtub3duRmllbGRzKCkKCXZhciBuZXh0IGNvbnRyb2wKCWlmIGRlY29kZXIuRGVjb2RlKCZuZXh0KSAhPSBu"
    "aWwgfHwgZGVjb2Rlci5EZWNvZGUoJnN0cnVjdHt9e30pICE9IGlvLkVPRiB8fAoJCSFtb2Rlc1tuZXh0Lk1vZGVdIHx8ICFh"
    "cHBzW25leHQuQXBwXSB7CgkJaHR0cC5FcnJvcih3LCAiY29udHJvbCByZWplY3RlZCIsIGh0dHAuU3RhdHVzQmFkUmVxdWVz"
    "dCkKCQlyZXR1cm4KCX0KCXAubXUuTG9jaygpCglkZWZlciBwLm11LlVubG9jaygpCglzd2l0Y2ggbmV4dC5PcGVyYXRpb24g"
    "ewoJY2FzZSAic3RhcnQiOgoJCWlmIHAuc3RhdGUuQ291bnRzWyJhY3RpdmUiXSAhPSAwIHsKCQkJaHR0cC5FcnJvcih3LCAi"
    "Y29udHJvbCBidXN5IiwgaHR0cC5TdGF0dXNDb25mbGljdCkKCQkJcmV0dXJuCgkJfQoJCXAucmVzZXQobmV4dC5Nb2RlLCBu"
    "ZXh0LkFwcCkKCWNhc2UgInJlY292ZXIiOgoJCXAuc3RhdGUuTW9kZSA9ICJ2YWxpZCIKCWNhc2UgInJlbGVhc2UiOgoJCXNl"
    "bGVjdCB7CgkJY2FzZSA8LXAucmVsZWFzZToKCQlkZWZhdWx0OgoJCQljbG9zZShwLnJlbGVhc2UpCgkJfQoJZGVmYXVsdDoK"
    "CQlodHRwLkVycm9yKHcsICJjb250cm9sIHJlamVjdGVkIiwgaHR0cC5TdGF0dXNCYWRSZXF1ZXN0KQoJCXJldHVybgoJfQoJ"
    "dy5Xcml0ZUhlYWRlcihodHRwLlN0YXR1c05vQ29udGVudCkKfQpmdW5jIGZhaWxFdmVyeShtb2RlIHN0cmluZykgYm9vbCB7"
    "CglyZXR1cm4gbW9kZSA9PSAibG9zcyIgfHwgbW9kZSA9PSAiaHR0cF9lcnJvciIKfQpmdW5jIChwICpwZWVyKSBiZWdpbihy"
    "ICpodHRwLlJlcXVlc3QsIGFwcCBzdHJpbmcpIChzdHJpbmcsIDwtY2hhbiBzdHJ1Y3R7fSkgewoJcC5tdS5Mb2NrKCkKCWRl"
    "ZmVyIHAubXUuVW5sb2NrKCkKCXAuc3RhdGUuQ291bnRzWyJyZXF1ZXN0cyJdKysKCXAuc3RhdGUuQ291bnRzWyJhY3RpdmUi"
    "XSsrCglwLnN0YXRlLkNoZWNrc1siY29va2llU2VlbiJdID0gcC5zdGF0ZS5DaGVja3NbImNvb2tpZVNlZW4iXSB8fCByLkhl"
    "YWRlci5HZXQoIkNvb2tpZSIpICE9ICIiCglwLnN0YXRlLkNoZWNrc1siYXV0aG9yaXphdGlvblNlZW4iXSA9IHAuc3RhdGUu"
    "Q2hlY2tzWyJhdXRob3JpemF0aW9uU2VlbiJdIHx8IHIuSGVhZGVyLkdldCgiQXV0aG9yaXphdGlvbiIpICE9ICIiCglwLnN0"
    "YXRlLkNoZWNrc1siYm9keVNlZW4iXSA9IHAuc3RhdGUuQ2hlY2tzWyJib2R5U2VlbiJdIHx8IHIuQ29udGVudExlbmd0aCA+"
    "IDAgfHwgbGVuKHIuVHJhbnNmZXJFbmNvZGluZykgPiAwCglwLnN0YXRlLkNoZWNrc1sicXVlcnlTZWVuIl0gPSBwLnN0YXRl"
    "LkNoZWNrc1sicXVlcnlTZWVuIl0gfHwgci5VUkwuUmF3UXVlcnkgIT0gIiIKCW1vZGUgOj0gcC5zdGF0ZS5Nb2RlCglpZiBh"
    "cHAgIT0gcC5zdGF0ZS5BcHAgfHwgcC5zdGF0ZS5Db3VudHNbInJlcXVlc3RzIl0gPiAxICYmICFmYWlsRXZlcnkobW9kZSkg"
    "ewoJCW1vZGUgPSAidmFsaWQiCgl9CglyZXR1cm4gbW9kZSwgcC5yZWxlYXNlCn0KZnVuYyAocCAqcGVlcikgZmluaXNoKG91"
    "dGNvbWUgc3RyaW5nKSB7CglwLm11LkxvY2soKQoJZGVmZXIgcC5tdS5VbmxvY2soKQoJcC5zdGF0ZS5Db3VudHNbImFjdGl2"
    "ZSJdLS0KCWtleSA6PSBtYXBbc3RyaW5nXXN0cmluZ3siY29tcGxldGUiOiAiY29tcGxldGVkIiwgImNhbmNlbGVkIjogImNh"
    "bmNlbGVkIiwgImV4cGlyZWQiOiAiZXhwaXJlZCJ9W291dGNvbWVdCglpZiBrZXkgPT0gIiIgewoJCWtleSA9ICJmYWlsdXJl"
    "cyIKCX0KCXAuc3RhdGUuQ291bnRzW2tleV0rKwp9CmZ1bmMgKHAgKnBlZXIpIGhvbGQociAqaHR0cC5SZXF1ZXN0LCByZWxl"
    "YXNlIDwtY2hhbiBzdHJ1Y3R7fSkgc3RyaW5nIHsKCXAubXUuTG9jaygpCglwLnN0YXRlLkNvdW50c1siaG9sZHMiXSsrCglw"
    "Lm11LlVubG9jaygpCgl0aW1lciA6PSB0aW1lLk5ld1RpbWVyKGhvbGRMaW1pdCkKCWRlZmVyIHRpbWVyLlN0b3AoKQoJc2Vs"
    "ZWN0IHsKCWNhc2UgPC1yLkNvbnRleHQoKS5Eb25lKCk6CgkJcmV0dXJuICJjYW5jZWxlZCIKCWNhc2UgPC1yZWxlYXNlOgoJ"
    "CXJldHVybiAicmVsZWFzZWQiCgljYXNlIDwtdGltZXIuQzoKCQlyZXR1cm4gImV4cGlyZWQiCgl9Cn0KZnVuYyAocCAqcGVl"
    "cikgc2VuZFByZWZpeCh3IGh0dHAuUmVzcG9uc2VXcml0ZXIsIGRhdGEgW11ieXRlKSBib29sIHsKCXcuSGVhZGVyKCkuU2V0"
    "KCJDb250ZW50LUxlbmd0aCIsIGZtdC5TcHJpbnQobGVuKGRhdGEpKSkKCV8sIGVyciA6PSB3LldyaXRlKGRhdGFbOjMyXSkK"
    "CWlmIGVyciA9PSBuaWwgewoJCWVyciA9IGh0dHAuTmV3UmVzcG9uc2VDb250cm9sbGVyKHcpLkZsdXNoKCkKCX0KCWlmIGVy"
    "ciAhPSBuaWwgewoJCXJldHVybiBmYWxzZQoJfQoJcC5tdS5Mb2NrKCkKCXAuc3RhdGUuQ2hlY2tzWyJib2R5UHJlZml4U2Vu"
    "dCJdID0gdHJ1ZQoJcC5tdS5VbmxvY2soKQoJcmV0dXJuIHRydWUKfQpmdW5jIChwICpwZWVyKSB0ZW1wbGF0ZSh3IGh0dHAu"
    "UmVzcG9uc2VXcml0ZXIsIHIgKmh0dHAuUmVxdWVzdCwgYXBwIHN0cmluZykgewoJbW9kZSwgcmVsZWFzZSA6PSBwLmJlZ2lu"
    "KHIsIGFwcCkKCW91dGNvbWUgOj0gImZhaWxlZCIKCWRlZmVyIGZ1bmMoKSB7IHAuZmluaXNoKG91dGNvbWUpIH0oKQoJZGF0"
    "YSA6PSBwLnRlbXBsYXRlc1thcHBdCgl3LkhlYWRlcigpLlNldCgiQ29udGVudC1UeXBlIiwgImFwcGxpY2F0aW9uL3lhbWwi"
    "KQoJc3dpdGNoIG1vZGUgewoJY2FzZSAibG9zcyI6CgkJcGFuaWMoaHR0cC5FcnJBYm9ydEhhbmRsZXIpCgljYXNlICJodHRw"
    "X2Vycm9yIjoKCQlodHRwLkVycm9yKHcsICJ0ZW1wbGF0ZSB1bmF2YWlsYWJsZSIsIGh0dHAuU3RhdHVzU2VydmljZVVuYXZh"
    "aWxhYmxlKQoJCXJldHVybgoJY2FzZSAiYm9keSIsICJsYXRlX2JvZHkiOgoJCWlmICFwLnNlbmRQcmVmaXgodywgZGF0YSkg"
    "ewoJCQlyZXR1cm4KCQl9Cgl9CglpZiBtb2RlICE9ICJ2YWxpZCIgewoJCW91dGNvbWUgPSBwLmhvbGQociwgcmVsZWFzZSkK"
    "CQlpZiBvdXRjb21lICE9ICJyZWxlYXNlZCIgewoJCQlyZXR1cm4KCQl9Cgl9CglpZiBtb2RlID09ICJib2R5IiB8fCBtb2Rl"
    "ID09ICJsYXRlX2JvZHkiIHsKCQlkYXRhID0gZGF0YVszMjpdCgl9CglpZiBfLCBlcnIgOj0gdy5Xcml0ZShkYXRhKTsgZXJy"
    "ID09IG5pbCB7CgkJb3V0Y29tZSA9ICJjb21wbGV0ZSIKCX0KfQpmdW5jIChwICpwZWVyKSBzZXJ2ZSh3IGh0dHAuUmVzcG9u"
    "c2VXcml0ZXIsIHIgKmh0dHAuUmVxdWVzdCkgewoJdy5IZWFkZXIoKS5TZXQoIkNhY2hlLUNvbnRyb2wiLCAibm8tc3RvcmUi"
    "KQoJdy5IZWFkZXIoKS5TZXQoIlgtQ29udGVudC1UeXBlLU9wdGlvbnMiLCAibm9zbmlmZiIpCglpZiByLlVSTC5QYXRoID09"
    "ICIvX19xNDdfXy9jb250cm9sIiB7CgkJcC5jb250cm9sbGVyKHcsIHIpCgkJcmV0dXJuCgl9CglpZiByLk1ldGhvZCAhPSBo"
    "dHRwLk1ldGhvZEdldCB7CgkJaHR0cC5FcnJvcih3LCAibWV0aG9kIHJlamVjdGVkIiwgaHR0cC5TdGF0dXNNZXRob2ROb3RB"
    "bGxvd2VkKQoJCXJldHVybgoJfQoJaWYgci5VUkwuUGF0aCA9PSAiL19fcTQ3X18vd2l0bmVzcyIgewoJCXcuSGVhZGVyKCku"
    "U2V0KCJDb250ZW50LVR5cGUiLCAiYXBwbGljYXRpb24vanNvbiIpCgkJaWYgZXJyIDo9IGpzb24uTmV3RW5jb2Rlcih3KS5F"
    "bmNvZGUocC5zbmFwc2hvdCgpKTsgZXJyICE9IG5pbCB7CgkJCXJldHVybgoJCX0KCQlyZXR1cm4KCX0KCWZvciBhcHAgOj0g"
    "cmFuZ2UgYXBwcyB7CgkJaWYgci5VUkwuUGF0aCA9PSAiL2Fzc2V0cy9pbnN0YWxsLyIrYXBwKyIueWFtbCIgewoJCQlwLnRl"
    "bXBsYXRlKHcsIHIsIGFwcCkKCQkJcmV0dXJuCgkJfQoJfQoJcC5maWxlcy5TZXJ2ZUhUVFAodywgcikKfQpmdW5jIG1haW4o"
    "KSB7Cglyb290LCBlcnIgOj0gb3BlblNpdGUob3MuR2V0ZW52KCJLSU5PU0FJTF9RNDdfU0lURSIpKQoJaWYgZXJyICE9IG5p"
    "bCB7CgkJZGllKCJzaXRlIikKCX0KCXAsIGVyciA6PSBuZXdQZWVyKHJvb3QpCglpZiBlcnIgIT0gbmlsIHsKCQlkaWUoImFz"
    "c2V0cyIpCgl9CglsaXN0ZW5lciwgZXJyIDo9IG5ldC5MaXN0ZW5UQ1AoInRjcDQiLCAmbmV0LlRDUEFkZHJ7SVA6IG5ldC5J"
    "UHY0KDEyNywgMCwgMCwgMSksIFBvcnQ6IDQxODQ3fSkKCWlmIGVyciAhPSBuaWwgewoJCWRpZSgibGlzdGVuIikKCX0KCXNl"
    "cnZlciA6PSAmaHR0cC5TZXJ2ZXJ7SGFuZGxlcjogaHR0cC5IYW5kbGVyRnVuYyhwLnNlcnZlKSwgUmVhZEhlYWRlclRpbWVv"
    "dXQ6IDUgKiB0aW1lLlNlY29uZCwKCQlSZWFkVGltZW91dDogNSAqIHRpbWUuU2Vjb25kLCBXcml0ZVRpbWVvdXQ6IDMwICog"
    "dGltZS5TZWNvbmQsIElkbGVUaW1lb3V0OiA1ICogdGltZS5TZWNvbmQsCgkJTWF4SGVhZGVyQnl0ZXM6IDgxOTIsIEVycm9y"
    "TG9nOiBsb2cuTmV3KGlvLkRpc2NhcmQsICIiLCAwKX0KCWN0eCwgc3RvcCA6PSBzaWduYWwuTm90aWZ5Q29udGV4dChjb250"
    "ZXh0LkJhY2tncm91bmQoKSwgc3lzY2FsbC5TSUdJTlQsIHN5c2NhbGwuU0lHVEVSTSkKCWRlZmVyIHN0b3AoKQoJZ28gZnVu"
    "YygpIHsKCQlpZiBlcnIgOj0gc2VydmVyLlNlcnZlKGxpc3RlbmVyKTsgZXJyICE9IG5pbCAmJiAhZXJyb3JzLklzKGVyciwg"
    "aHR0cC5FcnJTZXJ2ZXJDbG9zZWQpIHsKCQkJZGllKCJzZXJ2ZSIpCgkJfQoJfSgpCglmbXQuUHJpbnRsbigie1wic2NoZW1h"
    "VmVyc2lvblwiOjEsXCJraW5kXCI6XCJxNDctcmVhZHlcIixcInJlYWR5XCI6dHJ1ZX0iKQoJbGlmZXRpbWUsIHN0b3BMaWZl"
    "dGltZSA6PSBjb250ZXh0LldpdGhUaW1lb3V0KGN0eCwgMjQwKnRpbWUuU2Vjb25kKQoJZGVmZXIgc3RvcExpZmV0aW1lKCkK"
    "CTwtbGlmZXRpbWUuRG9uZSgpCglzaHV0ZG93biwgY2FuY2VsIDo9IGNvbnRleHQuV2l0aFRpbWVvdXQoY29udGV4dC5CYWNr"
    "Z3JvdW5kKCksIDUqdGltZS5TZWNvbmQpCglkZWZlciBjYW5jZWwoKQoJaWYgZXJyIDo9IHNlcnZlci5TaHV0ZG93bihzaHV0"
    "ZG93bik7IGVyciAhPSBuaWwgewoJCWlmIGVyciA6PSBzZXJ2ZXIuQ2xvc2UoKTsgZXJyICE9IG5pbCB7CgkJCWRpZSgic2h1"
    "dGRvd24iKQoJCX0KCX0KCWlmIGVyciA6PSByb290LkNsb3NlKCk7IGVyciAhPSBuaWwgewoJCWRpZSgicm9vdCIpCgl9Cglm"
    "bXQuUHJpbnRsbigie1wic2NoZW1hVmVyc2lvblwiOjEsXCJraW5kXCI6XCJxNDctc3RvcHBlZFwiLFwic3RvcHBlZFwiOnRy"
    "dWV9IikKfQo=",
    validate=True)


class Q47FormatAdmissionTests(unittest.TestCase):
    def call(self, **changes):
        values = {"path": GO_FILES[0], "original": FIXTURE,
                  "output": FIXTURE, "exit_code": 0, "stderr": b""}
        values.update(changes)
        return format_record(**values)

    def test_allowlist_is_the_exact_singleton(self):
        self.assertEqual(GO_FILES, ("apps/player/e2e/compose-template-fixture.go",))

    def test_known_source_projection_preserves_input_and_output(self):
        formatted = FIXTURE + b"\n"
        before = FIXTURE
        record = self.call(output=formatted)
        self.assertEqual(record["original"]["sha256"], hashlib.sha256(FIXTURE).hexdigest())
        self.assertEqual(record["formatted"]["sha256"], hashlib.sha256(formatted).hexdigest())
        self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True), formatted)
        self.assertEqual(FIXTURE, before)
        self.assertEqual(record["originalLines"], 299)
        self.assertEqual(record["formattedLines"], 300)
        self.assertTrue(record["adoptionAllowed"])
        self.assertTrue(record["changed"])

    def test_over300_retains_bounded_diagnosis_but_never_allows_adoption(self):
        formatted = FIXTURE + b"\n\n"
        record = self.call(output=formatted)
        self.assertEqual(record["formattedLines"], 301)
        self.assertFalse(record["adoptionAllowed"])
        self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True), formatted)

    def test_unknown_paths_are_never_exported(self):
        for path in ("../private.go", "/tmp/private.go", "fixture.go", GO_FILES[0]+"\n", True):
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.call(path=path)

    def test_original_is_exact_authoritative_public_fixture(self):
        for original in (FIXTURE+b" ", FIXTURE[:-1], b"package main\n", None, "", b""):
            with self.subTest(kind=type(original).__name__), self.assertRaises(ValueError):
                self.call(original=original)

    def test_only_bounded_utf8_nonempty_output_is_projected(self):
        for output in (None, "", b"", b"x"*(OUTPUT_CAP+1), b"\xff", b"package\0main"):
            with self.subTest(kind=type(output).__name__), self.assertRaises(ValueError):
                self.call(output=output)

    def test_boolean_or_nonzero_exit_is_not_formatter_success(self):
        for code in (True, False, None, "0", 1, -9):
            with self.subTest(code=code), self.assertRaises(ValueError):
                self.call(exit_code=code)

    def test_stderr_is_neither_exported_nor_ignored(self):
        for stderr in (b"fictional diagnostic", None, ""):
            with self.subTest(kind=type(stderr).__name__), self.assertRaises(ValueError):
                self.call(stderr=stderr)

    def test_unchanged_projection_still_requires_separate_review(self):
        record = self.call()
        self.assertFalse(record["changed"])
        self.assertTrue(record["adoptionAllowed"])
        self.assertTrue(record["reviewRequired"])
        self.assertEqual(record["path"], GO_FILES[0])


if __name__ == "__main__":
    unittest.main()
