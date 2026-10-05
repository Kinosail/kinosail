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
        'ICogdGltZS5TZWNvbmQKdmFyIG1vZGVzID0gbWFwW3N0cmluZ11ib29sewoJInZhbGlkIjogdHJ1ZSwgImhlYWRlcnMiOiB0cnVl'
        'LCAiYm9keSI6IHRydWUsICJsb3NzIjogdHJ1ZSwKCSJsYXRlX2hlYWRlcnMiOiB0cnVlLCAibGF0ZV9ib2R5IjogdHJ1ZSwgImh0'
        'dHBfZXJyb3IiOiB0cnVlLAp9CnZhciBhcHBzID0gbWFwW3N0cmluZ11ib29seyJwbGF5ZXIiOiB0cnVlLCAic3VidGl0bGVzIjog'
        'dHJ1ZSwgImJvdGgiOiB0cnVlfQp0eXBlIGNvbnRyb2wgc3RydWN0IHsKCU9wZXJhdGlvbiBzdHJpbmcgYGpzb246Im9wZXJhdGlv'
        'biJgCglNb2RlICAgICAgc3RyaW5nIGBqc29uOiJtb2RlImAKCUFwcCAgICAgICBzdHJpbmcgYGpzb246ImFwcCJgCn0KdHlwZSB3'
        'aXRuZXNzIHN0cnVjdCB7CglNb2RlICAgICAgICAgICBzdHJpbmcgICAgICAgICAgYGpzb246Im1vZGUiYAoJQXBwICAgICAgICAg'
        'ICAgc3RyaW5nICAgICAgICAgIGBqc29uOiJhcHAiYAoJVGVtcGxhdGVTSEEyNTYgc3RyaW5nICAgICAgICAgIGBqc29uOiJ0ZW1w'
        'bGF0ZVNIQTI1NiJgCglDb3VudHMgICAgICAgICBtYXBbc3RyaW5nXWludCAgYGpzb246ImNvdW50cyJgCglDaGVja3MgICAgICAg'
        'ICBtYXBbc3RyaW5nXWJvb2wgYGpzb246ImNoZWNrcyJgCn0KdHlwZSBwZWVyIHN0cnVjdCB7CgltdSAgICAgICAgc3luYy5NdXRl'
        'eAoJZmlsZXMgICAgIGh0dHAuSGFuZGxlcgoJdGVtcGxhdGVzIG1hcFtzdHJpbmddW11ieXRlCglyZWxlYXNlICAgY2hhbiBzdHJ1'
        'Y3R7fQoJc3RhdGUgICAgIHdpdG5lc3MKfQoKZnVuYyAocCAqcGVlcikgcmVzZXQobW9kZSwgYXBwIHN0cmluZykgewoJcC5yZWxl'
        'YXNlID0gbWFrZShjaGFuIHN0cnVjdHt9KQoJcC5zdGF0ZSA9IHdpdG5lc3N7TW9kZTogbW9kZSwgQXBwOiBhcHAsCgkJVGVtcGxh'
        'dGVTSEEyNTY6IGZtdC5TcHJpbnRmKCIleCIsIHNoYTI1Ni5TdW0yNTYocC50ZW1wbGF0ZXNbYXBwXSkpLAoJCUNvdW50czogbWFw'
        'W3N0cmluZ11pbnR7InJlcXVlc3RzIjogMCwgImFjdGl2ZSI6IDAsICJob2xkcyI6IDAsCgkJCSJjYW5jZWxlZCI6IDAsICJjb21w'
        'bGV0ZWQiOiAwLCAiZmFpbHVyZXMiOiAwLCAiZXhwaXJlZCI6IDAsICJ0ZW1wbGF0ZUJ5dGVzIjogbGVuKHAudGVtcGxhdGVzW2Fw'
        'cF0pfSwKCQlDaGVja3M6IG1hcFtzdHJpbmddYm9vbHsiY29va2llU2VlbiI6IGZhbHNlLCAiYXV0aG9yaXphdGlvblNlZW4iOiBm'
        'YWxzZSwgImJvZHlTZWVuIjogZmFsc2UsCgkJCSJxdWVyeVNlZW4iOiBmYWxzZSwgImJvZHlQcmVmaXhTZW50IjogZmFsc2V9fQp9'
        'CmZ1bmMgKHAgKnBlZXIpIHNuYXBzaG90KCkgd2l0bmVzcyB7CglwLm11LkxvY2soKQoJZGVmZXIgcC5tdS5VbmxvY2soKQoJY29w'
        'eSA6PSBwLnN0YXRlCgljb3B5LkNvdW50cyA9IG1hcHMuQ2xvbmUoY29weS5Db3VudHMpCgljb3B5LkNoZWNrcyA9IG1hcHMuQ2xv'
        'bmUoY29weS5DaGVja3MpCglyZXR1cm4gY29weQp9CmZ1bmMgKHAgKnBlZXIpIGNvbnRyb2xsZXIodyBodHRwLlJlc3BvbnNlV3Jp'
        'dGVyLCByICpodHRwLlJlcXVlc3QpIHsKCWlmIHIuTWV0aG9kICE9IGh0dHAuTWV0aG9kUG9zdCB8fCByLkhlYWRlci5HZXQoIlgt'
        'UTQ3LUNvbnRyb2wiKSAhPSAiMSIgewoJCWh0dHAuRXJyb3IodywgImNvbnRyb2wgcmVqZWN0ZWQiLCBodHRwLlN0YXR1c0JhZFJl'
        'cXVlc3QpCgkJcmV0dXJuCgl9CglyLkJvZHkgPSBodHRwLk1heEJ5dGVzUmVhZGVyKHcsIHIuQm9keSwgNTEyKQoJZGVjb2RlciA6'
        'PSBqc29uLk5ld0RlY29kZXIoci5Cb2R5KQoJZGVjb2Rlci5EaXNhbGxvd1Vua25vd25GaWVsZHMoKQoJdmFyIG5leHQgY29udHJv'
        'bAoJaWYgZGVjb2Rlci5EZWNvZGUoJm5leHQpICE9IG5pbCB8fCBkZWNvZGVyLkRlY29kZSgmc3RydWN0e317fSkgIT0gaW8uRU9G'
        'IHx8CgkJIW1vZGVzW25leHQuTW9kZV0gfHwgIWFwcHNbbmV4dC5BcHBdIHsKCQlodHRwLkVycm9yKHcsICJjb250cm9sIHJlamVj'
        'dGVkIiwgaHR0cC5TdGF0dXNCYWRSZXF1ZXN0KQoJCXJldHVybgoJfQoJcC5tdS5Mb2NrKCkKCWRlZmVyIHAubXUuVW5sb2NrKCkK'
        'CXN3aXRjaCBuZXh0Lk9wZXJhdGlvbiB7CgljYXNlICJzdGFydCI6CgkJaWYgcC5zdGF0ZS5Db3VudHNbImFjdGl2ZSJdICE9IDAg'
        'ewoJCQlodHRwLkVycm9yKHcsICJjb250cm9sIGJ1c3kiLCBodHRwLlN0YXR1c0NvbmZsaWN0KQoJCQlyZXR1cm4KCQl9CgkJcC5y'
        'ZXNldChuZXh0Lk1vZGUsIG5leHQuQXBwKQoJY2FzZSAicmVjb3ZlciI6CgkJcC5zdGF0ZS5Nb2RlID0gInZhbGlkIgoJY2FzZSAi'
        'cmVsZWFzZSI6CgkJc2VsZWN0IHsKCQljYXNlIDwtcC5yZWxlYXNlOgoJCWRlZmF1bHQ6CgkJCWNsb3NlKHAucmVsZWFzZSkKCQl9'
        'CglkZWZhdWx0OgoJCWh0dHAuRXJyb3IodywgImNvbnRyb2wgcmVqZWN0ZWQiLCBodHRwLlN0YXR1c0JhZFJlcXVlc3QpCgkJcmV0'
        'dXJuCgl9Cgl3LldyaXRlSGVhZGVyKGh0dHAuU3RhdHVzTm9Db250ZW50KQp9CmZ1bmMgZmFpbEV2ZXJ5KG1vZGUgc3RyaW5nKSBi'
        'b29sIHsKCXJldHVybiBtb2RlID09ICJsb3NzIiB8fCBtb2RlID09ICJodHRwX2Vycm9yIgp9CmZ1bmMgKHAgKnBlZXIpIGJlZ2lu'
        'KHIgKmh0dHAuUmVxdWVzdCwgYXBwIHN0cmluZykgKHN0cmluZywgPC1jaGFuIHN0cnVjdHt9KSB7CglwLm11LkxvY2soKQoJZGVm'
        'ZXIgcC5tdS5VbmxvY2soKQoJcC5zdGF0ZS5Db3VudHNbInJlcXVlc3RzIl0rKwoJcC5zdGF0ZS5Db3VudHNbImFjdGl2ZSJdKysK'
        'CXAuc3RhdGUuQ2hlY2tzWyJjb29raWVTZWVuIl0gPSBwLnN0YXRlLkNoZWNrc1siY29va2llU2VlbiJdIHx8IHIuSGVhZGVyLkdl'
        'dCgiQ29va2llIikgIT0gIiIKCXAuc3RhdGUuQ2hlY2tzWyJhdXRob3JpemF0aW9uU2VlbiJdID0gcC5zdGF0ZS5DaGVja3NbImF1'
        'dGhvcml6YXRpb25TZWVuIl0gfHwgci5IZWFkZXIuR2V0KCJBdXRob3JpemF0aW9uIikgIT0gIiIKCXAuc3RhdGUuQ2hlY2tzWyJi'
        'b2R5U2VlbiJdID0gcC5zdGF0ZS5DaGVja3NbImJvZHlTZWVuIl0gfHwgci5Db250ZW50TGVuZ3RoID4gMCB8fCBsZW4oci5UcmFu'
        'c2ZlckVuY29kaW5nKSA+IDAKCXAuc3RhdGUuQ2hlY2tzWyJxdWVyeVNlZW4iXSA9IHAuc3RhdGUuQ2hlY2tzWyJxdWVyeVNlZW4i'
        'XSB8fCByLlVSTC5SYXdRdWVyeSAhPSAiIgoJbW9kZSA6PSBwLnN0YXRlLk1vZGUKCWlmIGFwcCAhPSBwLnN0YXRlLkFwcCB8fCBw'
        'LnN0YXRlLkNvdW50c1sicmVxdWVzdHMiXSA+IDEgJiYgIWZhaWxFdmVyeShtb2RlKSB7CgkJbW9kZSA9ICJ2YWxpZCIKCX0KCXJl'
        'dHVybiBtb2RlLCBwLnJlbGVhc2UKfQpmdW5jIChwICpwZWVyKSBmaW5pc2gob3V0Y29tZSBzdHJpbmcpIHsKCXAubXUuTG9jaygp'
        'CglkZWZlciBwLm11LlVubG9jaygpCglwLnN0YXRlLkNvdW50c1siYWN0aXZlIl0tLQoJa2V5IDo9IG1hcFtzdHJpbmddc3RyaW5n'
        'eyJjb21wbGV0ZSI6ICJjb21wbGV0ZWQiLCAiY2FuY2VsZWQiOiAiY2FuY2VsZWQiLCAiZXhwaXJlZCI6ICJleHBpcmVkIn1bb3V0'
        'Y29tZV0KCWlmIGtleSA9PSAiIiB7CgkJa2V5ID0gImZhaWx1cmVzIgoJfQoJcC5zdGF0ZS5Db3VudHNba2V5XSsrCn0KZnVuYyAo'
        'cCAqcGVlcikgaG9sZChyICpodHRwLlJlcXVlc3QsIHJlbGVhc2UgPC1jaGFuIHN0cnVjdHt9KSBzdHJpbmcgewoJcC5tdS5Mb2Nr'
        'KCkKCXAuc3RhdGUuQ291bnRzWyJob2xkcyJdKysKCXAubXUuVW5sb2NrKCkKCXRpbWVyIDo9IHRpbWUuTmV3VGltZXIoaG9sZExp'
        'bWl0KQoJZGVmZXIgdGltZXIuU3RvcCgpCglzZWxlY3QgewoJY2FzZSA8LXIuQ29udGV4dCgpLkRvbmUoKToKCQlyZXR1cm4gImNh'
        'bmNlbGVkIgoJY2FzZSA8LXJlbGVhc2U6CgkJcmV0dXJuICJyZWxlYXNlZCIKCWNhc2UgPC10aW1lci5DOgoJCXJldHVybiAiZXhw'
        'aXJlZCIKCX0KfQpmdW5jIChwICpwZWVyKSBzZW5kUHJlZml4KHcgaHR0cC5SZXNwb25zZVdyaXRlciwgZGF0YSBbXWJ5dGUpIGJv'
        'b2wgewoJdy5IZWFkZXIoKS5TZXQoIkNvbnRlbnQtTGVuZ3RoIiwgZm10LlNwcmludChsZW4oZGF0YSkpKQoJXywgZXJyIDo9IHcu'
        'V3JpdGUoZGF0YVs6MzJdKQoJaWYgZXJyID09IG5pbCB7CgkJZXJyID0gaHR0cC5OZXdSZXNwb25zZUNvbnRyb2xsZXIodykuRmx1'
        'c2goKQoJfQoJaWYgZXJyICE9IG5pbCB7CgkJcmV0dXJuIGZhbHNlCgl9CglwLm11LkxvY2soKQoJcC5zdGF0ZS5DaGVja3NbImJv'
        'ZHlQcmVmaXhTZW50Il0gPSB0cnVlCglwLm11LlVubG9jaygpCglyZXR1cm4gdHJ1ZQp9CmZ1bmMgKHAgKnBlZXIpIHRlbXBsYXRl'
        'KHcgaHR0cC5SZXNwb25zZVdyaXRlciwgciAqaHR0cC5SZXF1ZXN0LCBhcHAgc3RyaW5nKSB7Cgltb2RlLCByZWxlYXNlIDo9IHAu'
        'YmVnaW4ociwgYXBwKQoJb3V0Y29tZSA6PSAiZmFpbGVkIgoJZGVmZXIgZnVuYygpIHsgcC5maW5pc2gob3V0Y29tZSkgfSgpCglk'
        'YXRhIDo9IHAudGVtcGxhdGVzW2FwcF0KCXcuSGVhZGVyKCkuU2V0KCJDb250ZW50LVR5cGUiLCAiYXBwbGljYXRpb24veWFtbCIp'
        'Cglzd2l0Y2ggbW9kZSB7CgljYXNlICJsb3NzIjoKCQlwYW5pYyhodHRwLkVyckFib3J0SGFuZGxlcikKCWNhc2UgImh0dHBfZXJy'
        'b3IiOgoJCWh0dHAuRXJyb3IodywgInRlbXBsYXRlIHVuYXZhaWxhYmxlIiwgaHR0cC5TdGF0dXNTZXJ2aWNlVW5hdmFpbGFibGUp'
        'CgkJcmV0dXJuCgljYXNlICJib2R5IiwgImxhdGVfYm9keSI6CgkJaWYgIXAuc2VuZFByZWZpeCh3LCBkYXRhKSB7CgkJCXJldHVy'
        'bgoJCX0KCX0KCWlmIG1vZGUgIT0gInZhbGlkIiB7CgkJb3V0Y29tZSA9IHAuaG9sZChyLCByZWxlYXNlKQoJCWlmIG91dGNvbWUg'
        'IT0gInJlbGVhc2VkIiB7CgkJCXJldHVybgoJCX0KCX0KCWlmIG1vZGUgPT0gImJvZHkiIHx8IG1vZGUgPT0gImxhdGVfYm9keSIg'
        'ewoJCWRhdGEgPSBkYXRhWzMyOl0KCX0KCWlmIF8sIGVyciA6PSB3LldyaXRlKGRhdGEpOyBlcnIgPT0gbmlsIHsKCQlvdXRjb21l'
        'ID0gImNvbXBsZXRlIgoJfQp9CmZ1bmMgKHAgKnBlZXIpIHNlcnZlKHcgaHR0cC5SZXNwb25zZVdyaXRlciwgciAqaHR0cC5SZXF1'
        'ZXN0KSB7Cgl3LkhlYWRlcigpLlNldCgiQ2FjaGUtQ29udHJvbCIsICJuby1zdG9yZSIpCgl3LkhlYWRlcigpLlNldCgiWC1Db250'
        'ZW50LVR5cGUtT3B0aW9ucyIsICJub3NuaWZmIikKCWlmIHIuVVJMLlBhdGggPT0gIi9fX3E0N19fL2NvbnRyb2wiIHsKCQlwLmNv'
        'bnRyb2xsZXIodywgcikKCQlyZXR1cm4KCX0KCWlmIHIuTWV0aG9kICE9IGh0dHAuTWV0aG9kR2V0IHsKCQlodHRwLkVycm9yKHcs'
        'ICJtZXRob2QgcmVqZWN0ZWQiLCBodHRwLlN0YXR1c01ldGhvZE5vdEFsbG93ZWQpCgkJcmV0dXJuCgl9CglpZiByLlVSTC5QYXRo'
        'ID09ICIvX19xNDdfXy93aXRuZXNzIiB7CgkJdy5IZWFkZXIoKS5TZXQoIkNvbnRlbnQtVHlwZSIsICJhcHBsaWNhdGlvbi9qc29u'
        'IikKCQlpZiBlcnIgOj0ganNvbi5OZXdFbmNvZGVyKHcpLkVuY29kZShwLnNuYXBzaG90KCkpOyBlcnIgIT0gbmlsIHsKCQkJcmV0'
        'dXJuCgkJfQoJCXJldHVybgoJfQoJZm9yIGFwcCA6PSByYW5nZSBhcHBzIHsKCQlpZiByLlVSTC5QYXRoID09ICIvYXNzZXRzL2lu'
        'c3RhbGwvIithcHArIi55YW1sIiB7CgkJCXAudGVtcGxhdGUodywgciwgYXBwKQoJCQlyZXR1cm4KCQl9Cgl9CglwLmZpbGVzLlNl'
        'cnZlSFRUUCh3LCByKQp9Cg=='
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
