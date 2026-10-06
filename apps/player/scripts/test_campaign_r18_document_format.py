"""Pure format/history/protocol rejection controls; no tool or file runs."""
import base64
import copy
import hashlib
import json
import unittest
from unittest.mock import Mock, patch
import subprocess

from campaign_r18_document_format import (
    GO_FILES, format_record, formatter_command, history_kind, admit_format,
)
from campaign_r18_document_format_process import PacketProjection, settle_child

SOURCE = base64.b64decode("cGFja2FnZSBzZXJ2ZXJfdGVzdAoKaW1wb3J0ICgKCSJuZXQvaHR0cCIKCSJzdHJpbmdzIgoJInRlc3RpbmciCikKCmZ1bmMgVGVzdEhvbWVBc3Npc3RhbnREb2N1bWVudFRhcmdldHNSZW1haW5JbmRlcGVuZGVudFRocm91Z2hQdWJsaWNBcHAodCAqdGVzdGluZy5UKSB7CgljbGllbnQgOj0gbmV3RG9jdW1lbnRUYXJnZXRDbGllbnQodCkKCWZpcnN0LCBzZWNvbmQgOj0gY2xpZW50LmNsYWltKHQsICIiKSwgY2xpZW50LmNsYWltKHQsICIiKQoJaWYgZmlyc3QuSUQgPT0gc2Vjb25kLklEIHx8IGZpcnN0LkNsYWltID09IHNlY29uZC5DbGFpbSB7CgkJdC5GYXRhbCgiUjE4IHNlcGFyYXRlIGRvY3VtZW50IGNsYWltcyBzaGFyZWQgdGFyZ2V0IGlkZW50aXR5IG9yIGF1dGhvcml0eSIpCgl9CgljbGllbnQucG9sbCh0LCBmaXJzdC5JRCwgW11zdHJpbmd7Zmlyc3QuQ2xhaW19LCBmYWxzZSkKCWNsaWVudC5wb2xsKHQsIHNlY29uZC5JRCwgW11zdHJpbmd7c2Vjb25kLkNsYWltfSwgZmFsc2UpCglzbmFwc2hvdCA6PSBjbGllbnQuc25hcHNob3QodCkKCWlmIGxlbihzbmFwc2hvdCkgIT0gMiB8fCBzbmFwc2hvdFtmaXJzdC5JRF0uUG9zaXRpb24gIT0gMSB8fCBzbmFwc2hvdFtzZWNvbmQuSURdLlBvc2l0aW9uICE9IDEgewoJCXQuRmF0YWwoIlIxOCBpbmRlcGVuZGVudCBkb2N1bWVudCBzdGF0ZXMgZGlkIG5vdCBwdWJsaXNoIHR3byBleGFjdCB0YXJnZXRzIikKCX0KCWNsaWVudC5xdWV1ZVNlZWsodCwgZmlyc3QuSUQpCgljbGllbnQucG9sbCh0LCBzZWNvbmQuSUQsIFtdc3RyaW5ne3NlY29uZC5DbGFpbX0sIGZhbHNlKQoJY2xpZW50LnBvbGwodCwgZmlyc3QuSUQsIFtdc3RyaW5ne2ZpcnN0LkNsYWltfSwgdHJ1ZSkKCWNsaWVudC5wb2xsKHQsIGZpcnN0LklELCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0sIGZhbHNlKQoJY2xpZW50LnBvbGwodCwgc2Vjb25kLklELCBbXXN0cmluZ3tzZWNvbmQuQ2xhaW19LCBmYWxzZSkKfQoKZnVuYyBUZXN0SG9tZUFzc2lzdGFudERvY3VtZW50U2libGluZ0Nhbm5vdE92ZXJ3cml0ZVJlbGVhc2VPckRyYWluKHQgKnRlc3RpbmcuVCkgewoJY2xpZW50IDo9IG5ld0RvY3VtZW50VGFyZ2V0Q2xpZW50KHQpCgljbGFpbSA6PSBjbGllbnQuY2xhaW0odCwgInIxOC1zaWJsaW5nLWZpeHR1cmUiKQoJcGF0aCA6PSBkb2N1bWVudFRhcmdldHNQYXRoICsgIi8iICsgY2xhaW0uSUQKCWNsaWVudC5wb2xsKHQsIGNsYWltLklELCBbXXN0cmluZ3tjbGFpbS5DbGFpbX0sIGZhbHNlKQoJY2xpZW50LnF1ZXVlU2Vlayh0LCBjbGFpbS5JRCkKCWJlZm9yZSA6PSBjbGllbnQuc25hcHNob3QodCkKCWNvbmZsaWN0IDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgZG9jdW1lbnRUYXJnZXRzUGF0aCsiL2NsYWltcyIsIGB7ImlkIjoicjE4LXNpYmxpbmctZml4dHVyZSJ9YCwgbmlsKQoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQsIGNvbmZsaWN0LCBodHRwLlN0YXR1c0NvbmZsaWN0KQoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U25hcHNob3QodCwgY2xpZW50LnNuYXBzaG90KHQpLCBiZWZvcmUpCgloZWFkZXJzIDo9IFtdW11zdHJpbmd7bmlsLCB7Indyb25nLWRvY3VtZW50LWNsYWltLTAwMDAwMDAwMCJ9LCB7Y2xhaW0uQ2xhaW0sIGNsYWltLkNsYWltfSwge3N0cmluZ3MuUmVwZWF0KCJhIiwgNjUpfX0KCWZvciBfLCB2YWx1ZXMgOj0gcmFuZ2UgaGVhZGVycyB7CgkJcmVqZWN0ZWQgOj0gY2xpZW50LmNhbGwodCwgaHR0cC5NZXRob2RQdXQsIHBhdGgsIGRvY3VtZW50VGFyZ2V0Qm9keSg5KSwgdmFsdWVzKQoJCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCByZWplY3RlZCwgaHR0cC5TdGF0dXNGb3JiaWRkZW4pCgkJcmVxdWlyZURvY3VtZW50VGFyZ2V0U25hcHNob3QodCwgY2xpZW50LnNuYXBzaG90KHQpLCBiZWZvcmUpCgkJcmVsZWFzZWQgOj0gY2xpZW50LmNhbGwodCwgaHR0cC5NZXRob2RQb3N0LCBwYXRoKyIvcmVsZWFzZSIsIGB7fWAsIHZhbHVlcykKCQlyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVsZWFzZWQsIGh0dHAuU3RhdHVzRm9yYmlkZGVuKQoJCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJfQoJY2xpZW50LnBvbGwodCwgY2xhaW0uSUQsIFtdc3RyaW5ne2NsYWltLkNsYWltfSwgdHJ1ZSkKCWNsaWVudC5wb2xsKHQsIGNsYWltLklELCBbXXN0cmluZ3tjbGFpbS5DbGFpbX0sIGZhbHNlKQp9CgpmdW5jIFRlc3RIb21lQXNzaXN0YW50RG9jdW1lbnRSZWxlYXNlS2VlcHNDYW5kaWRhdGVBbmRSZWplY3RzUmV0aXJlZENsYWltKHQgKnRlc3RpbmcuVCkgewoJY2xpZW50IDo9IG5ld0RvY3VtZW50VGFyZ2V0Q2xpZW50KHQpCglmaXJzdCA6PSBjbGllbnQuY2xhaW0odCwgInIxOC1yZWxvYWQtZml4dHVyZSIpCglwYXRoIDo9IGRvY3VtZW50VGFyZ2V0c1BhdGggKyAiLyIgKyBmaXJzdC5JRAoJY2xpZW50LnBvbGwodCwgZmlyc3QuSUQsIFtdc3RyaW5ne2ZpcnN0LkNsYWltfSwgZmFsc2UpCglyZWxlYXNlIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgcGF0aCsiL3JlbGVhc2UiLCBge31gLCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0pCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVsZWFzZSwgaHR0cC5TdGF0dXNOb0NvbnRlbnQpCglpZiBsZW4oY2xpZW50LnNuYXBzaG90KHQpKSAhPSAwIHsKCQl0LkZhdGFsKCJSMTggc3VjY2Vzc2Z1bCBkb2N1bWVudCByZWxlYXNlIGxlZnQgYSBwdWJsaXNoZWQgdGFyZ2V0IikKCX0KCXNlY29uZCA6PSBjbGllbnQuY2xhaW0odCwgZmlyc3QuSUQpCglpZiBzZWNvbmQuSUQgIT0gZmlyc3QuSUQgfHwgc2Vjb25kLkNsYWltID09IGZpcnN0LkNsYWltIHsKCQl0LkZhdGFsKCJSMTggcmVjbGFpbSBjaGFuZ2VkIGF1dG9tYXRpb24gaWRlbnRpdHkgb3IgcmV1c2VkIHJldGlyZWQgYXV0aG9yaXR5IikKCX0KCWNsaWVudC5wb2xsKHQsIHNlY29uZC5JRCwgW11zdHJpbmd7c2Vjb25kLkNsYWltfSwgZmFsc2UpCgljbGllbnQucXVldWVTZWVrKHQsIHNlY29uZC5JRCkKCWJlZm9yZSA6PSBjbGllbnQuc25hcHNob3QodCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFB1dCwgcGF0aCwgZG9jdW1lbnRUYXJnZXRCb2R5KDkpLCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0pLCBodHRwLlN0YXR1c0ZvcmJpZGRlbikKCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQsIGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgcGF0aCsiL3JlbGVhc2UiLCBge31gLCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0pLCBodHRwLlN0YXR1c0ZvcmJpZGRlbikKCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJY2xpZW50LnBvbGwodCwgc2Vjb25kLklELCBbXXN0cmluZ3tzZWNvbmQuQ2xhaW19LCB0cnVlKQoJY2xpZW50LnBvbGwodCwgc2Vjb25kLklELCBbXXN0cmluZ3tzZWNvbmQuQ2xhaW19LCBmYWxzZSkKfQoKZnVuYyBUZXN0SG9tZUFzc2lzdGFudERvY3VtZW50TmF0aXZlU3RyaWN0Q29udHJhY3RSZW1haW5zVW5jaGFuZ2VkKHQgKnRlc3RpbmcuVCkgewoJY2xpZW50IDo9IG5ld0RvY3VtZW50VGFyZ2V0Q2xpZW50KHQpCgljb25zdCBpZCA9ICJyMTgtbmF0aXZlLWZpeHR1cmUiCglwYXRoIDo9IGRvY3VtZW50VGFyZ2V0c1BhdGggKyAiLyIgKyBpZAoJY2xpZW50LnBvbGwodCwgaWQsIG5pbCwgZmFsc2UpCgljbGllbnQucXVldWVTZWVrKHQsIGlkKQoJYmVmb3JlIDo9IGNsaWVudC5zbmFwc2hvdCh0KQoJdW5rbm93blN0YXRlIDo9IHN0cmluZ3MuVHJpbVN1ZmZpeChkb2N1bWVudFRhcmdldEJvZHkoOSksICJ9IikgKyBgLCJjbGFpbSI6InVua25vd24tZmllbGQifWAKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFB1dCwgcGF0aCwgdW5rbm93blN0YXRlLCBuaWwpLCBodHRwLlN0YXR1c0JhZFJlcXVlc3QpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTbmFwc2hvdCh0LCBjbGllbnQuc25hcHNob3QodCksIGJlZm9yZSkKCXVua25vd25Db21tYW5kIDo9IGB7ImNvbW1hbmQiOiJzZWVrIiwicG9zaXRpb24iOjksImNsYWltIjoidW5rbm93bi1maWVsZCJ9YAoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQsIGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgcGF0aCsiL2NvbW1hbmRzIiwgdW5rbm93bkNvbW1hbmQsIG5pbCksIGh0dHAuU3RhdHVzQmFkUmVxdWVzdCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJY2xpZW50LnBvbGwodCwgaWQsIG5pbCwgdHJ1ZSkKCWNsaWVudC5wb2xsKHQsIGlkLCBuaWwsIGZhbHNlKQp9Cg==", validate=True)

HELPER = base64.b64decode("cGFja2FnZSBzZXJ2ZXJfdGVzdAoKaW1wb3J0ICgKCSJjb250ZXh0IgoJImVuY29kaW5nL2pzb24iCgkiaW8iCgkibmV0L2h0dHAiCgkibmV0L2h0dHAvaHR0cHRlc3QiCgkicmVmbGVjdCIKCSJyZWdleHAiCgkic3RyaW5ncyIKCSJ0ZXN0aW5nIgoJInRpbWUiCikKCi8vIFRoZXNlIHRlc3RzIGV4ZXJjaXNlIHRoZSByZWFsIGF1dGhlbnRpY2F0ZWQgYXBwbGljYXRpb24gaGFuZGxlciBhbmQgcHVibGljCi8vIHJvdXRlcy4gVGhleSBkbyBub3Qgc2ltdWxhdGUgYnJvd3NlciBzdG9yYWdlLCBjbGFpbSBtZWRpYSBwbGF5YmFjaywgb3IgYnlwYXNzCi8vIENTUkYuIFRoZSBjYW5vbmljYWwgQVBJIGZpeHR1cmUgc3VwcGxpZXMgZGlzcG9zYWJsZSBPd25lciBlbnJvbGxtZW50Lgpjb25zdCBkb2N1bWVudFRhcmdldHNQYXRoID0gIi9hcGkvdjEvaG9tZS1hc3Npc3RhbnQvcGxheWVycyIKCnZhciBkb2N1bWVudFRhcmdldElEID0gcmVnZXhwLk11c3RDb21waWxlKGBeW0EtWmEtejAtOV8tXXsxLDY0fSRgKQoKdHlwZSBkb2N1bWVudFRhcmdldENsaWVudCBzdHJ1Y3QgewoJaGFuZGxlciBodHRwLkhhbmRsZXIKCWNvb2tpZSAgKmh0dHAuQ29va2llCgljc3JmICAgIHN0cmluZwp9Cgp0eXBlIGRvY3VtZW50VGFyZ2V0Q2FwdHVyZSBzdHJ1Y3QgewoJKmh0dHB0ZXN0LlJlc3BvbnNlUmVjb3JkZXIKCW92ZXJmbG93IGJvb2wKfQoKZnVuYyAoY2FwdHVyZSAqZG9jdW1lbnRUYXJnZXRDYXB0dXJlKSBXcml0ZShib2R5IFtdYnl0ZSkgKGludCwgZXJyb3IpIHsKCWlmIGxlbihib2R5KSA+IDI2MjE0NC1jYXB0dXJlLkJvZHkuTGVuKCkgewoJCWNhcHR1cmUub3ZlcmZsb3cgPSB0cnVlCgkJcmV0dXJuIDAsIGlvLkVyclNob3J0V3JpdGUKCX0KCXJldHVybiBjYXB0dXJlLlJlc3BvbnNlUmVjb3JkZXIuV3JpdGUoYm9keSkKfQoKZnVuYyAoY2FwdHVyZSAqZG9jdW1lbnRUYXJnZXRDYXB0dXJlKSBXcml0ZVN0cmluZyhib2R5IHN0cmluZykgKGludCwgZXJyb3IpIHsKCXJldHVybiBjYXB0dXJlLldyaXRlKFtdYnl0ZShib2R5KSkKfQoKdHlwZSBkb2N1bWVudFRhcmdldENsYWltIHN0cnVjdCB7CglJRCAgICAgICAgc3RyaW5nIGBqc29uOiJpZCJgCglDbGFpbSAgICAgc3RyaW5nIGBqc29uOiJjbGFpbSJgCglFeHBpcmVzSW4gaW50ICAgIGBqc29uOiJleHBpcmVzSW4iYAp9Cgp0eXBlIGRvY3VtZW50VGFyZ2V0U3RhdGUgc3RydWN0IHsKCUlEICAgICAgIHN0cmluZyAgYGpzb246ImlkImAKCU5hbWUgICAgIHN0cmluZyAgYGpzb246Im5hbWUiYAoJU3RhdGUgICAgc3RyaW5nICBganNvbjoic3RhdGUiYAoJVGl0bGUgICAgc3RyaW5nICBganNvbjoidGl0bGUiYAoJSXRlbUlEICAgc3RyaW5nICBganNvbjoiaXRlbUlkImAKCVBvc2l0aW9uIGZsb2F0NjQgYGpzb246InBvc2l0aW9uImAKCUR1cmF0aW9uIGZsb2F0NjQgYGpzb246ImR1cmF0aW9uImAKCVZvbHVtZSAgIGZsb2F0NjQgYGpzb246InZvbHVtZSJgCglNdXRlZCAgICBib29sICAgIGBqc29uOiJtdXRlZCJgCn0KCmZ1bmMgbmV3RG9jdW1lbnRUYXJnZXRDbGllbnQodCAqdGVzdGluZy5UKSAqZG9jdW1lbnRUYXJnZXRDbGllbnQgewoJdC5IZWxwZXIoKQoJaGFuZGxlciwgb3duZXIgOj0gYXBpU2VydmVyKHQpCgllbmFibGVkIDo9IGFwaUNhbGwodCwgaGFuZGxlciwgb3duZXIsIGh0dHAuTWV0aG9kUHV0LCAiL2FwaS92MS9zZXR0aW5ncy9ob21lLWFzc2lzdGFudCIsIG1hcFtzdHJpbmddYW55eyJlbmFibGVkIjogdHJ1ZX0pCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgZW5hYmxlZCwgaHR0cC5TdGF0dXNPSykKCWNsaWVudCA6PSAmZG9jdW1lbnRUYXJnZXRDbGllbnR7aGFuZGxlcjogaGFuZGxlciwgY29va2llOiAmaHR0cC5Db29raWV7CgkJTmFtZTogIl9fSG9zdC1raW5vc2FpbF9zZXNzaW9uIiwgVmFsdWU6IG93bmVyLCBTZWN1cmU6IHRydWUsCgkJSHR0cE9ubHk6IHRydWUsIFBhdGg6ICIvIiwgU2FtZVNpdGU6IGh0dHAuU2FtZVNpdGVTdHJpY3RNb2RlLAoJfX0KCXNldHRpbmdzIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kR2V0LCAiL3NldHRpbmdzIiwgIiIsIG5pbCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCBzZXR0aW5ncywgaHR0cC5TdGF0dXNPSykKCW1hdGNoIDo9IHJlZ2V4cC5NdXN0Q29tcGlsZShgPG1ldGEgbmFtZT0ia2lub3NhaWwtY3NyZiIgY29udGVudD0iKFteIl0rKSI+YCkuRmluZFN0cmluZ1N1Ym1hdGNoKHNldHRpbmdzLkJvZHkuU3RyaW5nKCkpCglpZiBsZW4obWF0Y2gpICE9IDIgfHwgbGVuKG1hdGNoWzFdKSA9PSAwIHx8IGxlbihtYXRjaFsxXSkgPiAyNTYgewoJCXQuRmF0YWwoIlIxOCBhdXRoZW50aWNhdGVkIFNldHRpbmdzIENTUkYgcHJlcmVxdWlzaXRlIHdhcyBub3QgbWV0IikKCX0KCWNsaWVudC5jc3JmID0gbWF0Y2hbMV0KCXZhciBtZSBzdHJ1Y3QgewoJCVZpZXdlciBzdHJ1Y3QgewoJCQlJRCAgICBzdHJpbmcgYGpzb246ImlkImAKCQkJT3duZXIgYm9vbCAgIGBqc29uOiJvd25lciJgCgkJfSBganNvbjoidmlld2VyImAKCX0KCXJlc3BvbnNlIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kR2V0LCAiL2FwaS92MS9tZSIsICIiLCBuaWwpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVzcG9uc2UsIGh0dHAuU3RhdHVzT0spCglyZWFkRG9jdW1lbnRUYXJnZXRKU09OKHQsIHJlc3BvbnNlLCAmbWUpCglpZiBtZS5WaWV3ZXIuSUQgPT0gIiIgfHwgIW1lLlZpZXdlci5Pd25lciB7CgkJdC5GYXRhbCgiUjE4IHB1YmxpYyBjdXJyZW50LU93bmVyIGlkZW50aXR5IHByZXJlcXVpc2l0ZSB3YXMgbm90IG1ldCIpCgl9CglyZXR1cm4gY2xpZW50Cn0KCmZ1bmMgKGNsaWVudCAqZG9jdW1lbnRUYXJnZXRDbGllbnQpIGNhbGwodCAqdGVzdGluZy5ULCBtZXRob2QsIHBhdGgsIGJvZHkgc3RyaW5nLCBjbGFpbXMgW11zdHJpbmcpICpodHRwdGVzdC5SZXNwb25zZVJlY29yZGVyIHsKCXQuSGVscGVyKCkKCWlmIGxlbihib2R5KSA+IDQwOTYgewoJCXQuRmF0YWwoIlIxOCBhdXRob3JlZCByZXF1ZXN0IGV4Y2VlZGVkIGl0cyBib2R5IGJvdW5kIikKCX0KCWN0eCwgY2FuY2VsIDo9IGNvbnRleHQuV2l0aFRpbWVvdXQodC5Db250ZXh0KCksIDMqdGltZS5TZWNvbmQpCglkZWZlciBjYW5jZWwoKQoJcmVxdWVzdCA6PSBodHRwdGVzdC5OZXdSZXF1ZXN0V2l0aENvbnRleHQoY3R4LCBtZXRob2QsICJodHRwczovL2tpbm9zYWlsLnRlc3QiK3BhdGgsIHN0cmluZ3MuTmV3UmVhZGVyKGJvZHkpKQoJZGVmZXIgZnVuYygpIHsgXyA9IHJlcXVlc3QuQm9keS5DbG9zZSgpIH0oKQoJcmVxdWVzdC5IZWFkZXIuU2V0KCJVc2VyLUFnZW50IiwgIktpbm9zYWlsIGRvY3VtZW50IHRhcmdldCByZWdyZXNzaW9uIikKCXJlcXVlc3QuSGVhZGVyLlNldCgiT3JpZ2luIiwgImh0dHBzOi8va2lub3NhaWwudGVzdCIpCglyZXF1ZXN0LkhlYWRlci5TZXQoIkNvbnRlbnQtVHlwZSIsICJhcHBsaWNhdGlvbi9qc29uIikKCXJlcXVlc3QuQWRkQ29va2llKGNsaWVudC5jb29raWUpCglpZiBjbGllbnQuY3NyZiAhPSAiIiB7CgkJcmVxdWVzdC5IZWFkZXIuU2V0KCJYLUtpbm9zYWlsLUNTUkYiLCBjbGllbnQuY3NyZikKCX0KCWZvciBfLCBjbGFpbSA6PSByYW5nZSBjbGFpbXMgewoJCXJlcXVlc3QuSGVhZGVyLkFkZCgiWC1LaW5vc2FpbC1QbGF5ZXItQ2xhaW0iLCBjbGFpbSkKCX0KCXJlc3BvbnNlIDo9ICZkb2N1bWVudFRhcmdldENhcHR1cmV7UmVzcG9uc2VSZWNvcmRlcjogaHR0cHRlc3QuTmV3UmVjb3JkZXIoKX0KCWNsaWVudC5oYW5kbGVyLlNlcnZlSFRUUChyZXNwb25zZSwgcmVxdWVzdCkKCWlmIGN0eC5FcnIoKSAhPSBuaWwgewoJCXQuRmF0YWwoIlIxOCBwdWJsaWMgcmVxdWVzdCBkaWQgbm90IHNldHRsZSBpbnNpZGUgaXRzIGRlYWRsaW5lIikKCX0KCWlmIHJlc3BvbnNlLm92ZXJmbG93IHsKCQl0LkZhdGFsKCJSMTggcHVibGljIHJlc3BvbnNlIGV4Y2VlZGVkIGl0cyBjYXB0dXJlIGJvdW5kIikKCX0KCXJldHVybiByZXNwb25zZS5SZXNwb25zZVJlY29yZGVyCn0KCmZ1bmMgcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQgKnRlc3RpbmcuVCwgcmVzcG9uc2UgKmh0dHB0ZXN0LlJlc3BvbnNlUmVjb3JkZXIsIHdhbnQgaW50KSB7Cgl0LkhlbHBlcigpCglpZiByZXNwb25zZS5Db2RlICE9IHdhbnQgewoJCXQuRmF0YWxmKCJSMTggcHVibGljIHJlc3BvbnNlIEhUVFAgJWQsIGV4cGVjdGVkICVkIiwgcmVzcG9uc2UuQ29kZSwgd2FudCkKCX0KfQoKZnVuYyByZWFkRG9jdW1lbnRUYXJnZXRKU09OKHQgKnRlc3RpbmcuVCwgcmVzcG9uc2UgKmh0dHB0ZXN0LlJlc3BvbnNlUmVjb3JkZXIsIHRhcmdldCBhbnkpIHsKCXQuSGVscGVyKCkKCWlmIHJlc3BvbnNlLkJvZHkuTGVuKCkgPiAxNjM4NCB7CgkJdC5GYXRhbCgiUjE4IEpTT04gcmVzcG9uc2UgZXhjZWVkZWQgaXRzIGRlY29kZSBib3VuZCIpCgl9CglpZiBqc29uLlVubWFyc2hhbChyZXNwb25zZS5Cb2R5LkJ5dGVzKCksIHRhcmdldCkgIT0gbmlsIHsKCQl0LkZhdGFsKCJSMTggcHVibGljIHJlc3BvbnNlIHdhcyBub3QgdGhlIGV4cGVjdGVkIEpTT04iKQoJfQp9CgpmdW5jIChjbGllbnQgKmRvY3VtZW50VGFyZ2V0Q2xpZW50KSBjbGFpbSh0ICp0ZXN0aW5nLlQsIGlkIHN0cmluZykgZG9jdW1lbnRUYXJnZXRDbGFpbSB7Cgl0LkhlbHBlcigpCglib2R5LCBlcnIgOj0ganNvbi5NYXJzaGFsKHN0cnVjdCB7CgkJSUQgc3RyaW5nIGBqc29uOiJpZCxvbWl0ZW1wdHkiYAoJfXtpZH0pCglpZiBlcnIgIT0gbmlsIHsKCQl0LkZhdGFsKCJSMTggYXV0aG9yZWQgY2xhaW0gcmVxdWVzdCBjb3VsZCBub3QgYmUgZW5jb2RlZCIpCgl9CglyZXNwb25zZSA6PSBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFBvc3QsIGRvY3VtZW50VGFyZ2V0c1BhdGgrIi9jbGFpbXMiLCBzdHJpbmcoYm9keSksIG5pbCkKCWlmIHJlc3BvbnNlLkNvZGUgIT0gaHR0cC5TdGF0dXNDcmVhdGVkIHsKCQl0LkZhdGFsZigiUjE4IGNsYWltLXJvdXRlIHByZXJlcXVpc2l0ZTogSFRUUCAlZCwgZXhwZWN0ZWQgMjAxIiwgcmVzcG9uc2UuQ29kZSkKCX0KCXZhciBjbGFpbSBkb2N1bWVudFRhcmdldENsYWltCglyZWFkRG9jdW1lbnRUYXJnZXRKU09OKHQsIHJlc3BvbnNlLCAmY2xhaW0pCglpZiAhZG9jdW1lbnRUYXJnZXRJRC5NYXRjaFN0cmluZyhjbGFpbS5JRCkgfHwgIWRvY3VtZW50VGFyZ2V0SUQuTWF0Y2hTdHJpbmcoY2xhaW0uQ2xhaW0pIHsKCQl0LkZhdGFsKCJSMTggY2xhaW0gcmVzcG9uc2UgaWRlbnRpdGllcyB2aW9sYXRlZCB0aGUgcHVibGljIGJvdW5kIikKCX0KCWlmIGxlbihjbGFpbS5DbGFpbSkgPCAyMCB8fCBjbGFpbS5FeHBpcmVzSW4gIT0gMzAgfHwgcmVzcG9uc2UuSGVhZGVyKCkuR2V0KCJDYWNoZS1Db250cm9sIikgIT0gIm5vLXN0b3JlIiB7CgkJdC5GYXRhbCgiUjE4IGNsYWltIHJlc3BvbnNlIG93bmVyc2hpcCwgbGVhc2Ugb3IgY2FjaGUgY29udHJhY3Qgd2FzIGludmFsaWQiKQoJfQoJaWYgaWQgIT0gIiIgJiYgY2xhaW0uSUQgIT0gaWQgewoJCXQuRmF0YWwoIlIxOCBjbGFpbSBjaGFuZ2VkIHRoZSByZXF1ZXN0ZWQgc3RhYmxlIGNhbmRpZGF0ZSIpCgl9CglyZXR1cm4gY2xhaW0KfQoKZnVuYyAoY2xpZW50ICpkb2N1bWVudFRhcmdldENsaWVudCkgc25hcHNob3QodCAqdGVzdGluZy5UKSBtYXBbc3RyaW5nXWRvY3VtZW50VGFyZ2V0U3RhdGUgewoJdC5IZWxwZXIoKQoJcmVzcG9uc2UgOj0gY2xpZW50LmNhbGwodCwgaHR0cC5NZXRob2RHZXQsIGRvY3VtZW50VGFyZ2V0c1BhdGgsICIiLCBuaWwpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVzcG9uc2UsIGh0dHAuU3RhdHVzT0spCgl2YXIgcmVzdWx0IHN0cnVjdCB7CgkJUGxheWVycyBbXWRvY3VtZW50VGFyZ2V0U3RhdGUgYGpzb246InBsYXllcnMiYAoJfQoJcmVhZERvY3VtZW50VGFyZ2V0SlNPTih0LCByZXNwb25zZSwgJnJlc3VsdCkKCWlmIHJlc3VsdC5QbGF5ZXJzID09IG5pbCB7CgkJdC5GYXRhbCgiUjE4IHRhcmdldCBzbmFwc2hvdCBvbWl0dGVkIGl0cyBwdWJsaWMgcGxheWVyIGxpc3QiKQoJfQoJcGxheWVycyA6PSBtYWtlKG1hcFtzdHJpbmddZG9jdW1lbnRUYXJnZXRTdGF0ZSwgbGVuKHJlc3VsdC5QbGF5ZXJzKSkKCWZvciBfLCBwbGF5ZXIgOj0gcmFuZ2UgcmVzdWx0LlBsYXllcnMgewoJCWlmICFkb2N1bWVudFRhcmdldElELk1hdGNoU3RyaW5nKHBsYXllci5JRCkgewoJCQl0LkZhdGFsKCJSMTggcHVibGlzaGVkIHRhcmdldCBpZGVudGl0eSB3YXMgaW52YWxpZCIpCgkJfQoJCWlmIF8sIGR1cGxpY2F0ZSA6PSBwbGF5ZXJzW3BsYXllci5JRF07IGR1cGxpY2F0ZSB7CgkJCXQuRmF0YWwoIlIxOCBwdWJsaXNoZWQgc25hcHNob3QgY29udGFpbmVkIGR1cGxpY2F0ZSB0YXJnZXQgaWRlbnRpdGllcyIpCgkJfQoJCXBsYXllcnNbcGxheWVyLklEXSA9IHBsYXllcgoJfQoJcmV0dXJuIHBsYXllcnMKfQoKZnVuYyByZXF1aXJlRG9jdW1lbnRUYXJnZXRTbmFwc2hvdCh0ICp0ZXN0aW5nLlQsIGdvdCwgd2FudCBtYXBbc3RyaW5nXWRvY3VtZW50VGFyZ2V0U3RhdGUpIHsKCXQuSGVscGVyKCkKCWlmICFyZWZsZWN0LkRlZXBFcXVhbChnb3QsIHdhbnQpIHsKCQl0LkZhdGFsKCJSMTggcmVqZWN0ZWQgcmVxdWVzdCBjaGFuZ2VkIHRoZSBjb21wbGV0ZSBwdWJsaXNoZWQgdGFyZ2V0IHNuYXBzaG90IikKCX0KfQoKZnVuYyAoY2xpZW50ICpkb2N1bWVudFRhcmdldENsaWVudCkgcXVldWVTZWVrKHQgKnRlc3RpbmcuVCwgaWQgc3RyaW5nKSB7Cgl0LkhlbHBlcigpCglyZXNwb25zZSA6PSBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFBvc3QsIGRvY3VtZW50VGFyZ2V0c1BhdGgrIi8iK2lkKyIvY29tbWFuZHMiLCBgeyJjb21tYW5kIjoic2VlayIsInBvc2l0aW9uIjo0fWAsIG5pbCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCByZXNwb25zZSwgaHR0cC5TdGF0dXNBY2NlcHRlZCkKfQoKZnVuYyBkb2N1bWVudFRhcmdldEJvZHkocG9zaXRpb24gaW50KSBzdHJpbmcgewoJaWYgcG9zaXRpb24gPT0gOSB7CgkJcmV0dXJuIGB7Im5hbWUiOiJGaWN0aW9uYWwgZG9jdW1lbnQgdGFyZ2V0Iiwic3RhdGUiOiJwYXVzZWQiLCJwb3NpdGlvbiI6OSwiZHVyYXRpb24iOjEyLCJ2b2x1bWUiOjAuNX1gCgl9CglyZXR1cm4gYHsibmFtZSI6IkZpY3Rpb25hbCBkb2N1bWVudCB0YXJnZXQiLCJzdGF0ZSI6InBhdXNlZCIsInBvc2l0aW9uIjoxLCJkdXJhdGlvbiI6MTIsInZvbHVtZSI6MC41fWAKfQoKZnVuYyAoY2xpZW50ICpkb2N1bWVudFRhcmdldENsaWVudCkgcG9sbCh0ICp0ZXN0aW5nLlQsIGlkIHN0cmluZywgY2xhaW1zIFtdc3RyaW5nLCBzZWVrIGJvb2wpIHsKCXQuSGVscGVyKCkKCXJlc3BvbnNlIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUHV0LCBkb2N1bWVudFRhcmdldHNQYXRoKyIvIitpZCwgZG9jdW1lbnRUYXJnZXRCb2R5KDEpLCBjbGFpbXMpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVzcG9uc2UsIGh0dHAuU3RhdHVzT0spCgl2YXIgcmVzdWx0IHN0cnVjdCB7CgkJQ29tbWFuZCAganNvbi5SYXdNZXNzYWdlIGBqc29uOiJjb21tYW5kImAKCQlQb3NpdGlvbiBmbG9hdDY0ICAgICAgICAgYGpzb246InBvc2l0aW9uImAKCX0KCXJlYWREb2N1bWVudFRhcmdldEpTT04odCwgcmVzcG9uc2UsICZyZXN1bHQpCglpZiAhc2VlayB7CgkJaWYgc3RyaW5nKHJlc3VsdC5Db21tYW5kKSAhPSAibnVsbCIgewoJCQl0LkZhdGFsKCJSMTggdW5hZGRyZXNzZWQgb3IgYWxyZWFkeSBhY2tub3dsZWRnZWQgdGFyZ2V0IHJlY2VpdmVkIGEgY29tbWFuZCIpCgkJfQoJCXJldHVybgoJfQoJdmFyIGNvbW1hbmQgc3RyaW5nCglpZiBqc29uLlVubWFyc2hhbChyZXN1bHQuQ29tbWFuZCwgJmNvbW1hbmQpICE9IG5pbCB8fCBjb21tYW5kICE9ICJzZWVrIiB8fCByZXN1bHQuUG9zaXRpb24gIT0gNCB7CgkJdC5GYXRhbCgiUjE4IGFkZHJlc3NlZCB0YXJnZXQgZGlkIG5vdCByZWNlaXZlIHRoZSBxdWV1ZWQgc2VlayBleGFjdGx5IikKCX0KfQo=", validate=True)


def identity():
    return dict(sourceUnchanged=True, toolUnchanged=True, trackedUnchanged=True,
                historyAdmitted=True, exactInvocation=True, sourcesPinned=True)


def execution():
    return dict(exitCode=0, stopReason=None, ownedProcessExited=True,
                ownedGroupSettled=True, captureSettled=True)


def packet():
    return dict(schemaVersion=1, index=0, formatterExitCode=0, timedOut=False,
                formatterExited=True, captureSettled=True, sourceUnchanged=True,
                toolUnchanged=True, stdoutBytes=len(SOURCE),
                stdoutSHA256=hashlib.sha256(SOURCE).hexdigest(), stderrBytes=0,
                stderrSHA256=hashlib.sha256(b"").hexdigest(),
                outputBase64=base64.b64encode(SOURCE).decode(), failureKind=None)


class FormatControls(unittest.TestCase):
    def record(self, **changes):
        arguments = dict(path=GO_FILES[0], original=SOURCE, output=SOURCE,
                         exit_code=0, stderr_bytes=0)
        return format_record(**(arguments | changes))

    def test_only_two_fixed_sources_and_stock_stdin_command(self):
        self.assertEqual(GO_FILES, (
            "apps/player/internal/server/home_assistant_document_targets_test.go",
            "apps/player/internal/server/home_assistant_document_targets_helpers_test.go"))
        self.assertEqual(formatter_command("/owned/tool"),
                         ["/owned/tool", "fmt", "--stdin", "--config", "apps/player/.golangci.yml"])

    def test_candidate_binds_source_and_never_authorizes_adoption(self):
        record = self.record()
        self.assertEqual(record["original"]["sha256"], hashlib.sha256(SOURCE).hexdigest())
        self.assertEqual(base64.b64decode(record["formattedSourceBase64"], validate=True), SOURCE)
        self.assertFalse(record["autoAdoption"])
        self.assertFalse(record["semanticsVerified"])

    def test_unknown_paths_and_unpinned_source_bytes_are_blocked(self):
        for changes in ({"path": "../private.go"}, {"path": GO_FILES[0] + "\n"},
                        {"original": SOURCE + b"\n"}, {"original": None}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                self.record(**changes)

    def test_empty_non_utf8_nul_wrong_package_or_oversized_output_is_blocked(self):
        for output in (b"", b"\xff", b"package server_test\n\0", b"package main\n",
                       b"x" * 32769, b"package server_test\n" + b"\n" * 300):
            with self.subTest(size=len(output)), self.assertRaises(ValueError):
                self.record(output=output)

    def test_actual_failure_codes_bool_and_any_stderr_never_credit_output(self):
        for code in (True, False, None, 1, 2, -9):
            with self.subTest(code=code), self.assertRaises(ValueError):
                self.record(exit_code=code)
        for count in (True, None, -1, 1):
            with self.subTest(count=count), self.assertRaises(ValueError):
                self.record(stderr_bytes=count)

    def test_missing_or_shallow_history_stays_a_prerequisite_with_actual_status(self):
        self.assertEqual(history_kind(False, 0, 0), "history-admitted")
        self.assertEqual(history_kind(True, 0, 0), "history-shallow")
        self.assertEqual(history_kind(False, 128, None), "history-anchor-unavailable")
        self.assertEqual(history_kind(False, 0, 1), "history-not-proven-ancestor")
        for values in ((None, 0, 0), (False, True, 0), (False, 0, True)):
            self.assertEqual(history_kind(*values), "history-incomplete")

    def test_complete_projection_requires_two_distinct_pinned_records(self):
        result = admit_format([self.record()], [execution()], identity())
        self.assertEqual(result["classification"], "prerequisite-blocked")
        result = admit_format([self.record(), self.record()], [execution(), execution()], identity())
        self.assertEqual(result["classification"], "prerequisite-blocked")
        self.assertFalse(result["autoAdoption"])

    def test_exact_complete_pair_is_only_a_proposal(self):
        records = [self.record(), format_record(GO_FILES[1], HELPER, HELPER, 0, 0)]
        result = admit_format(records, [execution(), execution()], identity())
        self.assertEqual(result["classification"], "canonical-source-proposals")
        self.assertFalse(result["autoAdoption"])
        for checks in (identity() | {"unexpected": True}, identity() | {"sourcesPinned": 1}):
            self.assertEqual(admit_format(records, [execution(), execution()], checks)["classification"], "prerequisite-blocked")
        altered = copy.deepcopy(records)
        altered[1]["formatted"]["sha256"] = "0" * 64
        self.assertEqual(admit_format(altered, [execution(), execution()], identity())["classification"], "prerequisite-blocked")

    def test_bridge_failure_preserves_actual_code_without_candidate(self):
        for code, kind in ((1, "formatter-exit"), (2, "formatter-exit"), (-9, "formatter-timeout")):
            data = packet() | {"formatterExitCode": code, "outputBase64": None,
                               "timedOut": code == -9, "failureKind": kind}
            projection = PacketProjection(0)
            projection.consume(json.dumps(data).encode())
            self.assertFalse(projection.prerequisite)
            self.assertEqual(projection.result()["formatterExitCode"], code)
            self.assertIsNone(projection.result()["outputBase64"])

    def test_owned_leader_is_reaped_or_explicitly_unsettled(self):
        process = Mock()
        process.poll.return_value = None
        process.wait.return_value = -9
        self.assertTrue(settle_child(process))
        process.kill.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=3)
        process.reset_mock()
        process.wait.side_effect = subprocess.TimeoutExpired("fixed-formatter", 3)
        self.assertFalse(settle_child(process))
        process.kill.assert_called_once_with()

    def test_every_unchanged_identity_and_settlement_check_is_mandatory(self):
        for field in identity():
            checks = identity()
            checks[field] = False
            self.assertEqual(admit_format([], [], checks)["classification"], "prerequisite-blocked")
        status = execution()
        status["ownedGroupSettled"] = False
        self.assertEqual(admit_format([], [status], identity())["classification"], "prerequisite-blocked")

    def test_bridge_packet_is_closed_and_output_identity_is_checked(self):
        projection = PacketProjection(0)
        projection.consume(json.dumps(packet()).encode())
        self.assertFalse(projection.prerequisite)
        self.assertEqual(projection.result()["formatterExitCode"], 0)
        for field, value in (("index", 1), ("formatterExitCode", True),
                             ("stdoutSHA256", "0" * 64), ("stderrBytes", 1),
                             ("outputBase64", "invalid")):
            with self.subTest(field=field):
                data = packet()
                data[field] = value
                projection = PacketProjection(0)
                projection.consume(json.dumps(data).encode())
                self.assertTrue(projection.prerequisite)

    def test_unknown_duplicate_or_incomplete_packets_never_export_private_output(self):
        data = packet() | {"rawStderr": "PRIVATE_AUTH_SENTINEL"}
        for raw in (json.dumps(data).encode(), b"PRIVATE_AUTH_SENTINEL",
                    b'{"index":0,"index":1}', b'{"schemaVersion":NaN}'):
            projection = PacketProjection(0)
            projection.consume(raw)
            self.assertTrue(projection.prerequisite)
            self.assertNotIn("PRIVATE_AUTH_SENTINEL", repr(projection.result()))
        projection = PacketProjection(0)
        self.assertIsNone(projection.result())
        projection.consume(json.dumps(packet()).encode())
        projection.consume(json.dumps(packet()).encode())
        self.assertTrue(projection.prerequisite)


if __name__ == "__main__":
    unittest.main()
