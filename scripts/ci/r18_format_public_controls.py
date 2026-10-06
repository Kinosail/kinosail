#!/usr/bin/env python3
"""Fixed hosted R18 definition-only controls; never run formatter, Go or main."""
import argparse
import copy
import base64
import builtins
from contextlib import contextmanager, ExitStack
import datetime
import hashlib
import io
import json
import math
import ntpath
import os
from pathlib import Path
import re
import secrets
import selectors
import shutil
import signal
import socket
import stat
import subprocess
import sys
import threading
import time
import types
import unittest
from unittest.mock import patch

READER_ALIAS = "r18_controls_capture_reader"
PUBLIC_ALIAS = "r18_public_under_test"
TEST_ALIAS = "r18_fixed_public_controls"
WRAPPER_PIN = (PUBLIC_ALIAS, "apps/player/scripts/campaign-r18-format-public.py",
               "e259e6fed86892e8878c46e09cf8885c81cb9e2b",
               "60c348707a247c177307a6045947cb52b0d0cbdab349ca678f221c8c806fc2a3", 21690)
TEST_PIN = (TEST_ALIAS, "apps/player/scripts/test_campaign_r18_format_public.py",
            "d96242877298aa4c9484146b2dafa18025937c2d",
            "fca791c2a02b19fa418e5c59d16244bc652fd9eb4b303fa007dcb7fdcf0c1a18", 31306)
PACKAGE_PINS = (('campaign_r18_document_inputs', 'apps/player/scripts/campaign_r18_document_inputs.py', '5e1c5e36abc4ec60fe13030e8a4bce3fc35f6672', 'c64cfa8f8b85320c3d110c83ca3e7c8745f1ee894eb471fd1276829e0381ae98', 6702), ('campaign_r18_document_sources', 'apps/player/scripts/campaign_r18_document_sources.py', 'a73b0940fe148d217ec407ec54625528f4aeea83', 'cf304f4bdbdee2fc219fb279dd67bf34c031823a71a9d2940ed2ca8836b5b140', 9032), ('campaign_r18_document_events', 'apps/player/scripts/campaign_r18_document_events.py', '8fa40854f7e5e55bdd31eb1bb23ad1c2c903cf17', 'da42ba31d5e7334e065bb66822d139720dd6d52b1cd887cabe12016cbeaa0a57', 7028), ('campaign_r18_document_admission', 'apps/player/scripts/campaign_r18_document_admission.py', '8dcfaed7534a87dbd147740e16f9015772a4f6d9', '598ffa43a1a9d62388c6048e822f0caa5b7af1e871ad71b7f56d4b3f1144182b', 4096), ('r18_verified_contract_driver', 'apps/player/scripts/campaign-r18-document-contract.py', 'cd3994a3b83677d84f81e708c32d081267154570', '257736615cfca467874775c6b562f5059bb797c9848945e8ab6d4911415b8ca4', 11663), ('campaign_r18_document_format_process', 'apps/player/scripts/campaign_r18_document_format_process.py', 'd63e5f4179261ef47e06406288a70b4a3231b655', '46e2f837ade76fa1ab005a3454ce85edd94b3c4479474518af15bdb64eb1dbf6', 10254), ('campaign_r18_document_format', 'apps/player/scripts/campaign_r18_document_format.py', '4b62fed470e3eb8de54e74027260b4147458d3bd', 'f19f5edb3d45671e6e4504d24d8f96436772e95a59f96373a3d03dacfb34fe53', 13771), ('campaign_r06_sources', 'apps/subtitles/scripts/campaign_r06_sources.py', '7bb611975c0e42f65780f73d982ca5a35c70f019', '6c20ed559d58d13b8bbe9a0cd8e1306b60b05dcd79897cd94b8a92af617172d8', 4705), ('campaign_r06_browser_process', 'apps/subtitles/scripts/campaign_r06_browser_process.py', 'c88075fb514f93209796e79d6dd0bb37b079e7e7', '45aa8fe859bde5c0c978589d220e938d6e341e05da5fd6e004c583ada5b4462e', 6226), ('campaign_r18_format_public_files', 'apps/player/scripts/campaign_r18_format_public_files.py', '6dd31eb59ea881f60ddc568d30c13a15ca31071c', '08fba5901277d3fedaf0e8bec52dd8c9a641419f0590409b049c4739a42c5638', 4662), ('campaign_r18_format_public_artifacts', 'apps/player/scripts/campaign_r18_format_public_artifacts.py', '7dd720f244bf768948fe559ed7d94d3a07cfbfc0', '34e3565e586909e8b27007d2f4bc061edfef72516d4a562253178727661f4c5d', 17037))
READER_BASE64 = "IiIiRGVzY3JpcHRvci1vd25lZCBpbnB1dCByZWFkcyBhbmQgaW5jcmVtZW50YWwgdG9vbCBpbnZlbnRvcnkgYm91bmRzLiIiIgppbXBvcnQgb3MKZnJvbSBwYXRobGliIGltcG9ydCBQYXRoCmltcG9ydCByZQppbXBvcnQgc3RhdAppbXBvcnQgdGltZQoKRkxBR1MgPSBvcy5PX1JET05MWSB8IG9zLk9fTk9GT0xMT1cgfCBvcy5PX05PTkJMT0NLIHwgb3MuT19DTE9FWEVDCk1BWF9UT09MX0NPVU5UID0gMTI4Ck1BWF9UT09MX0JZVEVTID0gNTEyICogMTAyNCAqIDEwMjQKCgpkZWYgdGljayhkZWFkbGluZSk6CiAgICBpZiBkZWFkbGluZSBpcyBub3QgTm9uZSBhbmQgdGltZS5tb25vdG9uaWMoKSA+PSBkZWFkbGluZToKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJpbnB1dC1kZWFkbGluZSIpCgoKZGVmIGlkZW50aXR5KGluZm8pOgogICAgcmV0dXJuIChpbmZvLnN0X2RldiwgaW5mby5zdF9pbm8sIGluZm8uc3RfbW9kZSwgaW5mby5zdF9zaXplLCBpbmZvLnN0X210aW1lX25zLAogICAgICAgICAgICBpbmZvLnN0X2N0aW1lX25zLCBpbmZvLnN0X3VpZCwgaW5mby5zdF9naWQsIGluZm8uc3RfbmxpbmspCgoKZGVmIGNoZWNrZWRfY2xvc2UoKmRlc2NyaXB0b3JzKToKICAgIHVuY2VydGFpbiA9IEZhbHNlCiAgICBmb3IgZGVzY3JpcHRvciBpbiBkZXNjcmlwdG9yczoKICAgICAgICB0cnk6CiAgICAgICAgICAgIG9zLmNsb3NlKGRlc2NyaXB0b3IpCiAgICAgICAgZXhjZXB0IE9TRXJyb3I6CiAgICAgICAgICAgIHVuY2VydGFpbiA9IFRydWUKICAgIGlmIHVuY2VydGFpbjoKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJkZXNjcmlwdG9yLWNsb3NlLXVuY29uZmlybWVkIikKCgpkZWYgb3Blbl9kaXJlY3RvcnkocGF0aCwgZGVhZGxpbmU9Tm9uZSk6CiAgICBwYXRoID0gUGF0aChwYXRoKQogICAgaWYgbm90IHBhdGguaXNfYWJzb2x1dGUoKSBvciAiLi4iIGluIHBhdGgucGFydHMgb3IgbGVuKHBhdGgucGFydHMpID4gMTI4OgogICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoImRpcmVjdG9yeS1wYXRoLWJvdW5kYXJ5IikKICAgIHRpY2soZGVhZGxpbmUpCiAgICBkZXNjcmlwdG9yID0gb3Mub3BlbihwYXRoLmFuY2hvciwgRkxBR1MgfCBvcy5PX0RJUkVDVE9SWSkKICAgIHRyeToKICAgICAgICBmb3IgcGFydCBpbiBwYXRoLnBhcnRzWzE6XToKICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgbmV3ZXIgPSBvcy5vcGVuKHBhcnQsIEZMQUdTIHwgb3MuT19ESVJFQ1RPUlksIGRpcl9mZD1kZXNjcmlwdG9yKQogICAgICAgICAgICBvbGRlciwgZGVzY3JpcHRvciA9IGRlc2NyaXB0b3IsIG5ld2VyCiAgICAgICAgICAgIGNoZWNrZWRfY2xvc2Uob2xkZXIpCiAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICByZXR1cm4gZGVzY3JpcHRvcgogICAgZXhjZXB0IEJhc2VFeGNlcHRpb246CiAgICAgICAgY2hlY2tlZF9jbG9zZShkZXNjcmlwdG9yKQogICAgICAgIHJhaXNlCgoKZGVmIG9wZW5fcmVndWxhcihwYXRoLCBkZWFkbGluZT1Ob25lKToKICAgIHBhdGggPSBQYXRoKHBhdGgpCiAgICBpZiBub3QgcGF0aC5uYW1lIG9yIHBhdGgubmFtZSBpbiAoIi4iLCAiLi4iKToKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJyZWd1bGFyLXBhdGgtYm91bmRhcnkiKQogICAgcGFyZW50ID0gb3Blbl9kaXJlY3RvcnkocGF0aC5wYXJlbnQsIGRlYWRsaW5lKQogICAgdHJ5OgogICAgICAgIHRpY2soZGVhZGxpbmUpCiAgICAgICAgZGVzY3JpcHRvciA9IG9zLm9wZW4ocGF0aC5uYW1lLCBGTEFHUywgZGlyX2ZkPXBhcmVudCkKICAgICAgICByZXR1cm4gZGVzY3JpcHRvciwgcGFyZW50LCBwYXRoLm5hbWUKICAgIGV4Y2VwdCBCYXNlRXhjZXB0aW9uOgogICAgICAgIGNoZWNrZWRfY2xvc2UocGFyZW50KQogICAgICAgIHJhaXNlCgoKZGVmIHJlZ3VsYXIoaW5mbywgbGltaXQpOgogICAgaWYgbm90IHN0YXQuU19JU1JFRyhpbmZvLnN0X21vZGUpIG9yIHR5cGUoaW5mby5zdF9zaXplKSBpcyBub3QgaW50IG9yIG5vdCAwIDw9IGluZm8uc3Rfc2l6ZSA8PSBsaW1pdDoKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJyZWd1bGFyLXNpemUtYm91bmRhcnkiKQoKCmRlZiB2ZXJpZnlfcGF0aChwYXRoLCBleHBlY3RlZCwgZGVhZGxpbmUpOgogICAgZGVzY3JpcHRvciwgcGFyZW50LCBsZWFmID0gb3Blbl9yZWd1bGFyKHBhdGgsIGRlYWRsaW5lKQogICAgdHJ5OgogICAgICAgIHRpY2soZGVhZGxpbmUpCiAgICAgICAgb2JzZXJ2ZWQgPSBvcy5mc3RhdChkZXNjcmlwdG9yKQogICAgICAgIG5hbWVkID0gb3Muc3RhdChsZWFmLCBkaXJfZmQ9cGFyZW50LCBmb2xsb3dfc3ltbGlua3M9RmFsc2UpCiAgICAgICAgaWYgaWRlbnRpdHkob2JzZXJ2ZWQpICE9IGlkZW50aXR5KGV4cGVjdGVkKSBvciBpZGVudGl0eShuYW1lZCkgIT0gaWRlbnRpdHkoZXhwZWN0ZWQpOgogICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJuYW1lZC1pbnB1dC1jaGFuZ2VkIikKICAgIGZpbmFsbHk6CiAgICAgICAgY2hlY2tlZF9jbG9zZShkZXNjcmlwdG9yLCBwYXJlbnQpCgoKZGVmIHN0cmVhbV9yZWd1bGFyKHBhdGgsIGxpbWl0LCBkZWFkbGluZSwgYmVnaW4sIGNvbnN1bWUpOgogICAgaWYgdHlwZShsaW1pdCkgaXMgbm90IGludCBvciBub3QgMCA8PSBsaW1pdCA8PSAxMjggKiAxMDI0ICogMTAyNDoKICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJpbnB1dC1saW1pdCIpCiAgICB0aWNrKGRlYWRsaW5lKQogICAgZGVzY3JpcHRvciwgcGFyZW50LCBsZWFmID0gb3Blbl9yZWd1bGFyKHBhdGgsIGRlYWRsaW5lKQogICAgdHJ5OgogICAgICAgIGJlZm9yZSA9IG9zLmZzdGF0KGRlc2NyaXB0b3IpCiAgICAgICAgcmVndWxhcihiZWZvcmUsIGxpbWl0KQogICAgICAgIG5hbWVkID0gb3Muc3RhdChsZWFmLCBkaXJfZmQ9cGFyZW50LCBmb2xsb3dfc3ltbGlua3M9RmFsc2UpCiAgICAgICAgaWYgaWRlbnRpdHkobmFtZWQpICE9IGlkZW50aXR5KGJlZm9yZSk6CiAgICAgICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoIm9wZW5lZC1pbnB1dC1jaGFuZ2VkIikKICAgICAgICB0aWNrKGRlYWRsaW5lKQogICAgICAgIGJlZ2luKGJlZm9yZSkKICAgICAgICBjb3VudCA9IDAKICAgICAgICB3aGlsZSBUcnVlOgogICAgICAgICAgICB0aWNrKGRlYWRsaW5lKQogICAgICAgICAgICBkYXRhID0gb3MucmVhZChkZXNjcmlwdG9yLCBtaW4oNjU1MzYsIGxpbWl0IC0gY291bnQgKyAxKSkKICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgaWYgbm90IGRhdGE6CiAgICAgICAgICAgICAgICBicmVhawogICAgICAgICAgICBjb3VudCArPSBsZW4oZGF0YSkKICAgICAgICAgICAgaWYgY291bnQgPiBsaW1pdCBvciBjb3VudCA+IGJlZm9yZS5zdF9zaXplOgogICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigiaW5wdXQtZ3Jvd3RoIikKICAgICAgICAgICAgY29uc3VtZShkYXRhKQogICAgICAgIGFmdGVyID0gb3MuZnN0YXQoZGVzY3JpcHRvcikKICAgICAgICBuYW1lZCA9IG9zLnN0YXQobGVhZiwgZGlyX2ZkPXBhcmVudCwgZm9sbG93X3N5bWxpbmtzPUZhbHNlKQogICAgICAgIGlmIGNvdW50ICE9IGJlZm9yZS5zdF9zaXplIG9yIGlkZW50aXR5KGFmdGVyKSAhPSBpZGVudGl0eShiZWZvcmUpIG9yIGlkZW50aXR5KG5hbWVkKSAhPSBpZGVudGl0eShiZWZvcmUpOgogICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJkZXNjcmlwdG9yLWlucHV0LWNoYW5nZWQiKQogICAgICAgIHZlcmlmeV9wYXRoKHBhdGgsIGJlZm9yZSwgZGVhZGxpbmUpCiAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICByZXR1cm4gYmVmb3JlCiAgICBmaW5hbGx5OgogICAgICAgIGNoZWNrZWRfY2xvc2UoZGVzY3JpcHRvciwgcGFyZW50KQoKCmRlZiByZWFkX3NtYWxsKHBhdGgsIGxpbWl0LCBkZWFkbGluZT1Ob25lKToKICAgIGNodW5rcyA9IFtdCiAgICBzdHJlYW1fcmVndWxhcihwYXRoLCBsaW1pdCwgZGVhZGxpbmUsIGxhbWJkYSBfaW5mbzogTm9uZSwgY2h1bmtzLmFwcGVuZCkKICAgIHJldHVybiBiIiIuam9pbihjaHVua3MpCgoKY2xhc3MgQnVkZ2V0OgogICAgZGVmIF9faW5pdF9fKHNlbGYsIGRlYWRsaW5lLCBtYXhfY291bnQ9TUFYX1RPT0xfQ09VTlQsIG1heF9ieXRlcz1NQVhfVE9PTF9CWVRFUyk6CiAgICAgICAgaWYgdHlwZShtYXhfY291bnQpIGlzIG5vdCBpbnQgb3Igbm90IDEgPD0gbWF4X2NvdW50IDw9IE1BWF9UT09MX0NPVU5UOgogICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJ0b29sLWNvdW50LWxpbWl0IikKICAgICAgICBpZiB0eXBlKG1heF9ieXRlcykgaXMgbm90IGludCBvciBub3QgMCA8PSBtYXhfYnl0ZXMgPD0gTUFYX1RPT0xfQllURVM6CiAgICAgICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoInRvb2wtYnl0ZS1saW1pdCIpCiAgICAgICAgc2VsZi5kZWFkbGluZSwgc2VsZi5tYXhfY291bnQsIHNlbGYubWF4X2J5dGVzID0gZGVhZGxpbmUsIG1heF9jb3VudCwgbWF4X2J5dGVzCiAgICAgICAgc2VsZi5jb3VudCwgc2VsZi5ieXRlcyA9IDAsIDAKCiAgICBkZWYgcmVzZXJ2ZShzZWxmLCBzaXplKToKICAgICAgICB0aWNrKHNlbGYuZGVhZGxpbmUpCiAgICAgICAgaWYgdHlwZShzaXplKSBpcyBub3QgaW50IG9yIHNpemUgPCAwIG9yIHNlbGYuY291bnQgKyAxID4gc2VsZi5tYXhfY291bnQgb3Igc2VsZi5ieXRlcyArIHNpemUgPiBzZWxmLm1heF9ieXRlczoKICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1pbnZlbnRvcnktYnVkZ2V0IikKICAgICAgICBzZWxmLmNvdW50ICs9IDEKICAgICAgICBzZWxmLmJ5dGVzICs9IHNpemUKCgpkZWYgZW50cnlfbmFtZShuYW1lKToKICAgIGlmIG5vdCBpc2luc3RhbmNlKG5hbWUsIHN0cikgb3Igbm90IHJlLmZ1bGxtYXRjaChyIltBLVphLXowLTlfLi1dezEsMjU1fSIsIG5hbWUpIG9yIG5hbWUgaW4gKCIuIiwgIi4uIik6CiAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1lbnRyeS1uYW1lIikKICAgIHJldHVybiBuYW1lCgoKZGVmIGl0ZXJfdG9vbF9wYXRocyhkaXJlY3RvcnksIGRlYWRsaW5lKToKICAgIHRpY2soZGVhZGxpbmUpCiAgICB0b3AgPSBvcGVuX2RpcmVjdG9yeShkaXJlY3RvcnksIGRlYWRsaW5lKQogICAgcm9vdHMsIGZpbGVzID0gMCwgMAogICAgdHJ5OgogICAgICAgIHdpdGggb3Muc2NhbmRpcih0b3ApIGFzIGVudHJpZXM6CiAgICAgICAgICAgIGZvciBlbnRyeSBpbiBlbnRyaWVzOgogICAgICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgICAgIHJvb3RzICs9IDEKICAgICAgICAgICAgICAgIGlmIHJvb3RzID4gMzI6CiAgICAgICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1kaXJlY3RvcnktY291bnQiKQogICAgICAgICAgICAgICAgbmFtZSA9IGVudHJ5X25hbWUoZW50cnkubmFtZSkKICAgICAgICAgICAgICAgIGJlZm9yZSA9IG9zLnN0YXQobmFtZSwgZGlyX2ZkPXRvcCwgZm9sbG93X3N5bWxpbmtzPUZhbHNlKQogICAgICAgICAgICAgICAgaWYgc3RhdC5TX0lTUkVHKGJlZm9yZS5zdF9tb2RlKToKICAgICAgICAgICAgICAgICAgICBjb250aW51ZQogICAgICAgICAgICAgICAgaWYgbm90IHN0YXQuU19JU0RJUihiZWZvcmUuc3RfbW9kZSk6CiAgICAgICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1kaXJlY3Rvcnktc2hhcGUiKQogICAgICAgICAgICAgICAgY2hpbGQgPSBvcy5vcGVuKG5hbWUsIEZMQUdTIHwgb3MuT19ESVJFQ1RPUlksIGRpcl9mZD10b3ApCiAgICAgICAgICAgICAgICB0cnk6CiAgICAgICAgICAgICAgICAgICAgaWYgaWRlbnRpdHkob3MuZnN0YXQoY2hpbGQpKSAhPSBpZGVudGl0eShiZWZvcmUpOgogICAgICAgICAgICAgICAgICAgICAgICByYWlzZSBWYWx1ZUVycm9yKCJ0b29sLWRpcmVjdG9yeS1jaGFuZ2VkIikKICAgICAgICAgICAgICAgICAgICB3aXRoIG9zLnNjYW5kaXIoY2hpbGQpIGFzIGxlYXZlczoKICAgICAgICAgICAgICAgICAgICAgICAgZm9yIGxlYWYgaW4gbGVhdmVzOgogICAgICAgICAgICAgICAgICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgICAgICAgICAgICAgICAgIGZpbGVzICs9IDEKICAgICAgICAgICAgICAgICAgICAgICAgICAgIGlmIGZpbGVzID4gTUFYX1RPT0xfQ09VTlQ6CiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgcmFpc2UgVmFsdWVFcnJvcigidG9vbC1lbnRyeS1jb3VudCIpCiAgICAgICAgICAgICAgICAgICAgICAgICAgICBmaWxlbmFtZSA9IGVudHJ5X25hbWUobGVhZi5uYW1lKQogICAgICAgICAgICAgICAgICAgICAgICAgICAgaW5mbyA9IG9zLnN0YXQoZmlsZW5hbWUsIGRpcl9mZD1jaGlsZCwgZm9sbG93X3N5bWxpbmtzPUZhbHNlKQogICAgICAgICAgICAgICAgICAgICAgICAgICAgaWYgbm90IHN0YXQuU19JU1JFRyhpbmZvLnN0X21vZGUpOgogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIHJhaXNlIFZhbHVlRXJyb3IoInRvb2wtZW50cnktc2hhcGUiKQogICAgICAgICAgICAgICAgICAgICAgICAgICAgeWllbGQgZGlyZWN0b3J5IC8gbmFtZSAvIGZpbGVuYW1lCiAgICAgICAgICAgICAgICAgICAgdGljayhkZWFkbGluZSkKICAgICAgICAgICAgICAgIGZpbmFsbHk6CiAgICAgICAgICAgICAgICAgICAgY2hlY2tlZF9jbG9zZShjaGlsZCkKICAgICAgICB0aWNrKGRlYWRsaW5lKQogICAgZmluYWxseToKICAgICAgICBjaGVja2VkX2Nsb3NlKHRvcCkK"
NAMES = tuple(["test_only_exact_hosted_selection_without_cli_options","test_missing_selection_never_uses_a_suite_default","test_complete_bound_pair_is_only_a_proposal","test_safe_missing_history_prerequisite_remains_publishable_without_adoption","test_json_duplicate_nonfinite_invalid_utf8_or_depth_is_blocked","test_unknown_missing_extra_or_unbound_artifacts_are_blocked","test_actual_wrapper_status_and_every_settlement_flag_are_required","test_unknown_or_private_receipt_and_packet_fields_are_blocked","test_wrong_revision_candidate_packet_hash_or_after_state_is_blocked","test_source_path_duplicate_mode_and_tool_count_boundaries_are_blocked","test_summary_capture_never_exports_private_output_or_unknown_fields","test_exact_driver_arguments_include_required_fresh_output","test_fresh_private_name_is_not_created_and_collisions_are_preserved","test_extra_private_file_rejects_before_any_read","test_existing_public_directory_never_writes_or_deletes","test_source_capture_rejects_any_changed_size_blob_or_sha_before_import","test_summary_duplicate_nonfinite_or_wrong_phase_is_not_complete","test_invalid_admission_never_calls_publisher","test_publication_failure_preserves_every_artifact_without_cleanup"])
STDLIB = frozenset(("argparse", "base64", "copy", "datetime", "hashlib", "io", "json", "math", "ntpath", "os",
                   "pathlib", "re", "secrets", "selectors", "shutil", "signal", "socket", "stat",
                   "subprocess", "sys", "threading", "time", "types", "unittest", "unittest.mock"))

STAGES = tuple(["selection","root","reader-definitions","wrapper-capture","test-capture","wrapper-definitions","package-load","package-validation","test-definitions","test-selection","test-execution","complete"])
REASONS = tuple(["none","selection-prerequisite","revision-prerequisite","root-prerequisite","input-deadline","controls-deadline","regular-size-boundary","opened-input-changed","input-growth","descriptor-input-changed","named-input-changed","descriptor-close-unconfirmed","captured-source-size","captured-source-identity","captured-module-boundary","uncaptured-definition-import","package-pins","package-closure","package-module","package-byte-bound","test-class","test-alias","test-method-closure","mock-only-boundary","unexpected-test","duplicate-test","unexpected-terminal","unexpected-stop","directory-path-boundary","regular-path-boundary","input-limit","os-error","import-error","type-error","value-error","runtime-error","recursion-error","interrupted","unclassified","control-results"])

def advance(state, stage):
    if stage not in STAGES: raise ValueError("diagnostic-prerequisite")
    if state is not None: state["stage"] = stage

def error_reason(error):
    args = error.args
    if type(error) in (ValueError, RuntimeError, ImportError) and len(args) == 1 and type(args[0]) is str and args[0] in REASONS[1:31]:
        return args[0]
    for kind, reason in ((RecursionError, "recursion-error"), (KeyboardInterrupt, "interrupted"), (OSError, "os-error"), (ImportError, "import-error"), (TypeError, "type-error"), (ValueError, "value-error"), (RuntimeError, "runtime-error")):
        if isinstance(error, kind): return reason
    return "unclassified"

def selection(environment, argv):
    if len(argv) != 1 or any(environment.get(key) != value for key, value in
        (("CAMPAIGN_PROOF", "R18"), ("CAMPAIGN_R18_SUITE", "source-format"),
         ("GITHUB_ACTIONS", "true"), ("RUNNER_OS", "Linux"))):
        raise ValueError("selection-prerequisite")
    revision = environment.get("GITHUB_SHA")
    if type(revision) is not str or not re.fullmatch(r"[a-f0-9]{40}", revision):
        raise ValueError("revision-prerequisite")
    return revision

def tick(deadline):
    if time.monotonic() >= deadline:
        raise ValueError("controls-deadline")

def verified(raw, pin):
    if type(raw) is not bytes or len(raw) != pin[4]:
        raise ValueError("captured-source-size")
    oid = hashlib.sha1(("blob " + str(len(raw)) + "\0").encode() + raw).hexdigest()
    if oid != pin[2] or hashlib.sha256(raw).hexdigest() != pin[3]:
        raise ValueError("captured-source-identity")
    return raw

def capture(root, pin, reader, deadline):
    if pin not in (WRAPPER_PIN, TEST_PIN):
        raise ValueError("unlisted-capture")
    reader.tick(deadline)
    raw = verified(reader.read_small(root / pin[1], 32768, deadline), pin)
    reader.tick(deadline)
    return raw

def module_from(name, relative, raw, root):
    allowed = {READER_ALIAS: PACKAGE_PINS[0], PUBLIC_ALIAS: WRAPPER_PIN, TEST_ALIAS: TEST_PIN}
    pin = allowed.get(name)
    if pin is None or relative != pin[1] or name in sys.modules:
        raise ValueError("captured-module-boundary")
    verified(raw, pin)
    module = types.ModuleType(name)
    module.__file__ = str(root / relative)
    sys.modules[name] = module
    exec(compile(raw, module.__file__, "exec"), module.__dict__)
    return module

@contextmanager
def definition_imports():
    original = builtins.__import__
    allowed = STDLIB | {pin[0] for pin in PACKAGE_PINS} | {PUBLIC_ALIAS}
    def guarded(name, globals=None, locals=None, fromlist=(), level=0):
        if level != 0 or name not in allowed or name not in sys.modules:
            raise ImportError("uncaptured-definition-import")
        return original(name, globals, locals, fromlist, level)
    with patch.object(builtins, "__import__", guarded):
        yield

def validate_package(root, public, modules, captured):
    wanted = {pin[0] for pin in PACKAGE_PINS}
    if public.CODE_PINS != PACKAGE_PINS or type(modules) is not dict or type(captured) is not dict:
        raise ValueError("package-pins")
    if set(modules) != wanted or set(captured) != wanted:
        raise ValueError("package-closure")
    total = 0
    for pin in PACKAGE_PINS:
        raw, module = verified(captured[pin[0]], pin), modules[pin[0]]
        total += len(raw)
        if type(module) is not types.ModuleType or module.__name__ != pin[0] or module.__file__ != str(root / pin[1]):
            raise ValueError("package-module")
    if total > 256 * 1024:
        raise ValueError("package-byte-bound")

def prepare(root, deadline, state=None):
    advance(state, "root")
    if not isinstance(root, Path) or not root.is_absolute() or ".." in root.parts:
        raise ValueError("root-prerequisite")
    with definition_imports():
        advance(state, "reader-definitions")
        reader_raw = verified(base64.b64decode(READER_BASE64, validate=True), PACKAGE_PINS[0])
        reader = module_from(READER_ALIAS, PACKAGE_PINS[0][1], reader_raw, root)
        advance(state, "wrapper-capture")
        wrapper_raw = capture(root, WRAPPER_PIN, reader, deadline)
        advance(state, "test-capture")
        test_raw = capture(root, TEST_PIN, reader, deadline)
        advance(state, "wrapper-definitions")
        public = module_from(PUBLIC_ALIAS, WRAPPER_PIN[1], wrapper_raw, root)
        advance(state, "package-load")
        modules, captured = public.load_package(deadline)
        advance(state, "package-validation")
        validate_package(root, public, modules, captured)
        reader.tick(deadline)
        advance(state, "test-definitions")
        return module_from(TEST_ALIAS, TEST_PIN[1], test_raw, root)

def select_tests(module):
    target = getattr(module, "PublicControls", None)
    if module.__name__ != TEST_ALIAS or not isinstance(target, type) or not issubclass(target, unittest.TestCase):
        raise ValueError("test-class")
    if target.__name__ != "PublicControls" or target.__module__ != TEST_ALIAS:
        raise ValueError("test-alias")
    names = unittest.TestLoader().getTestCaseNames(target)
    if len(names) != 19 or set(names) != set(NAMES):
        raise ValueError("test-method-closure")
    return target

def denied(*_args, **_kwargs):
    raise RuntimeError("mock-only-boundary")

@contextmanager
def mock_only():
    # Tests may replace these defaults with mocks; an unmocked action always rejects.
    groups = (
        (builtins, ("open",)), (io, ("open",)),
        (os, ("open", "read", "write", "close", "stat", "lstat", "fstat", "scandir", "listdir",
              "mkdir", "makedirs", "unlink", "remove", "rmdir", "rename", "replace", "chdir",
              "chmod", "chown", "link", "symlink", "truncate", "utime", "system", "popen",
              "fork", "forkpty", "posix_spawn", "posix_spawnp", "execv", "execve", "kill", "killpg")),
        (subprocess, ("Popen", "run", "call", "check_call", "check_output")),
        (socket, ("socket", "create_connection")), (threading.Thread, ("start",)),
        (time, ("sleep",)),
    )
    with ExitStack() as stack:
        for owner, attributes in groups:
            for name in attributes:
                if hasattr(owner, name):
                    stack.enter_context(patch.object(owner, name, denied))
        yield

class SafeResult(unittest.TestResult):
    """Track parents/subtests without formatting or retaining private exceptions."""
    def __init__(self, deadline):
        super().__init__()
        self.deadline, self.records, self.stopped = deadline, {}, set()

    def name(self, test):
        name = getattr(test, "_testMethodName", None)
        if name not in NAMES:
            raise ValueError("unexpected-test")
        return name

    def startTest(self, test):
        tick(self.deadline)
        name = self.name(test)
        if name in self.records:
            raise ValueError("duplicate-test")
        self.records[name] = "started"
        super().startTest(test)

    def mark(self, test, status, subtest=False):
        name = self.name(test)
        before = self.records.get(name)
        if (before != "started" and not (subtest and before in ("fail", "error"))) or name in self.stopped:
            raise ValueError("unexpected-terminal")
        self.records[name] = "error" if before == "error" else status

    def addSuccess(self, test):
        self.mark(test, "pass")

    def addFailure(self, test, _error):
        self.mark(test, "fail")

    def addError(self, test, _error):
        self.mark(test, "error")

    def addSkip(self, test, _reason):
        self.mark(test, "skip")

    def addExpectedFailure(self, test, _error):
        self.mark(test, "fail")

    def addUnexpectedSuccess(self, test):
        self.mark(test, "fail")

    def addSubTest(self, test, _subtest, error):
        if error is not None:
            self.mark(test, "fail" if issubclass(error[0], test.failureException) else "error", subtest=True)

    def stopTest(self, test):
        tick(self.deadline)
        name = self.name(test)
        if name not in self.records or name in self.stopped:
            raise ValueError("unexpected-stop")
        self.stopped.add(name)
        super().stopTest(test)

    def report(self):
        complete = (len(self.records) == 19 and self.stopped == set(NAMES)
                    and all(status in ("pass", "fail", "error", "skip") for status in self.records.values())
                    and time.monotonic() < self.deadline)
        green = complete and all(status == "pass" for status in self.records.values())
        return {"schemaVersion": 1, "id": "R18", "suite": "source-format", "phase": "source-format-controls",
                "classification": "source-controls-green" if green else "controls-failed" if complete else "prerequisite-blocked",
                "count": len(self.records), "passed": sum(value == "pass" for value in self.records.values()),
                "failed": sum(value == "fail" for value in self.records.values()),
                "errors": sum(value == "error" for value in self.records.values()),
                "skipped": sum(value == "skip" for value in self.records.values()),
                "records": [{"test": name, "status": self.records[name] if self.records[name] != "started" else "incomplete"}
                            for name in NAMES if name in self.records],
                "formatterExecuted": False, "goExecuted": False, "autoAdoption": False, "semanticsVerified": False}

def run_controls(target, deadline):
    result = SafeResult(deadline)
    with mock_only():
        unittest.TestSuite(target(name) for name in NAMES).run(result)
    return result.report()

def main():
    deadline, report = time.monotonic() + 20, SafeResult(float("inf")).report()
    saved_path, saved_bytecode = list(sys.path), sys.dont_write_bytecode
    state, reason = {"stage": "selection"}, "none"
    try:
        selection(os.environ, sys.argv)
        sys.dont_write_bytecode = True
        advance(state, "root")
        module = prepare(Path(__file__).absolute().parents[2], deadline, state)
        advance(state, "test-selection")
        target = select_tests(module)
        advance(state, "test-execution")
        report = run_controls(target, deadline)
        advance(state, "complete")
        reason = "control-results"
    except (Exception, KeyboardInterrupt) as error:
        reason = error_reason(error)
    finally:
        sys.path[:] = saved_path
        sys.dont_write_bytecode = saved_bytecode
    if report["classification"] == "prerequisite-blocked":
        report["diagnostic"] = {"stage": state["stage"], "reason": reason}
    print(json.dumps(report, sort_keys=True, allow_nan=False))
    return 0 if report["classification"] == "source-controls-green" else 2

if __name__ == "__main__":
    sys.exit(main())
