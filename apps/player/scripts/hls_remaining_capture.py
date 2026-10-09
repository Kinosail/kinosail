"""Private actual codec invocation capture and closed production origin-refill recipe."""
from pathlib import Path
import shutil
from hls_followon_public import check
from hls_timeline_http import sha


def capture_codec(directory, source, pacing, installation, env, case):
    check(pacing is None or type(pacing) in [int, float] and pacing in [0.75, 0.9, 1.25], 'refill_capture_pacing')
    real = shutil.which('ffmpeg')
    check(real is not None, 'ffmpeg_unavailable')
    wrapper = directory / 'paced-ffmpeg'
    invocation = directory / 'pacing-invocations.jsonl'
    wrapper.write_text('#!/usr/bin/env python3\nimport os,sys,json\na=sys.argv[1:]\n'
        "if '-hls_time' in a and " + repr(pacing is not None) + ":\n"
        " if '-readrate' in a: a[a.index('-readrate')+1]=" + repr(str(pacing)) + '\n'
        " else: a[a.index('-i'):a.index('-i')]=['-readrate'," + repr(str(pacing)) + ']\n'
        + ' with open(' + repr(str(invocation)) + ", 'a') as f: f.write(json.dumps({'pid':os.getpid(),'parent':os.getppid(),'sourceMatched':" + repr(str(source)) + " in a,'readrate':a[a.index('-readrate')+1]})+'\\n')\n"
        + "if '-start_number' in a and a[a.index('-start_number')+1]=='4' and " + repr(str(source)) + " in a:\n"
        + ' value=json.dumps(a)\n if not (0<len(a)<=96 and all(len(v.encode())<=4096 for v in a) and len(value.encode())<=8192): raise SystemExit("refill_capture_bound")\n'
        + ' with os.fdopen(os.open(' + repr(str(directory / 'refill-recipe-private.json'))
        + ',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600),"w") as f: f.write(value)\n'
        + (" sys.path.insert(0," + repr(str(Path(__file__).parent)) + ")\n from hls_remaining_installation import installed_refill\n"
            + " a=installed_refill(a," + repr(str(source)) + ',' + repr(str(directory))
            + ",os.environ.get('KINOSAIL_INSTALLATION_RECEIPT_DIR'))\n" if installation else '')
        + 'os.execv(' + repr(real) + ',[' + repr(real) + ']+a)\n')
    wrapper.chmod(0o700)
    env['KINOSAIL_FFMPEG'] = str(wrapper)
    case['actualCodecInvocation'] = {'wrapperSHA256': sha(wrapper), 'executableSHA256': sha(Path(real))}
    if pacing is not None:
        case['testOnlyRealCodecPacing'] = dict(case['actualCodecInvocation'], readrate=pacing)
    return real, invocation


def origin_refill_template(source, root, case):
    selected = case.get('originSelectedAudio', {})
    check(selected == {'codec': 'flac', 'sampleRate': 48000, 'channels': 2, 'layout': 'stereo', 'index': 0,
        'sourceIndex': 0} and all(type(selected.get(k)) is int for k in ['sampleRate', 'channels', 'index', 'sourceIndex'])
        and type(case.get('planDurationSeconds')) in [int, float] and case['planDurationSeconds'] == 10,
        'refill_recipe_origin_eligibility')
    pacing = case.get('testOnlyRealCodecPacing')
    check(pacing is None or isinstance(pacing, dict) and type(pacing.get('readrate')) in [int, float] and
        pacing['readrate'] in [0.75, 0.9, 1.25], 'refill_recipe_pacing')
    expected = ['-hide_banner', '-loglevel', 'error', '-y', '-avoid_negative_ts', 'disabled', '-max_delay', '5000000',
        '-ss', '0.000', *(['-readrate', str(pacing['readrate'])] if pacing is not None else []), '-i', str(source),
        '-map', '0:a:0', '-vn', '-sn', '-dn', '-c:a', 'aac', '-ac', '2', '-b:a', '192000',
        '-output_ts_offset', str(8 - 382976 / 48000), '-bsf:a', 'noise=amount=0:drop=lt(pts\\,382976)',
        '-f', 'hls', '-hls_time', '2', '-hls_playlist_type', 'event', '-hls_segment_type', 'fmp4',
        '-hls_segment_options', 'movflags=+frag_discont+skip_sidx', '-hls_flags', 'temp_file',
        '-hls_fmp4_init_filename', 'init.mp4', '-start_number', '4', '-hls_segment_filename',
        str(root / 'audio/segment-%05d.m4s'), str(root / '.seek-4/audio/index.m3u8')]
    return expected
