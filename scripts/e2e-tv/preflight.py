"""Read-only finite TV runtime admission; execute before builds or resource creation."""
import re
import xml.etree.ElementTree as ET
TV_TYPE='com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K'
TV_IMAGE='system-images;android-36;android-tv;x86_64'
def apple_runtime(runtimes, types):
 if not isinstance(runtimes,dict) or not isinstance(types,dict):raise RuntimeError('Invalid TV metadata')
 rs=runtimes.get('runtimes');ts=types.get('devicetypes')
 if not isinstance(rs,list) or not isinstance(ts,list) or len(rs)>128 or len(ts)>512:raise RuntimeError('Invalid TV metadata cardinality')
 if sum(isinstance(t,dict) and t.get('identifier')==TV_TYPE for t in ts)!=1:raise RuntimeError('Required Apple TV type unavailable')
 valid=[r for r in rs if isinstance(r,dict) and r.get('isAvailable') is True and isinstance(r.get('identifier'),str) and re.fullmatch(r'com\.apple\.CoreSimulator\.SimRuntime\.tvOS-27-\d{1,2}(?:-\d{1,2})?',r['identifier']) and isinstance(r.get('version'),str) and re.fullmatch(r'27(?:\.\d{1,2}){1,2}',r['version'])]
 if not valid or len({r['identifier'] for r in valid})!=len(valid):raise RuntimeError('Required tvOS runtime unavailable/ambiguous')
 return max(valid,key=lambda r:tuple(int(n) for n in r['version'].split('.')))['identifier']
def android_image(raw):
 if not isinstance(raw,bytes) or len(raw)>65536 or b'<!' in raw:raise RuntimeError('Invalid TV package XML')
 try:root=ET.fromstring(raw)
 except ET.ParseError:raise RuntimeError('Malformed TV package XML') from None
 packages=[n for n in root.iter() if n.tag.split('}')[-1]=='localPackage']
 if len(packages)!=1 or packages[0].get('path')!=TV_IMAGE:raise RuntimeError('Wrong TV image identity')
 values={key:[(n.text or '').strip() for n in packages[0].iter() if n.tag.split('}')[-1]==key] for key in ('api-level','id','abi')}
 if values!={'api-level':['36'],'id':['android-tv'],'abi':['x86_64']}:raise RuntimeError('Wrong TV image metadata')
 return TV_IMAGE
