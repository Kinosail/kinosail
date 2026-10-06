"""Pure wrapper/admission controls; root injects GET-verified source modules."""
import base64
import copy
import hashlib
import json
import unittest
from unittest.mock import Mock, patch, MagicMock
from types import SimpleNamespace

import r18_public_under_test as public
import campaign_r18_format_public_artifacts as artifacts
import campaign_r18_format_public_files as files
import campaign_r18_document_format as formatter

SOURCE = base64.b64decode("cGFja2FnZSBzZXJ2ZXJfdGVzdAoKaW1wb3J0ICgKCSJuZXQvaHR0cCIKCSJzdHJpbmdzIgoJInRlc3RpbmciCikKCmZ1bmMgVGVzdEhvbWVBc3Npc3RhbnREb2N1bWVudFRhcmdldHNSZW1haW5JbmRlcGVuZGVudFRocm91Z2hQdWJsaWNBcHAodCAqdGVzdGluZy5UKSB7CgljbGllbnQgOj0gbmV3RG9jdW1lbnRUYXJnZXRDbGllbnQodCkKCWZpcnN0LCBzZWNvbmQgOj0gY2xpZW50LmNsYWltKHQsICIiKSwgY2xpZW50LmNsYWltKHQsICIiKQoJaWYgZmlyc3QuSUQgPT0gc2Vjb25kLklEIHx8IGZpcnN0LkNsYWltID09IHNlY29uZC5DbGFpbSB7CgkJdC5GYXRhbCgiUjE4IHNlcGFyYXRlIGRvY3VtZW50IGNsYWltcyBzaGFyZWQgdGFyZ2V0IGlkZW50aXR5IG9yIGF1dGhvcml0eSIpCgl9CgljbGllbnQucG9sbCh0LCBmaXJzdC5JRCwgW11zdHJpbmd7Zmlyc3QuQ2xhaW19LCBmYWxzZSkKCWNsaWVudC5wb2xsKHQsIHNlY29uZC5JRCwgW11zdHJpbmd7c2Vjb25kLkNsYWltfSwgZmFsc2UpCglzbmFwc2hvdCA6PSBjbGllbnQuc25hcHNob3QodCkKCWlmIGxlbihzbmFwc2hvdCkgIT0gMiB8fCBzbmFwc2hvdFtmaXJzdC5JRF0uUG9zaXRpb24gIT0gMSB8fCBzbmFwc2hvdFtzZWNvbmQuSURdLlBvc2l0aW9uICE9IDEgewoJCXQuRmF0YWwoIlIxOCBpbmRlcGVuZGVudCBkb2N1bWVudCBzdGF0ZXMgZGlkIG5vdCBwdWJsaXNoIHR3byBleGFjdCB0YXJnZXRzIikKCX0KCWNsaWVudC5xdWV1ZVNlZWsodCwgZmlyc3QuSUQpCgljbGllbnQucG9sbCh0LCBzZWNvbmQuSUQsIFtdc3RyaW5ne3NlY29uZC5DbGFpbX0sIGZhbHNlKQoJY2xpZW50LnBvbGwodCwgZmlyc3QuSUQsIFtdc3RyaW5ne2ZpcnN0LkNsYWltfSwgdHJ1ZSkKCWNsaWVudC5wb2xsKHQsIGZpcnN0LklELCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0sIGZhbHNlKQoJY2xpZW50LnBvbGwodCwgc2Vjb25kLklELCBbXXN0cmluZ3tzZWNvbmQuQ2xhaW19LCBmYWxzZSkKfQoKZnVuYyBUZXN0SG9tZUFzc2lzdGFudERvY3VtZW50U2libGluZ0Nhbm5vdE92ZXJ3cml0ZVJlbGVhc2VPckRyYWluKHQgKnRlc3RpbmcuVCkgewoJY2xpZW50IDo9IG5ld0RvY3VtZW50VGFyZ2V0Q2xpZW50KHQpCgljbGFpbSA6PSBjbGllbnQuY2xhaW0odCwgInIxOC1zaWJsaW5nLWZpeHR1cmUiKQoJcGF0aCA6PSBkb2N1bWVudFRhcmdldHNQYXRoICsgIi8iICsgY2xhaW0uSUQKCWNsaWVudC5wb2xsKHQsIGNsYWltLklELCBbXXN0cmluZ3tjbGFpbS5DbGFpbX0sIGZhbHNlKQoJY2xpZW50LnF1ZXVlU2Vlayh0LCBjbGFpbS5JRCkKCWJlZm9yZSA6PSBjbGllbnQuc25hcHNob3QodCkKCWNvbmZsaWN0IDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgZG9jdW1lbnRUYXJnZXRzUGF0aCsiL2NsYWltcyIsIGB7ImlkIjoicjE4LXNpYmxpbmctZml4dHVyZSJ9YCwgbmlsKQoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQsIGNvbmZsaWN0LCBodHRwLlN0YXR1c0NvbmZsaWN0KQoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U25hcHNob3QodCwgY2xpZW50LnNuYXBzaG90KHQpLCBiZWZvcmUpCgloZWFkZXJzIDo9IFtdW11zdHJpbmd7bmlsLCB7Indyb25nLWRvY3VtZW50LWNsYWltLTAwMDAwMDAwMCJ9LCB7Y2xhaW0uQ2xhaW0sIGNsYWltLkNsYWltfSwge3N0cmluZ3MuUmVwZWF0KCJhIiwgNjUpfX0KCWZvciBfLCB2YWx1ZXMgOj0gcmFuZ2UgaGVhZGVycyB7CgkJcmVqZWN0ZWQgOj0gY2xpZW50LmNhbGwodCwgaHR0cC5NZXRob2RQdXQsIHBhdGgsIGRvY3VtZW50VGFyZ2V0Qm9keSg5KSwgdmFsdWVzKQoJCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCByZWplY3RlZCwgaHR0cC5TdGF0dXNGb3JiaWRkZW4pCgkJcmVxdWlyZURvY3VtZW50VGFyZ2V0U25hcHNob3QodCwgY2xpZW50LnNuYXBzaG90KHQpLCBiZWZvcmUpCgkJcmVsZWFzZWQgOj0gY2xpZW50LmNhbGwodCwgaHR0cC5NZXRob2RQb3N0LCBwYXRoKyIvcmVsZWFzZSIsIGB7fWAsIHZhbHVlcykKCQlyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVsZWFzZWQsIGh0dHAuU3RhdHVzRm9yYmlkZGVuKQoJCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJfQoJY2xpZW50LnBvbGwodCwgY2xhaW0uSUQsIFtdc3RyaW5ne2NsYWltLkNsYWltfSwgdHJ1ZSkKCWNsaWVudC5wb2xsKHQsIGNsYWltLklELCBbXXN0cmluZ3tjbGFpbS5DbGFpbX0sIGZhbHNlKQp9CgpmdW5jIFRlc3RIb21lQXNzaXN0YW50RG9jdW1lbnRSZWxlYXNlS2VlcHNDYW5kaWRhdGVBbmRSZWplY3RzUmV0aXJlZENsYWltKHQgKnRlc3RpbmcuVCkgewoJY2xpZW50IDo9IG5ld0RvY3VtZW50VGFyZ2V0Q2xpZW50KHQpCglmaXJzdCA6PSBjbGllbnQuY2xhaW0odCwgInIxOC1yZWxvYWQtZml4dHVyZSIpCglwYXRoIDo9IGRvY3VtZW50VGFyZ2V0c1BhdGggKyAiLyIgKyBmaXJzdC5JRAoJY2xpZW50LnBvbGwodCwgZmlyc3QuSUQsIFtdc3RyaW5ne2ZpcnN0LkNsYWltfSwgZmFsc2UpCglyZWxlYXNlIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgcGF0aCsiL3JlbGVhc2UiLCBge31gLCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0pCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVsZWFzZSwgaHR0cC5TdGF0dXNOb0NvbnRlbnQpCglpZiBsZW4oY2xpZW50LnNuYXBzaG90KHQpKSAhPSAwIHsKCQl0LkZhdGFsKCJSMTggc3VjY2Vzc2Z1bCBkb2N1bWVudCByZWxlYXNlIGxlZnQgYSBwdWJsaXNoZWQgdGFyZ2V0IikKCX0KCXNlY29uZCA6PSBjbGllbnQuY2xhaW0odCwgZmlyc3QuSUQpCglpZiBzZWNvbmQuSUQgIT0gZmlyc3QuSUQgfHwgc2Vjb25kLkNsYWltID09IGZpcnN0LkNsYWltIHsKCQl0LkZhdGFsKCJSMTggcmVjbGFpbSBjaGFuZ2VkIGF1dG9tYXRpb24gaWRlbnRpdHkgb3IgcmV1c2VkIHJldGlyZWQgYXV0aG9yaXR5IikKCX0KCWNsaWVudC5wb2xsKHQsIHNlY29uZC5JRCwgW11zdHJpbmd7c2Vjb25kLkNsYWltfSwgZmFsc2UpCgljbGllbnQucXVldWVTZWVrKHQsIHNlY29uZC5JRCkKCWJlZm9yZSA6PSBjbGllbnQuc25hcHNob3QodCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFB1dCwgcGF0aCwgZG9jdW1lbnRUYXJnZXRCb2R5KDkpLCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0pLCBodHRwLlN0YXR1c0ZvcmJpZGRlbikKCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQsIGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgcGF0aCsiL3JlbGVhc2UiLCBge31gLCBbXXN0cmluZ3tmaXJzdC5DbGFpbX0pLCBodHRwLlN0YXR1c0ZvcmJpZGRlbikKCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJY2xpZW50LnBvbGwodCwgc2Vjb25kLklELCBbXXN0cmluZ3tzZWNvbmQuQ2xhaW19LCB0cnVlKQoJY2xpZW50LnBvbGwodCwgc2Vjb25kLklELCBbXXN0cmluZ3tzZWNvbmQuQ2xhaW19LCBmYWxzZSkKfQoKZnVuYyBUZXN0SG9tZUFzc2lzdGFudERvY3VtZW50TmF0aXZlU3RyaWN0Q29udHJhY3RSZW1haW5zVW5jaGFuZ2VkKHQgKnRlc3RpbmcuVCkgewoJY2xpZW50IDo9IG5ld0RvY3VtZW50VGFyZ2V0Q2xpZW50KHQpCgljb25zdCBpZCA9ICJyMTgtbmF0aXZlLWZpeHR1cmUiCglwYXRoIDo9IGRvY3VtZW50VGFyZ2V0c1BhdGggKyAiLyIgKyBpZAoJY2xpZW50LnBvbGwodCwgaWQsIG5pbCwgZmFsc2UpCgljbGllbnQucXVldWVTZWVrKHQsIGlkKQoJYmVmb3JlIDo9IGNsaWVudC5zbmFwc2hvdCh0KQoJdW5rbm93blN0YXRlIDo9IHN0cmluZ3MuVHJpbVN1ZmZpeChkb2N1bWVudFRhcmdldEJvZHkoOSksICJ9IikgKyBgLCJjbGFpbSI6InVua25vd24tZmllbGQifWAKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFB1dCwgcGF0aCwgdW5rbm93blN0YXRlLCBuaWwpLCBodHRwLlN0YXR1c0JhZFJlcXVlc3QpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTbmFwc2hvdCh0LCBjbGllbnQuc25hcHNob3QodCksIGJlZm9yZSkKCXVua25vd25Db21tYW5kIDo9IGB7ImNvbW1hbmQiOiJzZWVrIiwicG9zaXRpb24iOjksImNsYWltIjoidW5rbm93bi1maWVsZCJ9YAoJcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQsIGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUG9zdCwgcGF0aCsiL2NvbW1hbmRzIiwgdW5rbm93bkNvbW1hbmQsIG5pbCksIGh0dHAuU3RhdHVzQmFkUmVxdWVzdCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFNuYXBzaG90KHQsIGNsaWVudC5zbmFwc2hvdCh0KSwgYmVmb3JlKQoJY2xpZW50LnBvbGwodCwgaWQsIG5pbCwgdHJ1ZSkKCWNsaWVudC5wb2xsKHQsIGlkLCBuaWwsIGZhbHNlKQp9Cg==", validate=True)
HELPER = base64.b64decode("cGFja2FnZSBzZXJ2ZXJfdGVzdAoKaW1wb3J0ICgKCSJjb250ZXh0IgoJImVuY29kaW5nL2pzb24iCgkiaW8iCgkibmV0L2h0dHAiCgkibmV0L2h0dHAvaHR0cHRlc3QiCgkicmVmbGVjdCIKCSJyZWdleHAiCgkic3RyaW5ncyIKCSJ0ZXN0aW5nIgoJInRpbWUiCikKCi8vIFRoZXNlIHRlc3RzIGV4ZXJjaXNlIHRoZSByZWFsIGF1dGhlbnRpY2F0ZWQgYXBwbGljYXRpb24gaGFuZGxlciBhbmQgcHVibGljCi8vIHJvdXRlcy4gVGhleSBkbyBub3Qgc2ltdWxhdGUgYnJvd3NlciBzdG9yYWdlLCBjbGFpbSBtZWRpYSBwbGF5YmFjaywgb3IgYnlwYXNzCi8vIENTUkYuIFRoZSBjYW5vbmljYWwgQVBJIGZpeHR1cmUgc3VwcGxpZXMgZGlzcG9zYWJsZSBPd25lciBlbnJvbGxtZW50Lgpjb25zdCBkb2N1bWVudFRhcmdldHNQYXRoID0gIi9hcGkvdjEvaG9tZS1hc3Npc3RhbnQvcGxheWVycyIKCnZhciBkb2N1bWVudFRhcmdldElEID0gcmVnZXhwLk11c3RDb21waWxlKGBeW0EtWmEtejAtOV8tXXsxLDY0fSRgKQoKdHlwZSBkb2N1bWVudFRhcmdldENsaWVudCBzdHJ1Y3QgewoJaGFuZGxlciBodHRwLkhhbmRsZXIKCWNvb2tpZSAgKmh0dHAuQ29va2llCgljc3JmICAgIHN0cmluZwp9Cgp0eXBlIGRvY3VtZW50VGFyZ2V0Q2FwdHVyZSBzdHJ1Y3QgewoJKmh0dHB0ZXN0LlJlc3BvbnNlUmVjb3JkZXIKCW92ZXJmbG93IGJvb2wKfQoKZnVuYyAoY2FwdHVyZSAqZG9jdW1lbnRUYXJnZXRDYXB0dXJlKSBXcml0ZShib2R5IFtdYnl0ZSkgKGludCwgZXJyb3IpIHsKCWlmIGxlbihib2R5KSA+IDI2MjE0NC1jYXB0dXJlLkJvZHkuTGVuKCkgewoJCWNhcHR1cmUub3ZlcmZsb3cgPSB0cnVlCgkJcmV0dXJuIDAsIGlvLkVyclNob3J0V3JpdGUKCX0KCXJldHVybiBjYXB0dXJlLlJlc3BvbnNlUmVjb3JkZXIuV3JpdGUoYm9keSkKfQoKZnVuYyAoY2FwdHVyZSAqZG9jdW1lbnRUYXJnZXRDYXB0dXJlKSBXcml0ZVN0cmluZyhib2R5IHN0cmluZykgKGludCwgZXJyb3IpIHsKCXJldHVybiBjYXB0dXJlLldyaXRlKFtdYnl0ZShib2R5KSkKfQoKdHlwZSBkb2N1bWVudFRhcmdldENsYWltIHN0cnVjdCB7CglJRCAgICAgICAgc3RyaW5nIGBqc29uOiJpZCJgCglDbGFpbSAgICAgc3RyaW5nIGBqc29uOiJjbGFpbSJgCglFeHBpcmVzSW4gaW50ICAgIGBqc29uOiJleHBpcmVzSW4iYAp9Cgp0eXBlIGRvY3VtZW50VGFyZ2V0U3RhdGUgc3RydWN0IHsKCUlEICAgICAgIHN0cmluZyAgYGpzb246ImlkImAKCU5hbWUgICAgIHN0cmluZyAgYGpzb246Im5hbWUiYAoJU3RhdGUgICAgc3RyaW5nICBganNvbjoic3RhdGUiYAoJVGl0bGUgICAgc3RyaW5nICBganNvbjoidGl0bGUiYAoJSXRlbUlEICAgc3RyaW5nICBganNvbjoiaXRlbUlkImAKCVBvc2l0aW9uIGZsb2F0NjQgYGpzb246InBvc2l0aW9uImAKCUR1cmF0aW9uIGZsb2F0NjQgYGpzb246ImR1cmF0aW9uImAKCVZvbHVtZSAgIGZsb2F0NjQgYGpzb246InZvbHVtZSJgCglNdXRlZCAgICBib29sICAgIGBqc29uOiJtdXRlZCJgCn0KCmZ1bmMgbmV3RG9jdW1lbnRUYXJnZXRDbGllbnQodCAqdGVzdGluZy5UKSAqZG9jdW1lbnRUYXJnZXRDbGllbnQgewoJdC5IZWxwZXIoKQoJaGFuZGxlciwgb3duZXIgOj0gYXBpU2VydmVyKHQpCgllbmFibGVkIDo9IGFwaUNhbGwodCwgaGFuZGxlciwgb3duZXIsIGh0dHAuTWV0aG9kUHV0LCAiL2FwaS92MS9zZXR0aW5ncy9ob21lLWFzc2lzdGFudCIsIG1hcFtzdHJpbmddYW55eyJlbmFibGVkIjogdHJ1ZX0pCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgZW5hYmxlZCwgaHR0cC5TdGF0dXNPSykKCWNsaWVudCA6PSAmZG9jdW1lbnRUYXJnZXRDbGllbnR7aGFuZGxlcjogaGFuZGxlciwgY29va2llOiAmaHR0cC5Db29raWV7CgkJTmFtZTogIl9fSG9zdC1raW5vc2FpbF9zZXNzaW9uIiwgVmFsdWU6IG93bmVyLCBTZWN1cmU6IHRydWUsCgkJSHR0cE9ubHk6IHRydWUsIFBhdGg6ICIvIiwgU2FtZVNpdGU6IGh0dHAuU2FtZVNpdGVTdHJpY3RNb2RlLAoJfX0KCXNldHRpbmdzIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kR2V0LCAiL3NldHRpbmdzIiwgIiIsIG5pbCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCBzZXR0aW5ncywgaHR0cC5TdGF0dXNPSykKCW1hdGNoIDo9IHJlZ2V4cC5NdXN0Q29tcGlsZShgPG1ldGEgbmFtZT0ia2lub3NhaWwtY3NyZiIgY29udGVudD0iKFteIl0rKSI+YCkuRmluZFN0cmluZ1N1Ym1hdGNoKHNldHRpbmdzLkJvZHkuU3RyaW5nKCkpCglpZiBsZW4obWF0Y2gpICE9IDIgfHwgbGVuKG1hdGNoWzFdKSA9PSAwIHx8IGxlbihtYXRjaFsxXSkgPiAyNTYgewoJCXQuRmF0YWwoIlIxOCBhdXRoZW50aWNhdGVkIFNldHRpbmdzIENTUkYgcHJlcmVxdWlzaXRlIHdhcyBub3QgbWV0IikKCX0KCWNsaWVudC5jc3JmID0gbWF0Y2hbMV0KCXZhciBtZSBzdHJ1Y3QgewoJCVZpZXdlciBzdHJ1Y3QgewoJCQlJRCAgICBzdHJpbmcgYGpzb246ImlkImAKCQkJT3duZXIgYm9vbCAgIGBqc29uOiJvd25lciJgCgkJfSBganNvbjoidmlld2VyImAKCX0KCXJlc3BvbnNlIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kR2V0LCAiL2FwaS92MS9tZSIsICIiLCBuaWwpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVzcG9uc2UsIGh0dHAuU3RhdHVzT0spCglyZWFkRG9jdW1lbnRUYXJnZXRKU09OKHQsIHJlc3BvbnNlLCAmbWUpCglpZiBtZS5WaWV3ZXIuSUQgPT0gIiIgfHwgIW1lLlZpZXdlci5Pd25lciB7CgkJdC5GYXRhbCgiUjE4IHB1YmxpYyBjdXJyZW50LU93bmVyIGlkZW50aXR5IHByZXJlcXVpc2l0ZSB3YXMgbm90IG1ldCIpCgl9CglyZXR1cm4gY2xpZW50Cn0KCmZ1bmMgKGNsaWVudCAqZG9jdW1lbnRUYXJnZXRDbGllbnQpIGNhbGwodCAqdGVzdGluZy5ULCBtZXRob2QsIHBhdGgsIGJvZHkgc3RyaW5nLCBjbGFpbXMgW11zdHJpbmcpICpodHRwdGVzdC5SZXNwb25zZVJlY29yZGVyIHsKCXQuSGVscGVyKCkKCWlmIGxlbihib2R5KSA+IDQwOTYgewoJCXQuRmF0YWwoIlIxOCBhdXRob3JlZCByZXF1ZXN0IGV4Y2VlZGVkIGl0cyBib2R5IGJvdW5kIikKCX0KCWN0eCwgY2FuY2VsIDo9IGNvbnRleHQuV2l0aFRpbWVvdXQodC5Db250ZXh0KCksIDMqdGltZS5TZWNvbmQpCglkZWZlciBjYW5jZWwoKQoJcmVxdWVzdCA6PSBodHRwdGVzdC5OZXdSZXF1ZXN0V2l0aENvbnRleHQoY3R4LCBtZXRob2QsICJodHRwczovL2tpbm9zYWlsLnRlc3QiK3BhdGgsIHN0cmluZ3MuTmV3UmVhZGVyKGJvZHkpKQoJZGVmZXIgZnVuYygpIHsgXyA9IHJlcXVlc3QuQm9keS5DbG9zZSgpIH0oKQoJcmVxdWVzdC5IZWFkZXIuU2V0KCJVc2VyLUFnZW50IiwgIktpbm9zYWlsIGRvY3VtZW50IHRhcmdldCByZWdyZXNzaW9uIikKCXJlcXVlc3QuSGVhZGVyLlNldCgiT3JpZ2luIiwgImh0dHBzOi8va2lub3NhaWwudGVzdCIpCglyZXF1ZXN0LkhlYWRlci5TZXQoIkNvbnRlbnQtVHlwZSIsICJhcHBsaWNhdGlvbi9qc29uIikKCXJlcXVlc3QuQWRkQ29va2llKGNsaWVudC5jb29raWUpCglpZiBjbGllbnQuY3NyZiAhPSAiIiB7CgkJcmVxdWVzdC5IZWFkZXIuU2V0KCJYLUtpbm9zYWlsLUNTUkYiLCBjbGllbnQuY3NyZikKCX0KCWZvciBfLCBjbGFpbSA6PSByYW5nZSBjbGFpbXMgewoJCXJlcXVlc3QuSGVhZGVyLkFkZCgiWC1LaW5vc2FpbC1QbGF5ZXItQ2xhaW0iLCBjbGFpbSkKCX0KCXJlc3BvbnNlIDo9ICZkb2N1bWVudFRhcmdldENhcHR1cmV7UmVzcG9uc2VSZWNvcmRlcjogaHR0cHRlc3QuTmV3UmVjb3JkZXIoKX0KCWNsaWVudC5oYW5kbGVyLlNlcnZlSFRUUChyZXNwb25zZSwgcmVxdWVzdCkKCWlmIGN0eC5FcnIoKSAhPSBuaWwgewoJCXQuRmF0YWwoIlIxOCBwdWJsaWMgcmVxdWVzdCBkaWQgbm90IHNldHRsZSBpbnNpZGUgaXRzIGRlYWRsaW5lIikKCX0KCWlmIHJlc3BvbnNlLm92ZXJmbG93IHsKCQl0LkZhdGFsKCJSMTggcHVibGljIHJlc3BvbnNlIGV4Y2VlZGVkIGl0cyBjYXB0dXJlIGJvdW5kIikKCX0KCXJldHVybiByZXNwb25zZS5SZXNwb25zZVJlY29yZGVyCn0KCmZ1bmMgcmVxdWlyZURvY3VtZW50VGFyZ2V0U3RhdHVzKHQgKnRlc3RpbmcuVCwgcmVzcG9uc2UgKmh0dHB0ZXN0LlJlc3BvbnNlUmVjb3JkZXIsIHdhbnQgaW50KSB7Cgl0LkhlbHBlcigpCglpZiByZXNwb25zZS5Db2RlICE9IHdhbnQgewoJCXQuRmF0YWxmKCJSMTggcHVibGljIHJlc3BvbnNlIEhUVFAgJWQsIGV4cGVjdGVkICVkIiwgcmVzcG9uc2UuQ29kZSwgd2FudCkKCX0KfQoKZnVuYyByZWFkRG9jdW1lbnRUYXJnZXRKU09OKHQgKnRlc3RpbmcuVCwgcmVzcG9uc2UgKmh0dHB0ZXN0LlJlc3BvbnNlUmVjb3JkZXIsIHRhcmdldCBhbnkpIHsKCXQuSGVscGVyKCkKCWlmIHJlc3BvbnNlLkJvZHkuTGVuKCkgPiAxNjM4NCB7CgkJdC5GYXRhbCgiUjE4IEpTT04gcmVzcG9uc2UgZXhjZWVkZWQgaXRzIGRlY29kZSBib3VuZCIpCgl9CglpZiBqc29uLlVubWFyc2hhbChyZXNwb25zZS5Cb2R5LkJ5dGVzKCksIHRhcmdldCkgIT0gbmlsIHsKCQl0LkZhdGFsKCJSMTggcHVibGljIHJlc3BvbnNlIHdhcyBub3QgdGhlIGV4cGVjdGVkIEpTT04iKQoJfQp9CgpmdW5jIChjbGllbnQgKmRvY3VtZW50VGFyZ2V0Q2xpZW50KSBjbGFpbSh0ICp0ZXN0aW5nLlQsIGlkIHN0cmluZykgZG9jdW1lbnRUYXJnZXRDbGFpbSB7Cgl0LkhlbHBlcigpCglib2R5LCBlcnIgOj0ganNvbi5NYXJzaGFsKHN0cnVjdCB7CgkJSUQgc3RyaW5nIGBqc29uOiJpZCxvbWl0ZW1wdHkiYAoJfXtpZH0pCglpZiBlcnIgIT0gbmlsIHsKCQl0LkZhdGFsKCJSMTggYXV0aG9yZWQgY2xhaW0gcmVxdWVzdCBjb3VsZCBub3QgYmUgZW5jb2RlZCIpCgl9CglyZXNwb25zZSA6PSBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFBvc3QsIGRvY3VtZW50VGFyZ2V0c1BhdGgrIi9jbGFpbXMiLCBzdHJpbmcoYm9keSksIG5pbCkKCWlmIHJlc3BvbnNlLkNvZGUgIT0gaHR0cC5TdGF0dXNDcmVhdGVkIHsKCQl0LkZhdGFsZigiUjE4IGNsYWltLXJvdXRlIHByZXJlcXVpc2l0ZTogSFRUUCAlZCwgZXhwZWN0ZWQgMjAxIiwgcmVzcG9uc2UuQ29kZSkKCX0KCXZhciBjbGFpbSBkb2N1bWVudFRhcmdldENsYWltCglyZWFkRG9jdW1lbnRUYXJnZXRKU09OKHQsIHJlc3BvbnNlLCAmY2xhaW0pCglpZiAhZG9jdW1lbnRUYXJnZXRJRC5NYXRjaFN0cmluZyhjbGFpbS5JRCkgfHwgIWRvY3VtZW50VGFyZ2V0SUQuTWF0Y2hTdHJpbmcoY2xhaW0uQ2xhaW0pIHsKCQl0LkZhdGFsKCJSMTggY2xhaW0gcmVzcG9uc2UgaWRlbnRpdGllcyB2aW9sYXRlZCB0aGUgcHVibGljIGJvdW5kIikKCX0KCWlmIGxlbihjbGFpbS5DbGFpbSkgPCAyMCB8fCBjbGFpbS5FeHBpcmVzSW4gIT0gMzAgfHwgcmVzcG9uc2UuSGVhZGVyKCkuR2V0KCJDYWNoZS1Db250cm9sIikgIT0gIm5vLXN0b3JlIiB7CgkJdC5GYXRhbCgiUjE4IGNsYWltIHJlc3BvbnNlIG93bmVyc2hpcCwgbGVhc2Ugb3IgY2FjaGUgY29udHJhY3Qgd2FzIGludmFsaWQiKQoJfQoJaWYgaWQgIT0gIiIgJiYgY2xhaW0uSUQgIT0gaWQgewoJCXQuRmF0YWwoIlIxOCBjbGFpbSBjaGFuZ2VkIHRoZSByZXF1ZXN0ZWQgc3RhYmxlIGNhbmRpZGF0ZSIpCgl9CglyZXR1cm4gY2xhaW0KfQoKZnVuYyAoY2xpZW50ICpkb2N1bWVudFRhcmdldENsaWVudCkgc25hcHNob3QodCAqdGVzdGluZy5UKSBtYXBbc3RyaW5nXWRvY3VtZW50VGFyZ2V0U3RhdGUgewoJdC5IZWxwZXIoKQoJcmVzcG9uc2UgOj0gY2xpZW50LmNhbGwodCwgaHR0cC5NZXRob2RHZXQsIGRvY3VtZW50VGFyZ2V0c1BhdGgsICIiLCBuaWwpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVzcG9uc2UsIGh0dHAuU3RhdHVzT0spCgl2YXIgcmVzdWx0IHN0cnVjdCB7CgkJUGxheWVycyBbXWRvY3VtZW50VGFyZ2V0U3RhdGUgYGpzb246InBsYXllcnMiYAoJfQoJcmVhZERvY3VtZW50VGFyZ2V0SlNPTih0LCByZXNwb25zZSwgJnJlc3VsdCkKCWlmIHJlc3VsdC5QbGF5ZXJzID09IG5pbCB7CgkJdC5GYXRhbCgiUjE4IHRhcmdldCBzbmFwc2hvdCBvbWl0dGVkIGl0cyBwdWJsaWMgcGxheWVyIGxpc3QiKQoJfQoJcGxheWVycyA6PSBtYWtlKG1hcFtzdHJpbmddZG9jdW1lbnRUYXJnZXRTdGF0ZSwgbGVuKHJlc3VsdC5QbGF5ZXJzKSkKCWZvciBfLCBwbGF5ZXIgOj0gcmFuZ2UgcmVzdWx0LlBsYXllcnMgewoJCWlmICFkb2N1bWVudFRhcmdldElELk1hdGNoU3RyaW5nKHBsYXllci5JRCkgewoJCQl0LkZhdGFsKCJSMTggcHVibGlzaGVkIHRhcmdldCBpZGVudGl0eSB3YXMgaW52YWxpZCIpCgkJfQoJCWlmIF8sIGR1cGxpY2F0ZSA6PSBwbGF5ZXJzW3BsYXllci5JRF07IGR1cGxpY2F0ZSB7CgkJCXQuRmF0YWwoIlIxOCBwdWJsaXNoZWQgc25hcHNob3QgY29udGFpbmVkIGR1cGxpY2F0ZSB0YXJnZXQgaWRlbnRpdGllcyIpCgkJfQoJCXBsYXllcnNbcGxheWVyLklEXSA9IHBsYXllcgoJfQoJcmV0dXJuIHBsYXllcnMKfQoKZnVuYyByZXF1aXJlRG9jdW1lbnRUYXJnZXRTbmFwc2hvdCh0ICp0ZXN0aW5nLlQsIGdvdCwgd2FudCBtYXBbc3RyaW5nXWRvY3VtZW50VGFyZ2V0U3RhdGUpIHsKCXQuSGVscGVyKCkKCWlmICFyZWZsZWN0LkRlZXBFcXVhbChnb3QsIHdhbnQpIHsKCQl0LkZhdGFsKCJSMTggcmVqZWN0ZWQgcmVxdWVzdCBjaGFuZ2VkIHRoZSBjb21wbGV0ZSBwdWJsaXNoZWQgdGFyZ2V0IHNuYXBzaG90IikKCX0KfQoKZnVuYyAoY2xpZW50ICpkb2N1bWVudFRhcmdldENsaWVudCkgcXVldWVTZWVrKHQgKnRlc3RpbmcuVCwgaWQgc3RyaW5nKSB7Cgl0LkhlbHBlcigpCglyZXNwb25zZSA6PSBjbGllbnQuY2FsbCh0LCBodHRwLk1ldGhvZFBvc3QsIGRvY3VtZW50VGFyZ2V0c1BhdGgrIi8iK2lkKyIvY29tbWFuZHMiLCBgeyJjb21tYW5kIjoic2VlayIsInBvc2l0aW9uIjo0fWAsIG5pbCkKCXJlcXVpcmVEb2N1bWVudFRhcmdldFN0YXR1cyh0LCByZXNwb25zZSwgaHR0cC5TdGF0dXNBY2NlcHRlZCkKfQoKZnVuYyBkb2N1bWVudFRhcmdldEJvZHkocG9zaXRpb24gaW50KSBzdHJpbmcgewoJaWYgcG9zaXRpb24gPT0gOSB7CgkJcmV0dXJuIGB7Im5hbWUiOiJGaWN0aW9uYWwgZG9jdW1lbnQgdGFyZ2V0Iiwic3RhdGUiOiJwYXVzZWQiLCJwb3NpdGlvbiI6OSwiZHVyYXRpb24iOjEyLCJ2b2x1bWUiOjAuNX1gCgl9CglyZXR1cm4gYHsibmFtZSI6IkZpY3Rpb25hbCBkb2N1bWVudCB0YXJnZXQiLCJzdGF0ZSI6InBhdXNlZCIsInBvc2l0aW9uIjoxLCJkdXJhdGlvbiI6MTIsInZvbHVtZSI6MC41fWAKfQoKZnVuYyAoY2xpZW50ICpkb2N1bWVudFRhcmdldENsaWVudCkgcG9sbCh0ICp0ZXN0aW5nLlQsIGlkIHN0cmluZywgY2xhaW1zIFtdc3RyaW5nLCBzZWVrIGJvb2wpIHsKCXQuSGVscGVyKCkKCXJlc3BvbnNlIDo9IGNsaWVudC5jYWxsKHQsIGh0dHAuTWV0aG9kUHV0LCBkb2N1bWVudFRhcmdldHNQYXRoKyIvIitpZCwgZG9jdW1lbnRUYXJnZXRCb2R5KDEpLCBjbGFpbXMpCglyZXF1aXJlRG9jdW1lbnRUYXJnZXRTdGF0dXModCwgcmVzcG9uc2UsIGh0dHAuU3RhdHVzT0spCgl2YXIgcmVzdWx0IHN0cnVjdCB7CgkJQ29tbWFuZCAganNvbi5SYXdNZXNzYWdlIGBqc29uOiJjb21tYW5kImAKCQlQb3NpdGlvbiBmbG9hdDY0ICAgICAgICAgYGpzb246InBvc2l0aW9uImAKCX0KCXJlYWREb2N1bWVudFRhcmdldEpTT04odCwgcmVzcG9uc2UsICZyZXN1bHQpCglpZiAhc2VlayB7CgkJaWYgc3RyaW5nKHJlc3VsdC5Db21tYW5kKSAhPSAibnVsbCIgewoJCQl0LkZhdGFsKCJSMTggdW5hZGRyZXNzZWQgb3IgYWxyZWFkeSBhY2tub3dsZWRnZWQgdGFyZ2V0IHJlY2VpdmVkIGEgY29tbWFuZCIpCgkJfQoJCXJldHVybgoJfQoJdmFyIGNvbW1hbmQgc3RyaW5nCglpZiBqc29uLlVubWFyc2hhbChyZXN1bHQuQ29tbWFuZCwgJmNvbW1hbmQpICE9IG5pbCB8fCBjb21tYW5kICE9ICJzZWVrIiB8fCByZXN1bHQuUG9zaXRpb24gIT0gNCB7CgkJdC5GYXRhbCgiUjE4IGFkZHJlc3NlZCB0YXJnZXQgZGlkIG5vdCByZWNlaXZlIHRoZSBxdWV1ZWQgc2VlayBleGFjdGx5IikKCX0KfQo=", validate=True)

SHA = "a" * 40
ENV = {"CAMPAIGN_PROOF": "R18", "CAMPAIGN_R18_SUITE": "source-format",
       "GITHUB_ACTIONS": "true", "RUNNER_OS": "Linux", "GITHUB_SHA": SHA, "RUNNER_TEMP": "/private"}

def encode(value):
    return (json.dumps(value, sort_keys=True, allow_nan=False) + "\n").encode()

def process(code=0):
    return dict(exitCode=code, stopReason=None, ownedProcessExited=True,
                ownedGroupSettled=True, captureSettled=True, processLaunched=True, seconds=0.1)

def bind(values):
    raw = {name: encode(value) for name, value in values.items()}
    manifest = {"schemaVersion": 1, "artifacts": [
        {"name": name, "bytes": len(raw[name]), "sha256": hashlib.sha256(raw[name]).hexdigest()}
        for name in artifacts.NAMES[:3]]}
    raw["artifact-manifest.json"] = encode(manifest)
    return raw

def fixture():
    rows = [{"path": path, "gitBlob": oid, "sha256": sha, "bytes": size, "mode": 0o644}
            for path, (oid, sha, size) in artifacts.REQUIRED.items()]
    tools = [{"name": name, "gitBlob": "b" * 40, "sha256": "c" * 64, "bytes": 10, "mode": 0o755}
             for name in ("bin/go", "bin/gofmt", "VERSION", "pkg/tool/linux_amd64/compile", "python", "golangci-lint")]
    state = dict(revision=SHA, tree="d" * 40, sourceSHA256=artifacts.digest(rows),
                 toolchainSHA256=artifacts.digest(tools), inputCount=len(rows))
    records = [formatter.format_record(name, data, data, 0, 0)
               for name, data in zip(formatter.GO_FILES, (SOURCE, HELPER), strict=True)]
    packets = []
    for index, data in enumerate((SOURCE, HELPER)):
        packets.append(dict(schemaVersion=1, index=index, formatterExitCode=0, timedOut=False,
                            formatterExited=True, captureSettled=True, sourceUnchanged=True,
                            toolUnchanged=True, stdoutBytes=len(data), stdoutSHA256=hashlib.sha256(data).hexdigest(),
                            stderrBytes=0, stderrSHA256=hashlib.sha256(b"").hexdigest(), outputBase64=None, failureKind=None))
    processes = []
    for purpose in artifacts.PURPOSES:
        row = process() | {"purpose": purpose}
        if purpose.startswith("history-"):
            row |= {"captureBytes": 5, "captureSHA256": "0" * 64}
        elif purpose in ("hash", "silent", "files"):
            row |= {"captureLines": 0, "captureSHA256": "0" * 64}
        processes.append(row)
    tool = {key: value for key, value in tools[-1].items() if key != "name"}
    receipt = dict(schemaVersion=1, phase="canonical-format", expectedRevision=SHA,
                   baseRevision=formatter.BASE, baseTree=formatter.BASE_TREE,
                   invocation="golangci-fmt-stdin-player-two-v1", startedUTC="2026-10-05T12:00:00+00:00",
                   finishedUTC="2026-10-05T12:00:00+00:00", classification="canonical-source-proposals",
                   autoAdoption=False, semanticsVerified=False, processes=processes, formatterPackets=packets,
                   historyKind="history-admitted", beforeState=state, afterState=state, formatterBefore=tool,
                   formatterAfter=tool, count=2, identity={name: True for name in formatter.IDENTITY_FIELDS}, seconds=0.1)
    return {"receipt.json": receipt, "results.json": dict(schemaVersion=1, records=records, autoAdoption=False),
            "source-manifest.json": dict(schemaVersion=1, state=state, trackedInputs=rows, tools=tools)}

class PublicControls(unittest.TestCase):
    def test_only_exact_hosted_selection_without_cli_options(self):
        self.assertEqual(public.selection(ENV, ["wrapper"])[0], SHA)
        for name, value in (("CAMPAIGN_PROOF", "R06"), ("CAMPAIGN_R18_SUITE", "compile"),
                            ("GITHUB_ACTIONS", "false"), ("RUNNER_OS", "macOS"),
                            ("GITHUB_SHA", "a" * 39), ("RUNNER_TEMP", "../private")):
            with self.subTest(name=name), self.assertRaises(ValueError):
                public.selection(ENV | {name: value}, ["wrapper"])
        for argv in ([], ["wrapper", "--output", "/private"], ["wrapper", "--help"]):
            with self.assertRaises(ValueError):
                public.selection(ENV, argv)

    def test_missing_selection_never_uses_a_suite_default(self):
        for key in ENV:
            env = dict(ENV)
            env.pop(key)
            with self.assertRaises(ValueError):
                public.selection(env, ["wrapper"])

    def test_complete_bound_pair_is_only_a_proposal(self):
        self.assertEqual(artifacts.admit_bundle(bind(fixture()), SHA, process(), formatter), "canonical-source-proposals")

    def test_safe_missing_history_prerequisite_remains_publishable_without_adoption(self):
        values = fixture()
        receipt = values["receipt.json"]
        for key in ("beforeState", "afterState", "formatterBefore", "formatterAfter", "count"):
            receipt.pop(key)
        receipt.update(classification="prerequisite-blocked", historyKind="history-anchor-unavailable",
                       failureClass="ValueError", identity={name: False for name in formatter.IDENTITY_FIELDS},
                       formatterPackets=[], processes=receipt["processes"][:2])
        receipt["processes"][1]["exitCode"] = 128
        values["results.json"]["records"] = []
        values["source-manifest.json"].update(state=None, trackedInputs=[], tools=[])
        self.assertEqual(artifacts.admit_bundle(bind(values), SHA, process(2), formatter), "prerequisite-blocked")
        with self.assertRaises(ValueError):
            artifacts.admit_bundle(bind(values), SHA, process(0), formatter)

    def test_json_duplicate_nonfinite_invalid_utf8_or_depth_is_blocked(self):
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b"\xff", b"[" * 1200 + b"]" * 1200):
            with self.assertRaises(ValueError):
                artifacts.safe_json(raw)

    def test_unknown_missing_extra_or_unbound_artifacts_are_blocked(self):
        raw = bind(fixture())
        for data in (raw | {"private.log": b"PRIVATE_AUTH_SENTINEL"},
                     {key: value for key, value in raw.items() if key != "receipt.json"},
                     raw | {"receipt.json": raw["receipt.json"] + b" "}):
            with self.assertRaises(ValueError):
                artifacts.admit_bundle(data, SHA, process(), formatter)

    def test_actual_wrapper_status_and_every_settlement_flag_are_required(self):
        for key, value in (("exitCode", True), ("exitCode", 2), ("stopReason", "external-timeout"),
                           ("ownedProcessExited", False), ("ownedGroupSettled", False), ("captureSettled", False), ("processLaunched", False)):
            with self.assertRaises(ValueError):
                artifacts.admit_bundle(bind(fixture()), SHA, process() | {key: value}, formatter)

    def test_unknown_or_private_receipt_and_packet_fields_are_blocked(self):
        for target in ("receipt", "packet", "source"):
            values = fixture()
            record = values["receipt.json"] if target == "receipt" else values["receipt.json"]["formatterPackets"][0] if target == "packet" else values["source-manifest.json"]
            record["rawAuth"] = "PRIVATE_AUTH_SENTINEL"
            with self.assertRaises(ValueError):
                artifacts.admit_bundle(bind(values), SHA, process(), formatter)

    def test_wrong_revision_candidate_packet_hash_or_after_state_is_blocked(self):
        for change in ("revision", "packet", "after", "records"):
            values = fixture()
            if change == "revision":
                values["receipt.json"]["expectedRevision"] = "f" * 40
            elif change == "packet":
                values["receipt.json"]["formatterPackets"][0]["stdoutSHA256"] = "0" * 64
            elif change == "after":
                values["receipt.json"]["afterState"] = values["receipt.json"]["afterState"] | {"tree": "f" * 40}
            else:
                values["results.json"]["records"].reverse()
                values["results.json"]["records"][1]["path"] = formatter.GO_FILES[1]
            with self.assertRaises(ValueError):
                artifacts.admit_bundle(bind(values), SHA, process(), formatter)

    def test_source_path_duplicate_mode_and_tool_count_boundaries_are_blocked(self):
        for change in ("path", "duplicate", "mode", "tools"):
            values = fixture()
            source = values["source-manifest.json"]
            if change == "path":
                source["trackedInputs"][0]["path"] = "../private"
            elif change == "duplicate":
                source["trackedInputs"].append(copy.deepcopy(source["trackedInputs"][0]))
            elif change == "mode":
                source["trackedInputs"][0]["mode"] = True
            else:
                source["tools"] *= 24
            with self.assertRaises(ValueError):
                artifacts.admit_bundle(bind(values), SHA, process(), formatter)

    def test_summary_capture_never_exports_private_output_or_unknown_fields(self):
        value = dict(phase="canonical-format", classification="canonical-source-proposals", autoAdoption=False)
        projection = public.SummaryProjection()
        projection.consume(encode(value))
        self.assertFalse(projection.prerequisite)
        for raw in (b"PRIVATE_AUTH_SENTINEL", encode(value | {"raw": "PRIVATE_AUTH_SENTINEL"})):
            projection = public.SummaryProjection()
            projection.consume(raw)
            self.assertTrue(projection.prerequisite)
            self.assertNotIn("PRIVATE_AUTH_SENTINEL", repr(projection.result()))
        projection.consume(encode(value))
        self.assertIsNone(projection.result())

    def test_exact_driver_arguments_include_required_fresh_output(self):
        command = public.formatter_argv(SHA, "/private/r18-document-format-0123456789abcdef", {})
        self.assertEqual(command[-4:], ["--expected-revision", SHA, "--output", "/private/r18-document-format-0123456789abcdef"])
        self.assertNotIn("--child", command)
        self.assertNotIn("--work", command)

    def test_fresh_private_name_is_not_created_and_collisions_are_preserved(self):
        with patch.object(files, "open_directory", return_value=100), patch.object(files, "checked_close"), \
                patch.object(files.os, "stat", side_effect=FileNotFoundError), patch.object(files.os, "mkdir") as mkdir:
            result = files.fresh_output("/private", None, lambda: "0" * 32)
            self.assertEqual(str(result), "/private/r18-document-format-" + "0" * 32)
            mkdir.assert_not_called()
        with patch.object(files, "open_directory", return_value=100), patch.object(files, "checked_close"), \
                patch.object(files.os, "stat", return_value=Mock()) as stat:
            with self.assertRaises(ValueError):
                files.fresh_output("/private", None, lambda: "0" * 32)
            self.assertEqual(stat.call_count, 8)

    def test_extra_private_file_rejects_before_any_read(self):
        scanner = MagicMock()
        scanner.__enter__.return_value = iter([SimpleNamespace(name="private.log")])
        with patch.object(files, "open_directory", return_value=100), patch.object(files, "checked_close"), \
                patch.object(files.os, "fstat", return_value=Mock()), patch.object(files.os, "scandir", return_value=scanner), \
                patch.object(files, "read_small") as reader:
            with self.assertRaises(ValueError):
                files.read_bundle("/private", None)
            reader.assert_not_called()

    def test_existing_public_directory_never_writes_or_deletes(self):
        with patch.object(files, "open_directory", return_value=100), patch.object(files, "checked_close"), \
                patch.object(files.os, "mkdir", side_effect=FileExistsError), patch.object(files.os, "open", return_value=101), \
                patch.object(files.os, "write") as writer, patch.object(files.os, "unlink") as unlink:
            with self.assertRaises(FileExistsError):
                files.publish({name: b"x" for name in files.NAMES}, "/root", None)
            writer.assert_not_called()
            unlink.assert_not_called()

    def test_source_capture_rejects_any_changed_size_blob_or_sha_before_import(self):
        data = base64.b64decode(public.READER_BASE64, validate=True)
        pin = public.CODE_PINS[0]
        self.assertEqual(public.verified(data, pin), data)
        for index, value in ((2, "0" * 40), (3, "0" * 64), (4, len(data) + 1)):
            changed = list(pin)
            changed[index] = value
            with self.assertRaises(ValueError):
                public.verified(data, changed)
        with self.assertRaises(ValueError):
            public.verified(data + b"\n", pin)

    def test_summary_duplicate_nonfinite_or_wrong_phase_is_not_complete(self):
        value = dict(phase="canonical-format", classification="canonical-source-proposals", autoAdoption=False)
        projection = public.SummaryProjection()
        projection.consume(encode(value))
        projection.consume(encode(value))
        self.assertTrue(projection.prerequisite)
        for raw in (b'{"phase":"canonical-format","phase":"other"}', b'{"seconds":NaN}',
                    encode(value | {"phase": "compile"})):
            projection = public.SummaryProjection()
            projection.consume(raw)
            self.assertTrue(projection.prerequisite)
            self.assertIsNone(projection.result())

    def test_invalid_admission_never_calls_publisher(self):
        publisher = Mock()
        with self.assertRaises(ValueError):
            artifacts.admit_and_publish({}, SHA, process(), formatter, "/root", 90, publisher)
        publisher.assert_not_called()

    def test_publication_failure_preserves_every_artifact_without_cleanup(self):
        raw, publisher = bind(fixture()), Mock(side_effect=FileExistsError)
        with self.assertRaises(FileExistsError):
            artifacts.admit_and_publish(raw, SHA, process(), formatter, "/root", 90, publisher)
        publisher.assert_called_once_with(raw, "/root", 90)
        self.assertEqual(set(raw), set(artifacts.NAMES))

if __name__ == "__main__":
    unittest.main()
