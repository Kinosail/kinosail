"""Pure admission controls; no formatter, filesystem, network or app runs."""
import base64
import hashlib
import unittest

from campaign_q47_format import GO_FILES, OUTPUT_CAP, format_record

FIXTURES = {
    'apps/player/e2e/compose-template-fixture.go': base64.b64decode(
        'Ly9nbzpidWlsZCBxNDdwcm9vZgoKcGFja2FnZSBtYWluCgppbXBvcnQgKAoJImNvbnRleHQiCgkiZXJyb3JzIgoJImZtdCIKCSJp'
        'byIKCSJsb2ciCgkibmV0IgoJIm5ldC9odHRwIgoJIm9zIgoJIm9zL3NpZ25hbCIKCSJwYXRoL2ZpbGVwYXRoIgoJInN5c2NhbGwi'
        'CgkidGltZSIKKQoKZnVuYyBkaWUoY29kZSBzdHJpbmcpIHsKCWZtdC5GcHJpbnRsbihvcy5TdGRlcnIsICJxNDcgZml4dHVyZTog'
        'Iitjb2RlKQoJb3MuRXhpdCgyKQp9CmZ1bmMgb3BlblNpdGUoc2l0ZSBzdHJpbmcpICgqb3MuUm9vdCwgZXJyb3IpIHsKCXRlbXBv'
        'cmFyeSA6PSBvcy5HZXRlbnYoIlJVTk5FUl9URU1QIikKCWlmIGxlbihzaXRlKSA+IDQwOTYgfHwgIWZpbGVwYXRoLklzQWJzKHNp'
        'dGUpIHx8IHNpdGUgIT0gZmlsZXBhdGguQ2xlYW4oc2l0ZSkgfHwKCQkhZmlsZXBhdGguSXNBYnModGVtcG9yYXJ5KSB8fCB0ZW1w'
        'b3JhcnkgPT0gIi8iIHsKCQlyZXR1cm4gbmlsLCBlcnJvcnMuTmV3KCJzaXRlIGJvdW5kYXJ5IikKCX0KCXJlbGF0aXZlLCBlcnIg'
        'Oj0gZmlsZXBhdGguUmVsKHRlbXBvcmFyeSwgc2l0ZSkKCWlmIGVyciAhPSBuaWwgfHwgcmVsYXRpdmUgPT0gIi4iIHx8ICFmaWxl'
        'cGF0aC5Jc0xvY2FsKHJlbGF0aXZlKSB7CgkJcmV0dXJuIG5pbCwgZXJyb3JzLk5ldygic2l0ZSBjb250YWlubWVudCIpCgl9Cglw'
        'YXJlbnQsIGVyciA6PSBvcy5PcGVuUm9vdCh0ZW1wb3JhcnkpCglpZiBlcnIgIT0gbmlsIHsKCQlyZXR1cm4gbmlsLCBlcnIKCX0K'
        'CXJvb3QsIG9wZW5FcnIgOj0gcGFyZW50Lk9wZW5Sb290KHJlbGF0aXZlKQoJcmV0dXJuIHJvb3QsIGVycm9ycy5Kb2luKG9wZW5F'
        'cnIsIHBhcmVudC5DbG9zZSgpKQp9CmZ1bmMgcmVhZFB1Ymxpc2hlZChyb290ICpvcy5Sb290LCBuYW1lIHN0cmluZykgKFtdYnl0'
        'ZSwgZXJyb3IpIHsKCWZpbGUsIGVyciA6PSByb290Lk9wZW4obmFtZSkKCWlmIGVyciAhPSBuaWwgewoJCXJldHVybiBuaWwsIGVy'
        'cgoJfQoJZGF0YSwgcmVhZEVyciA6PSBpby5SZWFkQWxsKGlvLkxpbWl0UmVhZGVyKGZpbGUsIDE2Mzg1KSkKCWlmIGVycm9ycy5K'
        'b2luKHJlYWRFcnIsIGZpbGUuQ2xvc2UoKSkgIT0gbmlsIHx8IGxlbihkYXRhKSA8IDMyIHx8IGxlbihkYXRhKSA+IDE2Mzg0IHsK'
        'CQlyZXR1cm4gbmlsLCBlcnJvcnMuTmV3KCJ0ZW1wbGF0ZSB1bmF2YWlsYWJsZSIpCgl9CglyZXR1cm4gZGF0YSwgbmlsCn0KZnVu'
        'YyBuZXdQZWVyKHJvb3QgKm9zLlJvb3QpICgqcGVlciwgZXJyb3IpIHsKCXAgOj0gJnBlZXJ7ZmlsZXM6IGh0dHAuRmlsZVNlcnZl'
        'ckZTKHJvb3QuRlMoKSksIHRlbXBsYXRlczogbWFwW3N0cmluZ11bXWJ5dGV7fX0KCWZvciBhcHAgOj0gcmFuZ2UgYXBwcyB7CgkJ'
        'ZGF0YSwgZXJyIDo9IHJlYWRQdWJsaXNoZWQocm9vdCwgImFzc2V0cy9pbnN0YWxsLyIrYXBwKyIueWFtbCIpCgkJaWYgZXJyICE9'
        'IG5pbCB7CgkJCXJldHVybiBuaWwsIGVycm9ycy5OZXcoInRlbXBsYXRlIGJvdW5kIikKCQl9CgkJcC50ZW1wbGF0ZXNbYXBwXSA9'
        'IGRhdGEKCX0KCXAucmVzZXQoInZhbGlkIiwgInBsYXllciIpCglyZXR1cm4gcCwgbmlsCn0KCmZ1bmMgbWFpbigpIHsKCXJvb3Qs'
        'IGVyciA6PSBvcGVuU2l0ZShvcy5HZXRlbnYoIktJTk9TQUlMX1E0N19TSVRFIikpCglpZiBlcnIgIT0gbmlsIHsKCQlkaWUoInNp'
        'dGUiKQoJfQoJcCwgZXJyIDo9IG5ld1BlZXIocm9vdCkKCWlmIGVyciAhPSBuaWwgewoJCWRpZSgiYXNzZXRzIikKCX0KCWxpc3Rl'
        'bmVyLCBlcnIgOj0gbmV0Lkxpc3RlblRDUCgidGNwNCIsICZuZXQuVENQQWRkcntJUDogbmV0LklQdjQoMTI3LCAwLCAwLCAxKSwg'
        'UG9ydDogNDE4NDd9KQoJaWYgZXJyICE9IG5pbCB7CgkJZGllKCJsaXN0ZW4iKQoJfQoJc2VydmVyIDo9ICZodHRwLlNlcnZlcntI'
        'YW5kbGVyOiBodHRwLkhhbmRsZXJGdW5jKHAuc2VydmUpLCBSZWFkSGVhZGVyVGltZW91dDogNSAqIHRpbWUuU2Vjb25kLAoJCVJl'
        'YWRUaW1lb3V0OiA1ICogdGltZS5TZWNvbmQsIFdyaXRlVGltZW91dDogMzAgKiB0aW1lLlNlY29uZCwgSWRsZVRpbWVvdXQ6IDUg'
        'KiB0aW1lLlNlY29uZCwKCQlNYXhIZWFkZXJCeXRlczogODE5MiwgRXJyb3JMb2c6IGxvZy5OZXcoaW8uRGlzY2FyZCwgIiIsIDAp'
        'fQoJY3R4LCBzdG9wIDo9IHNpZ25hbC5Ob3RpZnlDb250ZXh0KGNvbnRleHQuQmFja2dyb3VuZCgpLCBzeXNjYWxsLlNJR0lOVCwg'
        'c3lzY2FsbC5TSUdURVJNKQoJZGVmZXIgc3RvcCgpCglnbyBmdW5jKCkgewoJCWlmIGVyciA6PSBzZXJ2ZXIuU2VydmUobGlzdGVu'
        'ZXIpOyBlcnIgIT0gbmlsICYmICFlcnJvcnMuSXMoZXJyLCBodHRwLkVyclNlcnZlckNsb3NlZCkgewoJCQlkaWUoInNlcnZlIikK'
        'CQl9Cgl9KCkKCWZtdC5QcmludGxuKCJ7XCJzY2hlbWFWZXJzaW9uXCI6MSxcImtpbmRcIjpcInE0Ny1yZWFkeVwiLFwicmVhZHlc'
        'Ijp0cnVlfSIpCglsaWZldGltZSwgc3RvcExpZmV0aW1lIDo9IGNvbnRleHQuV2l0aFRpbWVvdXQoY3R4LCAyNDAqdGltZS5TZWNv'
        'bmQpCglkZWZlciBzdG9wTGlmZXRpbWUoKQoJPC1saWZldGltZS5Eb25lKCkKCXNodXRkb3duLCBjYW5jZWwgOj0gY29udGV4dC5X'
        'aXRoVGltZW91dChjb250ZXh0LkJhY2tncm91bmQoKSwgNSp0aW1lLlNlY29uZCkKCWRlZmVyIGNhbmNlbCgpCglpZiBlcnIgOj0g'
        'c2VydmVyLlNodXRkb3duKHNodXRkb3duKTsgZXJyICE9IG5pbCB7CgkJaWYgZXJyIDo9IHNlcnZlci5DbG9zZSgpOyBlcnIgIT0g'
        'bmlsIHsKCQkJZGllKCJzaHV0ZG93biIpCgkJfQoJfQoJaWYgZXJyIDo9IHJvb3QuQ2xvc2UoKTsgZXJyICE9IG5pbCB7CgkJZGll'
        'KCJyb290IikKCX0KCWZtdC5QcmludGxuKCJ7XCJzY2hlbWFWZXJzaW9uXCI6MSxcImtpbmRcIjpcInE0Ny1zdG9wcGVkXCIsXCJz'
        'dG9wcGVkXCI6dHJ1ZX0iKQp9Cg=='
        , validate=True),
    'apps/player/e2e/compose-template-peer.go': base64.b64decode(
        'Ly9nbzpidWlsZCBxNDdwcm9vZgoKcGFja2FnZSBtYWluCgppbXBvcnQgKAoJImNyeXB0by9zaGEyNTYiCgkiZW5jb2RpbmcvanNv'
        'biIKCSJmbXQiCgkiaW8iCgkibWFwcyIKCSJuZXQvaHR0cCIKCSJzeW5jIgoJInRpbWUiCikKCmNvbnN0IGhvbGRMaW1pdCA9IDI1'
        'ICogdGltZS5TZWNvbmQKCnZhciBtb2RlcyA9IG1hcFtzdHJpbmddYm9vbHsKCSJ2YWxpZCI6IHRydWUsICJoZWFkZXJzIjogdHJ1'
        'ZSwgImJvZHkiOiB0cnVlLCAibG9zcyI6IHRydWUsCgkibGF0ZV9oZWFkZXJzIjogdHJ1ZSwgImxhdGVfYm9keSI6IHRydWUsICJo'
        'dHRwX2Vycm9yIjogdHJ1ZSwKfQp2YXIgYXBwcyA9IG1hcFtzdHJpbmddYm9vbHsicGxheWVyIjogdHJ1ZSwgInN1YnRpdGxlcyI6'
        'IHRydWUsICJib3RoIjogdHJ1ZX0KCnR5cGUgY29udHJvbCBzdHJ1Y3QgewoJT3BlcmF0aW9uIHN0cmluZyBganNvbjoib3BlcmF0'
        'aW9uImAKCU1vZGUgICAgICBzdHJpbmcgYGpzb246Im1vZGUiYAoJQXBwICAgICAgIHN0cmluZyBganNvbjoiYXBwImAKfQp0eXBl'
        'IHdpdG5lc3Mgc3RydWN0IHsKCU1vZGUgICAgICAgICAgIHN0cmluZyAgICAgICAgICBganNvbjoibW9kZSJgCglBcHAgICAgICAg'
        'ICAgICBzdHJpbmcgICAgICAgICAgYGpzb246ImFwcCJgCglUZW1wbGF0ZVNIQTI1NiBzdHJpbmcgICAgICAgICAgYGpzb246InRl'
        'bXBsYXRlU0hBMjU2ImAKCUNvdW50cyAgICAgICAgIG1hcFtzdHJpbmddaW50ICBganNvbjoiY291bnRzImAKCUNoZWNrcyAgICAg'
        'ICAgIG1hcFtzdHJpbmddYm9vbCBganNvbjoiY2hlY2tzImAKfQp0eXBlIHBlZXIgc3RydWN0IHsKCW11ICAgICAgICBzeW5jLk11'
        'dGV4CglmaWxlcyAgICAgaHR0cC5IYW5kbGVyCgl0ZW1wbGF0ZXMgbWFwW3N0cmluZ11bXWJ5dGUKCXJlbGVhc2UgICBjaGFuIHN0'
        'cnVjdHt9CglzdGF0ZSAgICAgd2l0bmVzcwp9CgpmdW5jIChwICpwZWVyKSByZXNldChtb2RlLCBhcHAgc3RyaW5nKSB7CglwLnJl'
        'bGVhc2UgPSBtYWtlKGNoYW4gc3RydWN0e30pCglwLnN0YXRlID0gd2l0bmVzc3tNb2RlOiBtb2RlLCBBcHA6IGFwcCwKCQlUZW1w'
        'bGF0ZVNIQTI1NjogZm10LlNwcmludGYoIiV4Iiwgc2hhMjU2LlN1bTI1NihwLnRlbXBsYXRlc1thcHBdKSksCgkJQ291bnRzOiBt'
        'YXBbc3RyaW5nXWludHsicmVxdWVzdHMiOiAwLCAiYWN0aXZlIjogMCwgImhvbGRzIjogMCwKCQkJImNhbmNlbGVkIjogMCwgImNv'
        'bXBsZXRlZCI6IDAsICJmYWlsdXJlcyI6IDAsICJleHBpcmVkIjogMCwgInRlbXBsYXRlQnl0ZXMiOiBsZW4ocC50ZW1wbGF0ZXNb'
        'YXBwXSl9LAoJCUNoZWNrczogbWFwW3N0cmluZ11ib29seyJjb29raWVTZWVuIjogZmFsc2UsICJhdXRob3JpemF0aW9uU2VlbiI6'
        'IGZhbHNlLCAiYm9keVNlZW4iOiBmYWxzZSwKCQkJInF1ZXJ5U2VlbiI6IGZhbHNlLCAiYm9keVByZWZpeFNlbnQiOiBmYWxzZX19'
        'Cn0KZnVuYyAocCAqcGVlcikgc25hcHNob3QoKSB3aXRuZXNzIHsKCXAubXUuTG9jaygpCglkZWZlciBwLm11LlVubG9jaygpCglj'
        'b3B5IDo9IHAuc3RhdGUKCWNvcHkuQ291bnRzID0gbWFwcy5DbG9uZShjb3B5LkNvdW50cykKCWNvcHkuQ2hlY2tzID0gbWFwcy5D'
        'bG9uZShjb3B5LkNoZWNrcykKCXJldHVybiBjb3B5Cn0KZnVuYyAocCAqcGVlcikgY29udHJvbGxlcih3IGh0dHAuUmVzcG9uc2VX'
        'cml0ZXIsIHIgKmh0dHAuUmVxdWVzdCkgewoJaWYgci5NZXRob2QgIT0gaHR0cC5NZXRob2RQb3N0IHx8IHIuSGVhZGVyLkdldCgi'
        'WC1RNDctQ29udHJvbCIpICE9ICIxIiB7CgkJaHR0cC5FcnJvcih3LCAiY29udHJvbCByZWplY3RlZCIsIGh0dHAuU3RhdHVzQmFk'
        'UmVxdWVzdCkKCQlyZXR1cm4KCX0KCXIuQm9keSA9IGh0dHAuTWF4Qnl0ZXNSZWFkZXIodywgci5Cb2R5LCA1MTIpCglkZWNvZGVy'
        'IDo9IGpzb24uTmV3RGVjb2RlcihyLkJvZHkpCglkZWNvZGVyLkRpc2FsbG93VW5rbm93bkZpZWxkcygpCgl2YXIgbmV4dCBjb250'
        'cm9sCglpZiBkZWNvZGVyLkRlY29kZSgmbmV4dCkgIT0gbmlsIHx8IGRlY29kZXIuRGVjb2RlKCZzdHJ1Y3R7fXt9KSAhPSBpby5F'
        'T0YgfHwKCQkhbW9kZXNbbmV4dC5Nb2RlXSB8fCAhYXBwc1tuZXh0LkFwcF0gewoJCWh0dHAuRXJyb3IodywgImNvbnRyb2wgcmVq'
        'ZWN0ZWQiLCBodHRwLlN0YXR1c0JhZFJlcXVlc3QpCgkJcmV0dXJuCgl9CglwLm11LkxvY2soKQoJZGVmZXIgcC5tdS5VbmxvY2so'
        'KQoJc3dpdGNoIG5leHQuT3BlcmF0aW9uIHsKCWNhc2UgInN0YXJ0IjoKCQlpZiBwLnN0YXRlLkNvdW50c1siYWN0aXZlIl0gIT0g'
        'MCB7CgkJCWh0dHAuRXJyb3IodywgImNvbnRyb2wgYnVzeSIsIGh0dHAuU3RhdHVzQ29uZmxpY3QpCgkJCXJldHVybgoJCX0KCQlw'
        'LnJlc2V0KG5leHQuTW9kZSwgbmV4dC5BcHApCgljYXNlICJyZWNvdmVyIjoKCQlwLnN0YXRlLk1vZGUgPSAidmFsaWQiCgljYXNl'
        'ICJyZWxlYXNlIjoKCQlzZWxlY3QgewoJCWNhc2UgPC1wLnJlbGVhc2U6CgkJZGVmYXVsdDoKCQkJY2xvc2UocC5yZWxlYXNlKQoJ'
        'CX0KCWRlZmF1bHQ6CgkJaHR0cC5FcnJvcih3LCAiY29udHJvbCByZWplY3RlZCIsIGh0dHAuU3RhdHVzQmFkUmVxdWVzdCkKCQly'
        'ZXR1cm4KCX0KCXcuV3JpdGVIZWFkZXIoaHR0cC5TdGF0dXNOb0NvbnRlbnQpCn0KZnVuYyBmYWlsRXZlcnkobW9kZSBzdHJpbmcp'
        'IGJvb2wgewoJcmV0dXJuIG1vZGUgPT0gImxvc3MiIHx8IG1vZGUgPT0gImh0dHBfZXJyb3IiCn0KZnVuYyAocCAqcGVlcikgYmVn'
        'aW4ociAqaHR0cC5SZXF1ZXN0LCBhcHAgc3RyaW5nKSAoc3RyaW5nLCA8LWNoYW4gc3RydWN0e30pIHsKCXAubXUuTG9jaygpCglk'
        'ZWZlciBwLm11LlVubG9jaygpCglwLnN0YXRlLkNvdW50c1sicmVxdWVzdHMiXSsrCglwLnN0YXRlLkNvdW50c1siYWN0aXZlIl0r'
        'KwoJcC5zdGF0ZS5DaGVja3NbImNvb2tpZVNlZW4iXSA9IHAuc3RhdGUuQ2hlY2tzWyJjb29raWVTZWVuIl0gfHwgci5IZWFkZXIu'
        'R2V0KCJDb29raWUiKSAhPSAiIgoJcC5zdGF0ZS5DaGVja3NbImF1dGhvcml6YXRpb25TZWVuIl0gPSBwLnN0YXRlLkNoZWNrc1si'
        'YXV0aG9yaXphdGlvblNlZW4iXSB8fCByLkhlYWRlci5HZXQoIkF1dGhvcml6YXRpb24iKSAhPSAiIgoJcC5zdGF0ZS5DaGVja3Nb'
        'ImJvZHlTZWVuIl0gPSBwLnN0YXRlLkNoZWNrc1siYm9keVNlZW4iXSB8fCByLkNvbnRlbnRMZW5ndGggPiAwIHx8IGxlbihyLlRy'
        'YW5zZmVyRW5jb2RpbmcpID4gMAoJcC5zdGF0ZS5DaGVja3NbInF1ZXJ5U2VlbiJdID0gcC5zdGF0ZS5DaGVja3NbInF1ZXJ5U2Vl'
        'biJdIHx8IHIuVVJMLlJhd1F1ZXJ5ICE9ICIiCgltb2RlIDo9IHAuc3RhdGUuTW9kZQoJaWYgYXBwICE9IHAuc3RhdGUuQXBwIHx8'
        'IHAuc3RhdGUuQ291bnRzWyJyZXF1ZXN0cyJdID4gMSAmJiAhZmFpbEV2ZXJ5KG1vZGUpIHsKCQltb2RlID0gInZhbGlkIgoJfQoJ'
        'cmV0dXJuIG1vZGUsIHAucmVsZWFzZQp9CmZ1bmMgKHAgKnBlZXIpIGZpbmlzaChvdXRjb21lIHN0cmluZykgewoJcC5tdS5Mb2Nr'
        'KCkKCWRlZmVyIHAubXUuVW5sb2NrKCkKCXAuc3RhdGUuQ291bnRzWyJhY3RpdmUiXS0tCglrZXkgOj0gbWFwW3N0cmluZ11zdHJp'
        'bmd7ImNvbXBsZXRlIjogImNvbXBsZXRlZCIsICJjYW5jZWxlZCI6ICJjYW5jZWxlZCIsICJleHBpcmVkIjogImV4cGlyZWQifVtv'
        'dXRjb21lXQoJaWYga2V5ID09ICIiIHsKCQlrZXkgPSAiZmFpbHVyZXMiCgl9CglwLnN0YXRlLkNvdW50c1trZXldKysKfQpmdW5j'
        'IChwICpwZWVyKSBob2xkKHIgKmh0dHAuUmVxdWVzdCwgcmVsZWFzZSA8LWNoYW4gc3RydWN0e30pIHN0cmluZyB7CglwLm11Lkxv'
        'Y2soKQoJcC5zdGF0ZS5Db3VudHNbImhvbGRzIl0rKwoJcC5tdS5VbmxvY2soKQoJdGltZXIgOj0gdGltZS5OZXdUaW1lcihob2xk'
        'TGltaXQpCglkZWZlciB0aW1lci5TdG9wKCkKCXNlbGVjdCB7CgljYXNlIDwtci5Db250ZXh0KCkuRG9uZSgpOgoJCXJldHVybiAi'
        'Y2FuY2VsZWQiCgljYXNlIDwtcmVsZWFzZToKCQlyZXR1cm4gInJlbGVhc2VkIgoJY2FzZSA8LXRpbWVyLkM6CgkJcmV0dXJuICJl'
        'eHBpcmVkIgoJfQp9CmZ1bmMgKHAgKnBlZXIpIHNlbmRQcmVmaXgodyBodHRwLlJlc3BvbnNlV3JpdGVyLCBkYXRhIFtdYnl0ZSkg'
        'Ym9vbCB7Cgl3LkhlYWRlcigpLlNldCgiQ29udGVudC1MZW5ndGgiLCBmbXQuU3ByaW50KGxlbihkYXRhKSkpCglfLCBlcnIgOj0g'
        'dy5Xcml0ZShkYXRhWzozMl0pCglpZiBlcnIgPT0gbmlsIHsKCQllcnIgPSBodHRwLk5ld1Jlc3BvbnNlQ29udHJvbGxlcih3KS5G'
        'bHVzaCgpCgl9CglpZiBlcnIgIT0gbmlsIHsKCQlyZXR1cm4gZmFsc2UKCX0KCXAubXUuTG9jaygpCglwLnN0YXRlLkNoZWNrc1si'
        'Ym9keVByZWZpeFNlbnQiXSA9IHRydWUKCXAubXUuVW5sb2NrKCkKCXJldHVybiB0cnVlCn0KZnVuYyAocCAqcGVlcikgdGVtcGxh'
        'dGUodyBodHRwLlJlc3BvbnNlV3JpdGVyLCByICpodHRwLlJlcXVlc3QsIGFwcCBzdHJpbmcpIHsKCW1vZGUsIHJlbGVhc2UgOj0g'
        'cC5iZWdpbihyLCBhcHApCglvdXRjb21lIDo9ICJmYWlsZWQiCglkZWZlciBmdW5jKCkgeyBwLmZpbmlzaChvdXRjb21lKSB9KCkK'
        'CWRhdGEgOj0gcC50ZW1wbGF0ZXNbYXBwXQoJdy5IZWFkZXIoKS5TZXQoIkNvbnRlbnQtVHlwZSIsICJhcHBsaWNhdGlvbi95YW1s'
        'IikKCXN3aXRjaCBtb2RlIHsKCWNhc2UgImxvc3MiOgoJCXBhbmljKGh0dHAuRXJyQWJvcnRIYW5kbGVyKQoJY2FzZSAiaHR0cF9l'
        'cnJvciI6CgkJaHR0cC5FcnJvcih3LCAidGVtcGxhdGUgdW5hdmFpbGFibGUiLCBodHRwLlN0YXR1c1NlcnZpY2VVbmF2YWlsYWJs'
        'ZSkKCQlyZXR1cm4KCWNhc2UgImJvZHkiLCAibGF0ZV9ib2R5IjoKCQlpZiAhcC5zZW5kUHJlZml4KHcsIGRhdGEpIHsKCQkJcmV0'
        'dXJuCgkJfQoJfQoJaWYgbW9kZSAhPSAidmFsaWQiIHsKCQlvdXRjb21lID0gcC5ob2xkKHIsIHJlbGVhc2UpCgkJaWYgb3V0Y29t'
        'ZSAhPSAicmVsZWFzZWQiIHsKCQkJcmV0dXJuCgkJfQoJfQoJaWYgbW9kZSA9PSAiYm9keSIgfHwgbW9kZSA9PSAibGF0ZV9ib2R5'
        'IiB7CgkJZGF0YSA9IGRhdGFbMzI6XQoJfQoJaWYgXywgZXJyIDo9IHcuV3JpdGUoZGF0YSk7IGVyciA9PSBuaWwgewoJCW91dGNv'
        'bWUgPSAiY29tcGxldGUiCgl9Cn0KZnVuYyAocCAqcGVlcikgc2VydmUodyBodHRwLlJlc3BvbnNlV3JpdGVyLCByICpodHRwLlJl'
        'cXVlc3QpIHsKCXcuSGVhZGVyKCkuU2V0KCJDYWNoZS1Db250cm9sIiwgIm5vLXN0b3JlIikKCXcuSGVhZGVyKCkuU2V0KCJYLUNv'
        'bnRlbnQtVHlwZS1PcHRpb25zIiwgIm5vc25pZmYiKQoJaWYgci5VUkwuUGF0aCA9PSAiL19fcTQ3X18vY29udHJvbCIgewoJCXAu'
        'Y29udHJvbGxlcih3LCByKQoJCXJldHVybgoJfQoJaWYgci5NZXRob2QgIT0gaHR0cC5NZXRob2RHZXQgewoJCWh0dHAuRXJyb3Io'
        'dywgIm1ldGhvZCByZWplY3RlZCIsIGh0dHAuU3RhdHVzTWV0aG9kTm90QWxsb3dlZCkKCQlyZXR1cm4KCX0KCWlmIHIuVVJMLlBh'
        'dGggPT0gIi9fX3E0N19fL3dpdG5lc3MiIHsKCQl3LkhlYWRlcigpLlNldCgiQ29udGVudC1UeXBlIiwgImFwcGxpY2F0aW9uL2pz'
        'b24iKQoJCWlmIGVyciA6PSBqc29uLk5ld0VuY29kZXIodykuRW5jb2RlKHAuc25hcHNob3QoKSk7IGVyciAhPSBuaWwgewoJCQly'
        'ZXR1cm4KCQl9CgkJcmV0dXJuCgl9Cglmb3IgYXBwIDo9IHJhbmdlIGFwcHMgewoJCWlmIHIuVVJMLlBhdGggPT0gIi9hc3NldHMv'
        'aW5zdGFsbC8iK2FwcCsiLnlhbWwiIHsKCQkJcC50ZW1wbGF0ZSh3LCByLCBhcHApCgkJCXJldHVybgoJCX0KCX0KCXAuZmlsZXMu'
        'U2VydmVIVFRQKHcsIHIpCn0K'
        , validate=True),
}


class Q47FormatAdmissionTests(unittest.TestCase):
    def call(self, **changes):
        path = changes.get("path", GO_FILES[0])
        original = FIXTURES.get(path, FIXTURES[GO_FILES[0]])
        values = {"path": path, "original": original,
                  "output": original, "exit_code": 0, "stderr": b""}
        values.update(changes)
        return format_record(**values)

    def test_allowlist_is_the_exact_pair(self):
        self.assertEqual(GO_FILES, (
            "apps/player/e2e/compose-template-fixture.go",
            "apps/player/e2e/compose-template-peer.go"))
        self.assertEqual(set(GO_FILES), set(FIXTURES))

    def test_known_source_projection_preserves_input_and_output(self):
        for path, original in FIXTURES.items():
            with self.subTest(path=path):
                formatted = original + b"\n" * (300 - len(original.splitlines()))
                before = original
                record = self.call(path=path, output=formatted)
                self.assertEqual(record["original"]["sha256"], hashlib.sha256(original).hexdigest())
                self.assertEqual(record["formatted"]["sha256"], hashlib.sha256(formatted).hexdigest())
                self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True), formatted)
                self.assertEqual(original, before)
                self.assertEqual(record["originalLines"], len(original.splitlines()))
                self.assertEqual(record["formattedLines"], 300)
                self.assertTrue(record["adoptionAllowed"])
                self.assertTrue(record["changed"])

    def test_over300_retains_bounded_diagnosis_but_never_allows_adoption(self):
        for path, original in FIXTURES.items():
            with self.subTest(path=path):
                formatted = original + b"\n" * (301 - len(original.splitlines()))
                record = self.call(path=path, output=formatted)
                self.assertEqual(record["formattedLines"], 301)
                self.assertFalse(record["adoptionAllowed"])
                self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True), formatted)

    def test_unknown_paths_are_never_exported(self):
        paths = ("../private.go", "/tmp/private.go", "fixture.go", True)
        for path in (*paths, *(name+"\n" for name in GO_FILES)):
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.call(path=path)

    def test_original_is_exact_authoritative_public_fixture(self):
        for path, source in FIXTURES.items():
            for original in (source+b" ", source[:-1], b"package main\n", None, "", b""):
                with self.subTest(path=path, kind=type(original).__name__), self.assertRaises(ValueError):
                    self.call(path=path, original=original)

    def test_pinned_originals_cannot_be_swapped_between_known_paths(self):
        for path, other in ((GO_FILES[0], GO_FILES[1]), (GO_FILES[1], GO_FILES[0])):
            with self.subTest(path=path), self.assertRaises(ValueError):
                self.call(path=path, original=FIXTURES[other])

    def test_only_bounded_utf8_nonempty_output_is_projected(self):
        for path in GO_FILES:
            for output in (None, "", b"", b"x"*(OUTPUT_CAP+1), b"\xff", b"package\0main"):
                with self.subTest(path=path, kind=type(output).__name__), self.assertRaises(ValueError):
                    self.call(path=path, output=output)

    def test_boolean_or_nonzero_exit_is_not_formatter_success(self):
        for path in GO_FILES:
            for code in (True, False, None, "0", 1, -9):
                with self.subTest(path=path, code=code), self.assertRaises(ValueError):
                    self.call(path=path, exit_code=code)

    def test_stderr_is_neither_exported_nor_ignored(self):
        for path in GO_FILES:
            for stderr in (b"fictional diagnostic", None, ""):
                with self.subTest(path=path, kind=type(stderr).__name__), self.assertRaises(ValueError):
                    self.call(path=path, stderr=stderr)

    def test_unchanged_projection_still_requires_separate_review(self):
        for path in GO_FILES:
            with self.subTest(path=path):
                record = self.call(path=path)
                self.assertFalse(record["changed"])
                self.assertTrue(record["adoptionAllowed"])
                self.assertTrue(record["reviewRequired"])
                self.assertEqual(record["path"], path)


if __name__ == "__main__":
    unittest.main()
