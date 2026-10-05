#!/usr/bin/env python3
"""Fixed hosted R18 format wrapper; four admitted JSON exports, no adoption."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import sys
import time
import types

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
CODE_PINS = (
    ("campaign_r18_document_inputs","apps/player/scripts/campaign_r18_document_inputs.py","5e1c5e36abc4ec60fe13030e8a4bce3fc35f6672","c64cfa8f8b85320c3d110c83ca3e7c8745f1ee894eb471fd1276829e0381ae98",6702),
    ("campaign_r18_document_sources","apps/player/scripts/campaign_r18_document_sources.py","c5ce4f7fe606b5005b82b12675c9c46324a83fdb","2a440beb92138f6b3dbf9e70c44d5155f2f92fdcdcb519699c330485d0e9abd4",9032),
    ("campaign_r18_document_events","apps/player/scripts/campaign_r18_document_events.py","8fa40854f7e5e55bdd31eb1bb23ad1c2c903cf17","da42ba31d5e7334e065bb66822d139720dd6d52b1cd887cabe12016cbeaa0a57",7028),
    ("campaign_r18_document_admission","apps/player/scripts/campaign_r18_document_admission.py","8dcfaed7534a87dbd147740e16f9015772a4f6d9","598ffa43a1a9d62388c6048e822f0caa5b7af1e871ad71b7f56d4b3f1144182b",4096),
    ("r18_verified_contract_driver","apps/player/scripts/campaign-r18-document-contract.py","cd3994a3b83677d84f81e708c32d081267154570","257736615cfca467874775c6b562f5059bb797c9848945e8ab6d4911415b8ca4",11663),
    ("campaign_r18_document_format_process","apps/player/scripts/campaign_r18_document_format_process.py","50dd213f5a2c4a7fdeb5de1b8038ec0e97877bc1","4aaec370c9a099a8eb08910f7adce1d1acf24fc6c054ac9ce6d114598442ad07",10254),
    ("campaign_r18_document_format","apps/player/scripts/campaign_r18_document_format.py","7525dbf7e7457c711d25be61760373662b116de1","a451639beefbf1b015617593ddae4debd41fe8de8781b4f6b5601066cc028d64",13771),
    ("campaign_r06_sources","apps/subtitles/scripts/campaign_r06_sources.py","7bb611975c0e42f65780f73d982ca5a35c70f019","6c20ed559d58d13b8bbe9a0cd8e1306b60b05dcd79897cd94b8a92af617172d8",4705),
    ("campaign_r06_browser_process","apps/subtitles/scripts/campaign_r06_browser_process.py","c88075fb514f93209796e79d6dd0bb37b079e7e7","45aa8fe859bde5c0c978589d220e938d6e341e05da5fd6e004c583ada5b4462e",6226),
    ("campaign_r18_format_public_files","apps/player/scripts/campaign_r18_format_public_files.py","6dd31eb59ea881f60ddc568d30c13a15ca31071c","08fba5901277d3fedaf0e8bec52dd8c9a641419f0590409b049c4739a42c5638",4662),
    ("campaign_r18_format_public_artifacts","apps/player/scripts/campaign_r18_format_public_artifacts.py","3eb11f30dda02aca581c84e9e9f775b4363dfb71","11d28fb8f1bf2e7c983ba9f60f2fc6fca3fb42019cd50e9349119518b22731eb",17100),
)
# Exact cleared reader bootstrap; verify current file before every other custom import.
READER_BASE64 = "IiIiRGVzY3JpcHRvci1vd25lZCBpbnB1dCByZWFkcyBhbmQgaW5jcmVtZW50YWwgdG9vbCBpbnZlbnRvcnkgYm91bmRzLiIiIgppbXBvcnQgb3MKZnJvbSBwYXRobGliIGltcG9ydCBQYXRoCmltcG9ydCByZQppbXBvcnQgc3RhdAppbXBvcnQgdGltZQoKRkxBR1MgPSBvcy5PX1JET05MWSB8IG9zLk9fTk9GT0xMT1cgfCBvcy5PX05PTkJMT0NLIHwgb3MuT19DTE9FWEVDCk1BWF9UT09MX0NPVU5UID0gMTI4Ck1BWF9UT09MX0JZVEVTID0gNTEyICogMTAyNCAqIDEwMjQKCgpkZWYgdGljayhkZWFkbGluZSk6CiAgICBpZiBkZWFkbGluZSBpcyBub3QgTm9uZSBhbmQgdGltZS5tb25vdG9uaWMoKSA+PSBkZWFkbGluZToKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJpbnB1dC1kZWFkbGluZSIpCgoKZGVmIGlkZW50aXR5KGluZm8pOgogICAgcmV0dXJuIChpbmZvLnN0X2RldiwgaW5mby5zdF9pbm8sIGluZm8uc3RfbW9kZSwgaW5mby5zdF9zaXplLCBpbmZvLnN0X210aW1lX25zLAogICAgICAgICAgICBpbmZvLnN0X2N0aW1lX25zLCBpbmZvLnN0X3VpZCwgaW5mby5zdF9naWQsIGluZm8uc3RfbmxpbmspCgoKZGVmIGNoZWNrZWRfY2xvc2UoKmRlc2NyaXB0b3JzKToKICAgIHVuY2VydGFpbiA9IEZhbHNlCiAgICBmb3IgZGVzY3JpcHRvciBpbiBkZXNjcmlwdG9yczoKICAgICAgICB0cnk6CiAgICAgICAgICAgIG9zLmNsb3NlKGRlc2NyaXB0b3IpCiAgICAgICAgZXhjZXB0IE9TRXJyb3I6CiAgICAgICAgICAgIHVuY2VydGFpbiA9IFRydWUKICAgIGlmIHVuY2VydGFpbjoKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJkZXNjcmlwdG9yLWNsb3NlLXVuY29uZmlybWVkIikKCgpkZWYgb3Blbl9kaXJlY3RvcnkocGF0aCwgZGVhZGxpbmU9Tm9uZSk6CiAgICBwYXRoID0gUGF0aChwYXRoKQogICAgaWYgbm90IHBhdGguaXNfYWJzb2x1dGUoKSBvciAiLi4iIGluIHBhdGgucGFydHMgb3IgbGVuKHBhdGgucGFydHMpID4gMTI4OgogICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoImRpcmVjdG9yeS1wYXRoLWJvdW5kYXJ5IikKICAgIHRpY2soZGVhZGxpbmUpCiAgICBkZXNjcmlwdG9yID0gb3Mub3BlbihwYXRoLmFuY2hvciwgRkxBR1MgfCBvcy5PX0RJUkVDVE9SWSkKICAgIHRyeToKICAgICAgICBmb3IgcGFydCBpbiBwYXRoLnBhcnRzWzE6XToKICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgbmV3ZXIgPSBvcy5vcGVuKHBhcnQsIEZMQUdTIHwgb3MuT19ESVJFQ1RPUlksIGRpcl9mZD1kZXNjcmlwdG9yKQogICAgICAgICAgICBvbGRlciwgZGVzY3JpcHRvciA9IGRlc2NyaXB0b3IsIG5ld2VyCiAgICAgICAgICAgIGNoZWNrZWRfY2xvc2Uob2xkZXIpCiAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICByZXR1cm4gZGVzY3JpcHRvcgogICAgZXhjZXB0IEJhc2VFeGNlcHRpb246CiAgICAgICAgY2hlY2tlZF9jbG9zZShkZXNjcmlwdG9yKQogICAgICAgIHJhaXNlCgoKZGVmIG9wZW5fcmVndWxhcihwYXRoLCBkZWFkbGluZT1Ob25lKToKICAgIHBhdGggPSBQYXRoKHBhdGgpCiAgICBpZiBub3QgcGF0aC5uYW1lIG9yIHBhdGgubmFtZSBpbiAoIi4iLCAiLi4iKToKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJyZWd1bGFyLXBhdGgtYm91bmRhcnkiKQogICAgcGFyZW50ID0gb3Blbl9kaXJlY3RvcnkocGF0aC5wYXJlbnQsIGRlYWRsaW5lKQogICAgdHJ5OgogICAgICAgIHRpY2soZGVhZGxpbmUpCiAgICAgICAgZGVzY3JpcHRvciA9IG9zLm9wZW4ocGF0aC5uYW1lLCBGTEFHUywgZGlyX2ZkPXBhcmVudCkKICAgICAgICByZXR1cm4gZGVzY3JpcHRvciwgcGFyZW50LCBwYXRoLm5hbWUKICAgIGV4Y2VwdCBCYXNlRXhjZXB0aW9uOgogICAgICAgIGNoZWNrZWRfY2xvc2UocGFyZW50KQogICAgICAgIHJhaXNlCgoKZGVmIHJlZ3VsYXIoaW5mbywgbGltaXQpOgogICAgaWYgbm90IHN0YXQuU19JU1JFRyhpbmZvLnN0X21vZGUpIG9yIHR5cGUoaW5mby5zdF9zaXplKSBpcyBub3QgaW50IG9yIG5vdCAwIDw9IGluZm8uc3Rfc2l6ZSA8PSBsaW1pdDoKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJyZWd1bGFyLXNpemUtYm91bmRhcnkiKQoKCmRlZiB2ZXJpZnlfcGF0aChwYXRoLCBleHBlY3RlZCwgZGVhZGxpbmUpOgogICAgZGVzY3JpcHRvciwgcGFyZW50LCBsZWFmID0gb3Blbl9yZWd1bGFyKHBhdGgsIGRlYWRsaW5lKQogICAgdHJ5OgogICAgICAgIHRpY2soZGVhZGxpbmUpCiAgICAgICAgb2JzZXJ2ZWQgPSBvcy5mc3RhdChkZXNjcmlwdG9yKQogICAgICAgIG5hbWVkID0gb3Muc3RhdChsZWFmLCBkaXJfZmQ9cGFyZW50LCBmb2xsb3dfc3ltbGlua3M9RmFsc2UpCiAgICAgICAgaWYgaWRlbnRpdHkob2JzZXJ2ZWQpICE9IGlkZW50aXR5KGV4cGVjdGVkKSBvciBpZGVudGl0eShuYW1lZCkgIT0gaWRlbnRpdHkoZXhwZWN0ZWQpOgogICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJuYW1lZC1pbnB1dC1jaGFuZ2VkIikKICAgIGZpbmFsbHk6CiAgICAgICAgY2hlY2tlZF9jbG9zZShkZXNjcmlwdG9yLCBwYXJlbnQpCgoKZGVmIHN0cmVhbV9yZWd1bGFyKHBhdGgsIGxpbWl0LCBkZWFkbGluZSwgYmVnaW4sIGNvbnN1bWUpOgogICAgaWYgdHlwZShsaW1pdCkgaXMgbm90IGludCBvciBub3QgMCA8PSBsaW1pdCA8PSAxMjggKiAxMDI0ICogMTAyNDoKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJpbnB1dC1saW1pdCIpCiAgICB0aWNrKGRlYWRsaW5lKQogICAgZGVzY3JpcHRvciwgcGFyZW50LCBsZWFmID0gb3Blbl9yZWd1bGFyKHBhdGgsIGRlYWRsaW5lKQogICAgdHJ5OgogICAgICAgIGJlZm9yZSA9IG9zLmZzdGF0KGRlc2NyaXB0b3IpCiAgICAgICAgcmVndWxhcihiZWZvcmUsIGxpbWl0KQogICAgICAgIG5hbWVkID0gb3Muc3RhdChsZWFmLCBkaXJfZmQ9cGFyZW50LCBmb2xsb3dfc3ltbGlua3M9RmFsc2UpCiAgICAgICAgaWYgaWRlbnRpdHkobmFtZWQpICE9IGlkZW50aXR5KGJlZm9yZSk6CiAgICAgICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoIm9wZW5lZC1pbnB1dC1jaGFuZ2VkIikKICAgICAgICB0aWNrKGRlYWRsaW5lKQogICAgICAgIGJlZ2luKGJlZm9yZSkKICAgICAgICBjb3VudCA9IDAKICAgICAgICB3aGlsZSBUcnVlOgogICAgICAgICAgICB0aWNrKGRlYWRsaW5lKQogICAgICAgICAgICBkYXRhID0gb3MucmVhZChkZXNjcmlwdG9yLCBtaW4oNjU1MzYsIGxpbWl0IC0gY291bnQgKyAxKSkKICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgaWYgbm90IGRhdGE6CiAgICAgICAgICAgICAgICBicmVhawogICAgICAgICAgICBjb3VudCArPSBsZW4oZGF0YSkKICAgICAgICAgICAgaWYgY291bnQgPiBsaW1pdCBvciBjb3VudCA+IGJlZm9yZS5zdF9zaXplOgogICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigiaW5wdXQtZ3Jvd3RoIikKICAgICAgICAgICAgY29uc3VtZShkYXRhKQogICAgICAgIGFmdGVyID0gb3MuZnN0YXQoZGVzY3JpcHRvcikKICAgICAgICBuYW1lZCA9IG9zLnN0YXQobGVhZiwgZGlyX2ZkPXBhcmVudCwgZm9sbG93X3N5bWxpbmtzPUZhbHNlKQogICAgICAgIGlmIGNvdW50ICE9IGJlZm9yZS5zdF9zaXplIG9yIGlkZW50aXR5KGFmdGVyKSAhPSBpZGVudGl0eShiZWZvcmUpIG9yIGlkZW50aXR5KG5hbWVkKSAhPSBpZGVudGl0eShiZWZvcmUpOgogICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJkZXNjcmlwdG9yLWlucHV0LWNoYW5nZWQiKQogICAgICAgIHZlcmlmeV9wYXRoKHBhdGgsIGJlZm9yZSwgZGVhZGxpbmUpCiAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICByZXR1cm4gYmVmb3JlCiAgICBmaW5hbGx5OgogICAgICAgIGNoZWNrZWRfY2xvc2UoZGVzY3JpcHRvciwgcGFyZW50KQoKCmRlZiByZWFkX3NtYWxsKHBhdGgsIGxpbWl0LCBkZWFkbGluZT1Ob25lKToKICAgIGNodW5rcyA9IFtdCiAgICBzdHJlYW1fcmVndWxhcihwYXRoLCBsaW1pdCwgZGVhZGxpbmUsIGxhbWJkYSBfaW5mbzogTm9uZSwgY2h1bmtzLmFwcGVuZCkKICAgIHJldHVybiBiIiIuam9pbihjaHVua3MpCgoKY2xhc3MgQnVkZ2V0OgogICAgZGVmIF9faW5pdF9fKHNlbGYsIGRlYWRsaW5lLCBtYXhfY291bnQ9TUFYX1RPT0xfQ09VTlQsIG1heF9ieXRlcz1NQVhfVE9PTF9CWVRFUyk6CiAgICAgICAgaWYgdHlwZShtYXhfY291bnQpIGlzIG5vdCBpbnQgb3Igbm90IDEgPD0gbWF4X2NvdW50IDw9IE1BWF9UT09MX0NPVU5UOgogICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJ0b29sLWNvdW50LWxpbWl0IikKICAgICAgICBpZiB0eXBlKG1heF9ieXRlcykgaXMgbm90IGludCBvciBub3QgMCA8PSBtYXhfYnl0ZXMgPD0gTUFYX1RPT0xfQllURVM6CiAgICAgICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoInRvb2wtYnl0ZS1saW1pdCIpCiAgICAgICAgc2VsZi5kZWFkbGluZSwgc2VsZi5tYXhfY291bnQsIHNlbGYubWF4X2J5dGVzID0gZGVhZGxpbmUsIG1heF9jb3VudCwgbWF4X2J5dGVzCiAgICAgICAgc2VsZi5jb3VudCwgc2VsZi5ieXRlcyA9IDAsIDAKCiAgICBkZWYgcmVzZXJ2ZShzZWxmLCBzaXplKToKICAgICAgICB0aWNrKHNlbGYuZGVhZGxpbmUpCiAgICAgICAgaWYgdHlwZShzaXplKSBpcyBub3QgaW50IG9yIHNpemUgPCAwIG9yIHNlbGYuY291bnQgKyAxID4gc2VsZi5tYXhfY291bnQgb3Igc2VsZi5ieXRlcyArIHNpemUgPiBzZWxmLm1heF9ieXRlczoKICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1pbnZlbnRvcnktYnVkZ2V0IikKICAgICAgICBzZWxmLmNvdW50ICs9IDEKICAgICAgICBzZWxmLmJ5dGVzICs9IHNpemUKCgpkZWYgZW50cnlfbmFtZShuYW1lKToKICAgIGlmIG5vdCBpc2luc3RhbmNlKG5hbWUsIHN0cikgb3Igbm90IHJlLmZ1bGxtYXRjaChyIltBLVphLXowLTlfLi1dezEsMjU1fSIsIG5hbWUpIG9yIG5hbWUgaW4gKCIuIiwgIi4uIik6CiAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1lbnRyeS1uYW1lIikKICAgIHJldHVybiBuYW1lCgoKZGVmIGl0ZXJfdG9vbF9wYXRocyhkaXJlY3RvcnksIGRlYWRsaW5lKToKICAgIHRpY2soZGVhZGxpbmUpCiAgICB0b3AgPSBvcGVuX2RpcmVjdG9yeShkaXJlY3RvcnksIGRlYWRsaW5lKQogICAgcm9vdHMsIGZpbGVzID0gMCwgMAogICAgdHJ5OgogICAgICAgIHdpdGggb3Muc2NhbmRpcih0b3ApIGFzIGVudHJpZXM6CiAgICAgICAgICAgIGZvciBlbnRyeSBpbiBlbnRyaWVzOgogICAgICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgICAgIHJvb3RzICs9IDEKICAgICAgICAgICAgICAgIGlmIHJvb3RzID4gMzI6CiAgICAgICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1kaXJlY3RvcnktY291bnQiKQogICAgICAgICAgICAgICAgbmFtZSA9IGVudHJ5X25hbWUoZW50cnkubmFtZSkKICAgICAgICAgICAgICAgIGJlZm9yZSA9IG9zLnN0YXQobmFtZSwgZGlyX2ZkPXRvcCwgZm9sbG93X3N5bWxpbmtzPUZhbHNlKQogICAgICAgICAgICAgICAgaWYgc3RhdC5TX0lTUkVHKGJlZm9yZS5zdF9tb2RlKToKICAgICAgICAgICAgICAgICAgICBjb250aW51ZQogICAgICAgICAgICAgICAgaWYgbm90IHN0YXQuU19JU0RJUihiZWZvcmUuc3RfbW9kZSk6CiAgICAgICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1kaXJlY3Rvcnktc2hhcGUiKQogICAgICAgICAgICAgICAgY2hpbGQgPSBvcy5vcGVuKG5hbWUsIEZMQUdTIHwgb3MuT19ESVJFQ1RPUlksIGRpcl9mZD10b3ApCiAgICAgICAgICAgICAgICB0cnk6CiAgICAgICAgICAgICAgICAgICAgaWYgaWRlbnRpdHkob3MuZnN0YXQoY2hpbGQpKSAhPSBpZGVudGl0eShiZWZvcmUpOgogICAgICAgICAgICAgICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJ0b29sLWRpcmVjdG9yeS1jaGFuZ2VkIikKICAgICAgICAgICAgICAgICAgICB3aXRoIG9zLnNjYW5kaXIoY2hpbGQpIGFzIGxlYXZlczoKICAgICAgICAgICAgICAgICAgICAgICAgZm9yIGxlYWYgaW4gbGVhdmVzOgogICAgICAgICAgICAgICAgICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgICAgICAgICAgICAgICAgIGZpbGVzICs9IDEKICAgICAgICAgICAgICAgICAgICAgICAgICAgIGlmIGZpbGVzID4gTUFYX1RPT0xfQ09VTlQ6CiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1lbnRyeS1jb3VudCIpCiAgICAgICAgICAgICAgICAgICAgICAgICAgICBmaWxlbmFtZSA9IGVudHJ5X25hbWUobGVhZi5uYW1lKQogICAgICAgICAgICAgICAgICAgICAgICAgICAgaW5mbyA9IG9zLnN0YXQoZmlsZW5hbWUsIGRpcl9mZD1jaGlsZCwgZm9sbG93X3N5bWxpbmtzPUZhbHNlKQogICAgICAgICAgICAgICAgICAgICAgICAgICAgaWYgbm90IHN0YXQuU19JU1JFRyhpbmZvLnN0X21vZGUpOgogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoInRvb2wtZW50cnktc2hhcGUiKQogICAgICAgICAgICAgICAgICAgICAgICAgICAgeWllbGQgZGlyZWN0b3J5IC8gbmFtZSAvIGZpbGVuYW1lCiAgICAgICAgICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgICAgIGZpbmFsbHk6CiAgICAgICAgICAgICAgICAgICAgY2hlY2tlZF9jbG9zZShjaGlsZCkKICAgICAgICB0aWNrKGRlYWRsaW5lKQogICAgZmluYWxseToKICAgICAgICBjaGVja2VkX2Nsb3NlKHRvcCkK"
CHILD_PROGRAM = "import base64, hashlib, json, pathlib, sys, types\nsys.dont_write_bytecode = True\npins = [[\"campaign_r18_document_inputs\",\"apps/player/scripts/campaign_r18_document_inputs.py\",\"5e1c5e36abc4ec60fe13030e8a4bce3fc35f6672\",\"c64cfa8f8b85320c3d110c83ca3e7c8745f1ee894eb471fd1276829e0381ae98\",6702],[\"campaign_r18_document_sources\",\"apps/player/scripts/campaign_r18_document_sources.py\",\"c5ce4f7fe606b5005b82b12675c9c46324a83fdb\",\"2a440beb92138f6b3dbf9e70c44d5155f2f92fdcdcb519699c330485d0e9abd4\",9032],[\"campaign_r18_document_events\",\"apps/player/scripts/campaign_r18_document_events.py\",\"8fa40854f7e5e55bdd31eb1bb23ad1c2c903cf17\",\"da42ba31d5e7334e065bb66822d139720dd6d52b1cd887cabe12016cbeaa0a57\",7028],[\"campaign_r18_document_admission\",\"apps/player/scripts/campaign_r18_document_admission.py\",\"8dcfaed7534a87dbd147740e16f9015772a4f6d9\",\"598ffa43a1a9d62388c6048e822f0caa5b7af1e871ad71b7f56d4b3f1144182b\",4096],[\"r18_verified_contract_driver\",\"apps/player/scripts/campaign-r18-document-contract.py\",\"cd3994a3b83677d84f81e708c32d081267154570\",\"257736615cfca467874775c6b562f5059bb797c9848945e8ab6d4911415b8ca4\",11663],[\"campaign_r18_document_format_process\",\"apps/player/scripts/campaign_r18_document_format_process.py\",\"50dd213f5a2c4a7fdeb5de1b8038ec0e97877bc1\",\"4aaec370c9a099a8eb08910f7adce1d1acf24fc6c054ac9ce6d114598442ad07\",10254],[\"campaign_r18_document_format\",\"apps/player/scripts/campaign_r18_document_format.py\",\"7525dbf7e7457c711d25be61760373662b116de1\",\"a451639beefbf1b015617593ddae4debd41fe8de8781b4f6b5601066cc028d64\",13771]]\ndata = json.loads(base64.b64decode(sys.argv[1], validate=True))\nroot = pathlib.Path(sys.argv[2]).parents[3]\nif type(data) is not dict or set(data) != {row[0] for row in pins}:\n    raise ValueError(\"captured-child-package\")\nverified = []\nfor name, relative, oid, sha, size in pins:\n    raw = base64.b64decode(data[name], validate=True)\n    actual_oid = hashlib.sha1((\"blob \" + str(len(raw)) + \"\\0\").encode() + raw).hexdigest()\n    if len(raw) != size or actual_oid != oid or hashlib.sha256(raw).hexdigest() != sha:\n        raise ValueError(\"captured-child-pin\")\n    verified.append((name, relative, raw))\nfor name, relative, raw in verified:\n    if name in sys.modules:\n        raise ValueError(\"captured-child-module\")\n    module = types.ModuleType(name)\n    module.__file__ = str(root / relative)\n    sys.modules[name] = module\n    exec(compile(raw, module.__file__, \"exec\"), module.__dict__)\nsys.argv = [sys.argv[2], *sys.argv[3:]]\nsys.exit(sys.modules[\"campaign_r18_document_format\"].main())\n"


def selection(environment, argv):
    if len(argv) != 1 or any(environment.get(key) != value for key, value in
        (("CAMPAIGN_PROOF", "R18"), ("CAMPAIGN_R18_SUITE", "source-format"),
         ("GITHUB_ACTIONS", "true"), ("RUNNER_OS", "Linux"))):
        raise ValueError("selection-prerequisite")
    revision, temporary = environment.get("GITHUB_SHA"), environment.get("RUNNER_TEMP")
    if type(revision) is not str or not re.fullmatch(r"[a-f0-9]{40}", revision):
        raise ValueError("revision-prerequisite")
    if type(temporary) is not str or not 0 < len(temporary) <= 4096 or "\0" in temporary:
        raise ValueError("temporary-prerequisite")
    path = Path(temporary)
    if not path.is_absolute() or ".." in path.parts or str(path) != temporary:
        raise ValueError("temporary-prerequisite")
    return revision, path


def verified(raw, pin):
    _name, _relative, oid, sha, size = pin
    actual = hashlib.sha1(("blob " + str(len(raw)) + "\0").encode() + raw).hexdigest()
    if len(raw) != size or actual != oid or hashlib.sha256(raw).hexdigest() != sha:
        raise ValueError("captured-source-pin")
    return raw


def memory_module(name, relative, raw):
    if name in sys.modules:
        raise ValueError("unexpected-captured-module")
    module = types.ModuleType(name)
    module.__file__ = str(ROOT / relative)
    sys.modules[name] = module
    exec(compile(raw, module.__file__, "exec"), module.__dict__)
    return module


def load_package(deadline):
    first = CODE_PINS[0]
    reader = memory_module(first[0], first[1], verified(base64.b64decode(READER_BASE64, validate=True), first))
    captured = {}
    total = 0
    # Capture every bounded source first; only the immutable reader definitions run early.
    for pin in CODE_PINS:
        reader.tick(deadline)
        raw = verified(reader.read_small(ROOT / pin[1], 32768, deadline), pin)
        total += len(raw)
        if total > 256 * 1024:
            raise ValueError("captured-package-bound")
        captured[pin[0]] = raw
    modules = {first[0]: reader}
    for name, relative, _oid, _sha, _size in CODE_PINS[1:]:
        modules[name] = memory_module(name, relative, captured[name])
    return modules, captured


def formatter_argv(revision, output, captured):
    payload = {name: base64.b64encode(captured[name]).decode()
               for name, _relative, _oid, _sha, _size in CODE_PINS[:7] if name in captured}
    encoded = base64.b64encode(json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()).decode()
    if len(encoded) > 120000:
        raise ValueError("child-argument-bound")
    driver = ROOT / "apps/player/scripts/campaign_r18_document_format.py"
    return [sys.executable, "-B", "-I", "-c", CHILD_PROGRAM, encoded, str(driver),
            "--expected-revision", revision, "--output", str(output)]


class SummaryProjection:
    """Only the driver closed terminal summary survives capture."""
    def __init__(self):
        self.value, self.prerequisite = None, False
        self.bytes, self.lines, self.hash = 0, 0, hashlib.sha256()

    def consume(self, raw):
        self.bytes += len(raw)
        self.lines += 1
        self.hash.update(raw)
        try:
            if self.prerequisite or self.value is not None or self.bytes > 4096 or self.lines != 1:
                raise ValueError("summary-bound")
            value = json.loads(raw.decode("utf-8"), object_pairs_hook=self.pairs, parse_constant=self.constant)
            if type(value) is not dict or set(value) not in (
                {"phase", "classification", "autoAdoption"},
                {"phase", "classification", "autoAdoption", "failureKind"}):
                raise ValueError("summary-shape")
            if value["phase"] != "canonical-format" or value["classification"] not in (
                "canonical-source-proposals", "prerequisite-blocked") or value["autoAdoption"] is not False:
                raise ValueError("summary-state")
            if "failureKind" in value and value["failureKind"] != "formatter-driver-prerequisite":
                raise ValueError("summary-failure")
            self.value = value
        except (ValueError, TypeError, UnicodeError, RecursionError):
            self.value, self.prerequisite = None, True

    @staticmethod
    def pairs(items):
        value = {}
        for key, item in items:
            if key in value:
                raise ValueError("summary-duplicate")
            value[key] = item
        return value

    @staticmethod
    def constant(_value):
        raise ValueError("summary-nonfinite")

    def result(self):
        return None if self.prerequisite else self.value


def main():
    started, published, classification = time.monotonic(), False, "prerequisite-blocked"
    deadline, stage, status = started + 90, "selection", {}
    try:
        revision, temporary = selection(os.environ, sys.argv)
        stage = "captured-imports"
        modules, captured = load_package(deadline)
        reader = modules["campaign_r18_document_inputs"]
        files = modules["campaign_r18_format_public_files"]
        artifacts = modules["campaign_r18_format_public_artifacts"]
        formatter = modules["campaign_r18_document_format"]
        # Existing sanitizer/hosted pin contract only; compiler/run/overlay functions remain unreachable.
        modules["r18_verified_contract_driver"].hosted(types.SimpleNamespace(expected_revision=revision))
        stage = "fresh-private-path"
        output = files.fresh_output(temporary, deadline, lambda: secrets.token_hex(16))
        stage = "owned-formatter"
        projection = SummaryProjection()
        bound = min(72, deadline - time.monotonic() - 18)
        if bound <= 0:
            raise ValueError("wrapper-deadline")
        modules["campaign_r06_browser_process"].execute(
            formatter_argv(revision, output, captured), bound, projection, status, cwd=ROOT / "apps/player")
        summary = projection.result()
        if summary is None:
            raise ValueError("summary-prerequisite")
        stage = "artifact-admission"
        raw = files.read_bundle(output, deadline)
        classification = artifacts.admit_bundle(raw, revision, status, formatter)
        if summary["classification"] != classification:
            raise ValueError("summary-artifact-mismatch")
        reader.tick(deadline)
        stage = "exclusive-publication"
        artifacts.admit_and_publish(raw, revision, status, formatter, ROOT, deadline)
        published = True
    except (OSError, ValueError, KeyError, ImportError, TypeError, RecursionError, KeyboardInterrupt):
        classification = "prerequisite-blocked"
    print(json.dumps({"id": "R18", "suite": "source-format", "classification": classification,
                      "publicationComplete": published, "stage": stage, "autoAdoption": False,
                      "driverExitCode": status.get("exitCode"),
                      "ownedProcessExited": status.get("ownedProcessExited") is True,
                      "ownedGroupSettled": status.get("ownedGroupSettled") is True,
                      "captureSettled": status.get("captureSettled") is True}, sort_keys=True))
    return 0 if published and classification == "canonical-source-proposals" else 2


if __name__ == "__main__":
    sys.exit(main())
