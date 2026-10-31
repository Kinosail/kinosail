"""Disposable loopback TV QA: large catalog and loaded/pending/empty/failed states."""
import colorsys, json, math, os, re, sys, time, zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit, parse_qs
from PIL import Image, ImageDraw
from io import BytesIO
STATE = {'mode': 'loaded', 'delay': 0, 'target': 'library', 'artworkUntil': 0}
PLAY = dict(rate=1, audioLanguage='auto', subtitleLanguage='auto', audioTrack='', subtitleTrack='', nightMode=False, dialogueBoost=False, volumeBoost=1)
VIEWER = {'server':'TV Polish QA','serverId':'tv-polish-qa','viewer':dict(id='qa',name='QA Viewer',owner=True,downloads=True,transcode=True,remote=False)}
PROGRESS = dict(seconds=0,watched=False,session='',revision=0)
TITLE_PREFS = {}
SHOW_LISTED = {'value': False}
def read_movie(args):
    if len(args) != 1 or not args[0] or len(args[0]) > 4096:
        raise ValueError('One generated media path is required')
    path = Path(args[0]).resolve(strict=True)
    cap = 32 * 1024 * 1024
    if not path.is_file() or not 0 < path.stat().st_size <= cap:
        raise ValueError('Invalid fixture media')
    with path.open('rb') as stream:
        data = stream.read(cap + 1)
    if not 0 < len(data) <= cap:
        raise ValueError('Invalid fixture media')
    return data

try:
    MOVIE = read_movie(sys.argv[1:])
except (ValueError, OSError):
    raise SystemExit('Supply one generated, nonempty media file of at most 32 MiB.')
TITLES = ['Night Sky','The Wild Coast','Moonrise','An Extraordinary Journey Across the Quiet Mountains','After the Rain','Before Dawn']
ITEMS = []
for n in range(160):
    title = TITLES[n] if n < len(TITLES) else f'Journey {n:03d}'
    ITEMS.append(dict(id=f'movie-{n:03d}',kind='video',title=title,year='2026',rating='PG',genres=['Adventure','Drama','Nature','Mystery'][n%4],plot='A quiet journey through unfamiliar landscapes. An ordinary evening turns into an extraordinary adventure.',artwork=f'/art/movie-{n:03d}',backdrop=f'/backdrop/movie-{n:03d}',stream=f'/media/movie-{n:03d}',container='mp4',progress={**PROGRESS,'seconds':3 if n<6 else 0,'session':'qa' if n<6 else '', 'revision':1 if n<6 else 0}))
if os.environ.get('KINOSAIL_TV_ARTWORK_CAST_FIXTURE') == '1':
    next(item for item in ITEMS if item['id'] == 'movie-004')['cast'] = [
        dict(name=f'Artwork QA Actor {n:02d}', role=f'Fixture role {n:02d}', image=f'/art/artwork-cast-{n:02d}')
        for n in range(1, 13)
    ]
ITEMS.append(dict(id='0123456789abcdef',kind='show',title='Into the Wild',artwork='/art/show',backdrop='/backdrop/show',genres='Nature',plot='A journey into the natural world.',progress=PROGRESS))
for n in range(4):
    ITEMS.append(dict(id=f'episode-{n}',kind='video',title=['First Light','The Long Way Home','A Quiet Evening','The Last Horizon'][n],plot=['Dawn breaks over the river valley.','The herd crosses the high pass.','Rain settles over the forest.','The journey ends at the coast.'][n],show='Into the Wild',showId='0123456789abcdef',season=1 if n<2 else 2,episode=n%2+1,rating='PG',artwork=f'/episode-art/episode-{n}',backdrop='/backdrop/show',stream=f'/media/episode-{n}',container='mp4',progress=PROGRESS))
for kind,title in [('music','Evening Light'),('audiobook','The Geometry of Sound'),('photo','Color Study'),('book','Field Notes')]:
    ITEMS.append(dict(id=kind,kind=kind,title=title,artwork='/art/'+kind,stream='/media/'+kind,artist='Signal Lab' if kind=='music' else '',container='mp4',progress=PROGRESS))
CAPTIONS = b'WEBVTT\n\n00:00:01.000 --> 00:00:06.000\nThe river bends toward the mountains.\n\n00:00:12.000 --> 00:00:18.000\nWe should reach the ridge by nightfall.\n'
def extras_enabled(): return os.environ.get('KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE') == '1'
def playback_extras(ident):
    episode = re.fullmatch(r'episode-([0-3])',ident)
    if not extras_enabled() or not episode: return dict(chapters=[],markers=[],subtitles=[],next='')
    n = int(episode.group(1))
    return dict(chapters=[dict(title='Opening',start=0,end=10),dict(title='Journey',start=10,end=24),dict(title='Credits',start=24,end=30)],
                markers=[dict(type='intro',label='',start=2,end=12),dict(type='credits',label='',start=24,end=30)],
                subtitles=[dict(label='English',language='en',source=f'/subtitle/{ident}/en.vtt',default=False)],next=f'episode-{n+1}' if n < 3 else '')
def art_label(path):
    name=path.split('/')[-1]
    item=next((i for i in ITEMS if i['id']==name and i.get('showId')),None)
    return f"S{item['season']} E{item['episode']}" if item and path.startswith('/episode-art/') else name.replace('-',' ').upper()
ART = {}
def artwork(path):
    if path not in ART:
        landscape=path.startswith(('/backdrop/','/episode-art/'))
        w,h=(1600,900) if landscape else (900,900) if path.endswith(('/music','/audiobook')) else (600,900)
        hue=.58+zlib.crc32(path.split("/")[-1].encode())%100/100*.14
        top,bottom,ridge=(tuple(int(c*255) for c in colorsys.hsv_to_rgb(hue,s,v)) for s,v in [(.72,.45),(.55,.78),(.6,.22)])
        img=Image.new('RGB',(w,h),top); draw=ImageDraw.Draw(img)
        for y in range(h): draw.line((0,y,w,y),fill=tuple(int(a+(b-a)*y/h) for a,b in zip(top,bottom)))
        draw.ellipse((w*.57,h*.17,w*.88,h*.48),fill=(216,218,233))
        draw.polygon([(0,h*.82),(w*.35,h*.36),(w*.68,h*.88),(w,h*.55),(w,h),(0,h)],fill=ridge)
        draw.text((36,h-90),art_label(path),fill='white',font_size=38)
        data=BytesIO(); img.save(data,'JPEG',quality=90); ART[path]=data.getvalue()
    return ART[path]
class Handler(BaseHTTPRequestHandler):
    def log_message(self,fmt,*args): print(self.command,urlsplit(self.path).path,flush=True)
    def send(self,code,value=None,kind='application/json',headers=None):
        data=json.dumps(value).encode() if kind=='application/json' else value or b''
        self.send_response(code); self.send_header('Content-Type',kind); self.send_header('Content-Length',str(len(data))); self.send_header('Cache-Control','no-store')
        for key,val in (headers or {}).items(): self.send_header(key,val)
        self.end_headers()
        try:
            self.wfile.write(data)
            if code==404 and urlsplit(self.path).path=='/backdrop/movie-005':
                print('ARTWORK_FIXTURE_RESPONSE',json.dumps(dict(route='missing-backdrop',status=404,completed=True,unixTime=time.time())),flush=True)
        except (BrokenPipeError,ConnectionResetError): pass
    def do_GET(self):
        if len(self.path) > 4096: return self.send(400, {'error':'invalid fixture target'})
        p=urlsplit(self.path); path=p.path; q=parse_qs(p.query)
        if path=='/qa/health': return self.send(200,{'status':'ready','state':dict(STATE)})
        if path=='/qa/state':
            try:
                if re.search(r'%(?![0-9a-fA-F]{2})', p.query): raise ValueError()
                q=parse_qs(p.query,keep_blank_values=True,strict_parsing=True,max_num_fields=5,errors='strict')
                if not set(q) <= {'mode','delay','target','profile','identity'} or any(len(v)!=1 for v in q.values()): raise ValueError()
                mode=q.get('mode',['loaded'])[0]; delay=float(q.get('delay',['0'])[0]); target=q.get('target',['library'])[0]
                identity=q.get('identity',[VIEWER['serverId']])[0]
                if mode not in ['loaded','empty','failed'] or target not in ['library','details','shows','playback','media','connection','artwork']: raise ValueError()
                if target=='connection' and (mode=='empty' or delay!=0): raise ValueError()
                if target=='artwork' and (mode!='failed' or not 0 < delay <= 90): raise ValueError()
                if not math.isfinite(delay) or not 0 <= delay <= (90 if target=='artwork' else 30) or q.get('profile',['qa'])[0] != 'qa': raise ValueError()
                if not re.fullmatch(r'tv-polish-qa(?:-[a-z0-9-]{1,64})?',identity): raise ValueError()
            except (ValueError, UnicodeError): return self.send(400,{'error':'invalid fixture state'})
            STATE.update(mode=mode,delay=delay,target=target,artworkUntil=time.time()+delay if target=='artwork' else 0); VIEWER['serverId']=identity
            return self.send(200,STATE)
        if self.headers.get('Authorization')!='Bearer tv-polish-fixture-token': return self.send(401,{'error':'fixture token required'})
        if STATE['target']=='connection' and STATE['mode']=='failed':
            self.close_connection=True
            return
        if path=='/api/v1/me': return self.send(200,VIEWER)
        active=(STATE['target']=='library' and path=='/api/v1/library') or (STATE['target']=='details' and re.fullmatch(r'/api/v1/items/[^/]+',path)) or (STATE['target']=='shows' and path.startswith('/api/v1/shows/')) or (STATE['target']=='playback' and path.endswith('/playback'))
        mode,delay=STATE['mode'],STATE['delay']
        if active:
            time.sleep(delay)
            if mode=='failed': return self.send(500,{'error':'simulated failure'})
        if path=='/api/v1/library':
            view=q.get('view',['all'])[0]; sort=q.get('sort',['title'])[0]; query=q.get('q',[''])[0]
            entries=[] if active and mode=='empty' else ITEMS
            if view=='history': entries=[i for i in entries if i['progress']['seconds']>0]
            elif view=='shows': entries=[i for i in entries if i['kind']=='show']
            elif view=='movies': entries=[i for i in entries if i['kind']=='video' and not i.get('showId')]
            elif view in ['music','audiobooks','photos','books']: entries=[i for i in entries if i['kind']=={'audiobooks':'audiobook','photos':'photo','books':'book'}.get(view,view)]
            elif view=='list': entries=[]
            entries=[i for i in entries if query.lower() in i['title'].lower()]
            if sort=='title': entries=sorted(entries,key=lambda i:i['title'])
            letters=[]
            if sort=='title' and not query:
                for index,item in enumerate(entries):
                    label=item['title'][0].upper()
                    if not letters or letters[-1]['label']!=label:
                        letters.append(dict(label=label,offset=index,count=0))
                    letters[-1]['count']+=1
            offset=int(q.get('offset',['0'])[0]); limit=int(q.get('limit',['60'])[0])
            return self.send(200,dict(items=entries[offset:offset+limit],total=len(entries),offset=offset,limit=limit,letters=letters,view=view,sort=sort,query=query))
        if path.startswith(('/art/','/backdrop/','/episode-art/')):
            if path.endswith('movie-005'): return self.send(404,{'error':'missing image'})
            if path.startswith('/art/') and time.time()<STATE['artworkUntil']:
                print('ARTWORK_OUTAGE_RESPONSE',json.dumps(dict(path=path,status=404,unixTime=time.time())),flush=True)
                return self.send(404,{'error':'simulated artwork outage'})
            time.sleep(.1); return self.send(200,artwork(path),'image/jpeg')
        if path=='/api/v1/albums': return self.send(200,{'albums':[dict(id='evening',title='Evening Light',artist='Signal Lab',artwork='/art/music')]})
        if path=='/api/v1/albums/evening': return self.send(200,dict(id='evening',title='Evening Light',artist='Signal Lab',tracks=[next(i for i in ITEMS if i['id']=='music')]))
        if path=='/api/v1/collections': return self.send(200,{'collections':['Weekend']})
        if path=='/api/v1/collections/Weekend': return self.send(200,dict(name='Weekend',items=ITEMS[:6]))
        if path=='/api/v1/actor' and os.environ.get('KINOSAIL_TV_ARTWORK_CAST_FIXTURE') == '1':
            name=q.get('name',[''])[0]
            person=next((p for p in ITEMS[4].get('cast',[]) if p['name']==name),None)
            if not person: return self.send(404,{'error':'missing fixture actor'})
            return self.send(200,dict(name=name,image=person['image'],movies=[],shows=[]))
        if path=='/api/v1/shows/0123456789abcdef/list': return self.send(200,{'listed':SHOW_LISTED['value']})
        if path=='/api/v1/shows': return self.send(200,{'shows':[dict(id='0123456789abcdef',title='Into the Wild',artwork='/art/show',backdrop='/backdrop/show')] if SHOW_LISTED['value'] or q.get('listed',['false'])[0]!='true' else []})
        if path=='/api/v1/shows/0123456789abcdef': return self.send(200,dict(id='0123456789abcdef',title='Into the Wild',backdrop='/backdrop/show',plot='A journey into the natural world.',episodes=[i for i in ITEMS if i.get('showId')],genres='Nature',cast=[]))
        if path=='/api/v1/me/media-preferences': return self.send(200,dict(playback=PLAY,autoDownloadNext=0,removeWatched=False,downloadLimitGiB=20,wifiOnly=True,readerFontSize=20,readerTheme='auto'))
        match=re.fullmatch(r'/api/v1/items/([^/]+)(.*)',path)
        if match:
            ident,suffix=match.groups(); item=next((i for i in ITEMS if i['id']==ident),None)
            if not item: return self.send(404,{'error':'missing title'})
            if not suffix: return self.send(200,dict(item={**item,'backdrop':''} if active and mode=='empty' else item,listed=False,profileId='qa'))
            if suffix=='/playback': return self.send(200,dict(media=dict(kind=item['kind'],duration=30),plan=dict(allowed=True,mode='direct',reason='compatible'),directAllowed=True,direct='/media/'+ident,directType='video/mp4',duration=30,start=item['progress']['seconds'],**playback_extras(ident)))
            if suffix=='/playback-preferences': return self.send(200,dict(playback=TITLE_PREFS.get(ident,PLAY),overridden=ident in TITLE_PREFS))
            if suffix=='/watch-progress': return self.send(200,dict(seconds=item['progress']['seconds'],duration=30))
            if suffix=='/bookmarks': return self.send(200,{'bookmarks':[]})
        if path.startswith('/subtitle/'):
            if not extras_enabled() or not re.fullmatch(r'/subtitle/episode-[0-3]/en\.vtt',path): return self.send(404,{'error':'missing subtitle'})
            return self.send(200,CAPTIONS,'text/vtt')
        if path.startswith('/media/'):
            if STATE['target']=='media': time.sleep(STATE['delay'])
            if path.endswith('photo'): return self.send(200,artwork('/backdrop/photo'),'image/jpeg')
            data=MOVIE; headers={'Accept-Ranges':'bytes'}; code=200
            if self.headers.get('Range'):
                match=re.fullmatch(r'bytes=([0-9]{0,20})-([0-9]{0,20})',self.headers['Range'])
                if not match or not any(match.groups()): return self.send(416,{'error':'invalid fixture range'})
                first,last=match.groups()
                if not first:
                    if int(last)==0: return self.send(416,{'error':'invalid fixture range'})
                    start=max(0,len(data)-int(last)); end=len(data)-1
                else:
                    start=int(first); end=int(last) if last else len(data)-1
                if start>=len(data) or end<start: return self.send(416,{'error':'invalid fixture range'})
                end=min(end,len(data)-1)
                headers['Content-Range']=f'bytes {start}-{end}/{len(data)}'; data=data[start:end+1]; code=206
            return self.send(code,data,'video/mp4',headers)
        return self.send(404,{'error':'unsupported fixture operation'})
    def read_body(self):
        lengths=self.headers.get_all('Content-Length',[])
        if len(lengths)!=1 or self.headers.get_all('Transfer-Encoding',[]): raise ValueError()
        if not re.fullmatch(r'[0-9]{1,5}',lengths[0]) or not 0<int(lengths[0])<=16384: raise ValueError()
        return json.loads(self.rfile.read(int(lengths[0])))
    def title_preferences(self):
        if self.headers.get_all('Authorization',[])!=['Bearer tv-polish-fixture-token']: return None,self.send(401,{'error':'fixture token required'})
        match=re.fullmatch(r'/api/v1/items/(movie-[0-9]{3})/playback-preferences',self.path)
        if not match: return None,self.send(404,{'error':'unsupported fixture operation'})
        return match.group(1),None
    def do_DELETE(self):
        ident,sent=self.title_preferences()
        if not ident: return sent
        TITLE_PREFS.pop(ident,None)
        return self.send(200,dict(playback=PLAY,overridden=False))
    def do_PUT(self):
        if STATE['target']=='connection' and STATE['mode']=='failed' and self.headers.get_all('Authorization',[])==['Bearer tv-polish-fixture-token']:
            self.close_connection=True
            return
        if self.path.endswith('/playback-preferences'):
            ident,sent=self.title_preferences()
            if not ident: return sent
            try:
                body=self.read_body()
                if not isinstance(body,dict) or set(body)!=set(PLAY): raise ValueError()
                if type(body['rate']) not in [int,float] or not 0.5<=body['rate']<=3: raise ValueError()
                if type(body['volumeBoost']) not in [int,float] or not 0<=body['volumeBoost']<=4: raise ValueError()
                if any(type(body[k]) is not bool for k in ['nightMode','dialogueBoost']): raise ValueError()
                if any(not isinstance(body[k],str) or len(body[k])>64 for k in ['audioLanguage','subtitleLanguage','audioTrack','subtitleTrack']): raise ValueError()
            except (ValueError,KeyError,TypeError,RecursionError): return self.send(400,{'error':'invalid fixture preferences'})
            TITLE_PREFS[ident]=body
            print('PREFERENCES_FIXTURE_SAVED',json.dumps(dict(item=ident,rate=body['rate'])),flush=True)
            return self.send(200,dict(playback=body,overridden=True))
        change=re.fullmatch(r'/api/v1/(items|shows)/([a-z0-9-]{1,32})/(watched|list)',self.path)
        if change:
            if self.headers.get_all('Authorization',[])!=['Bearer tv-polish-fixture-token']: return self.send(401,{'error':'fixture token required'})
            kind,ident,action=change.groups()
            try:
                body=self.read_body(); key='listed' if action=='list' else 'watched'
                if not isinstance(body,dict) or key not in body or type(body[key]) is not bool or set(body)-{key,'season'}: raise ValueError()
                if 'season' in body and (kind!='shows' or type(body['season']) is not int): raise ValueError()
            except (ValueError,KeyError,TypeError,RecursionError): return self.send(400,{'error':'invalid fixture change'})
            if action=='list':
                if kind!='shows' or ident!='0123456789abcdef': return self.send(404,{'error':'unsupported fixture operation'})
                SHOW_LISTED['value']=body['listed']; print('SHOW_LIST_FIXTURE_SAVED',json.dumps(body),flush=True)
                return self.send(200,{'listed':body['listed']})
            targets=[i for i in ITEMS if (i['id']==ident if kind=='items' else i.get('showId')==ident and body.get('season',i.get('season'))==i.get('season'))]
            if not targets: return self.send(404,{'error':'missing title'})
            for item in targets: item['progress']={**item['progress'],'seconds':0,'watched':body['watched']}
            print('WATCHED_FIXTURE_SAVED',json.dumps(dict(kind=kind,items=len(targets),watched=body['watched'])),flush=True)
            return self.send(200,{'watched':body['watched'],'items':len(targets)})
        if not re.fullmatch(r'/api/v1/items/(?:movie-[0-9]{3}|episode-[0-3])/progress/sync',self.path): return self.send(404,{'error':'unsupported fixture operation'})
        if self.headers.get_all('Authorization',[])!=['Bearer tv-polish-fixture-token']: return self.send(401,{'error':'fixture token required'})
        try:
            body=self.read_body()
            if not isinstance(body,dict) or set(body)!={'progress','expected','playbackToken'}: raise ValueError()
            for value in [body['progress'],body['expected']]:
                if not isinstance(value,dict) or set(value)!={'seconds','watched','session','revision'}: raise ValueError()
                if type(value['seconds']) not in [int,float] or not math.isfinite(value['seconds']) or not 0<=value['seconds']<=31536000: raise ValueError()
                if type(value['watched']) is not bool or type(value['revision']) is not int or not 0<=value['revision']<=9007199254740991: raise ValueError()
                if not isinstance(value['session'],str) or len(value['session'].encode())>128: raise ValueError()
            if not isinstance(body['playbackToken'],str) or len(body['playbackToken'].encode())>8192: raise ValueError()
        except (ValueError,KeyError,TypeError,RecursionError): return self.send(400,{'error':'invalid fixture progress'})
        return self.send(200,body['progress'])
print('TV polish fixture on 127.0.0.1:38359',flush=True)
ThreadingHTTPServer(('127.0.0.1',38359),Handler).serve_forever()
