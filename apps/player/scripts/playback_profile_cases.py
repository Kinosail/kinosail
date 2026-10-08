"""Closed playback catalogue; explicit capability skips are not playback passes."""
PROJECTS = ('chromium', 'firefox', 'webkit')
CDP_REASON = 'network emulation uses Chromium DevTools'
CASES = (
    ('playback-bandwidth.spec.ts', 'ordinary mobile bandwidth keeps moving video unobstructed'),
    ('playback-bandwidth.spec.ts', 'bandwidth starvation keeps Direct Play and recovers in place'),
    ('playback-bandwidth.spec.ts', 'an interrupted Direct Play request retries without transcoding'),
    ('playback-bandwidth.spec.ts', 'repeated bandwidth changes do not leave a stale player state'),
    ('playback-bandwidth.spec.ts', 'compatibility streaming recovers after the network disappears'),
    ('test-instance-direct-retry.spec.ts', 'direct retry at 390×844 › @smoke direct-only playback retries one failed media request'),
    ('test-instance-direct-retry.spec.ts', 'direct retry at 1440×900 › @smoke direct-only playback retries one failed media request'),
    ('test-instance-playback.spec.ts', '@smoke native playback seeks and retains controls on a populated title'),
    ('test-instance-playback.spec.ts', 'adaptive playback starts on Auto and keeps semantic quality choices'),
    ('test-instance-playback.spec.ts', 'Owner can inspect, update, and remove a skip marker'),
    ('test-instance-playback.spec.ts', 'native compatibility playback retains the full movie seek range'),
    ('instant-playback.spec.ts', 'Selecting a movie reaches moving video in under two seconds'),
    ('instant-show-play.spec.ts', 'a show card keeps Details and plays the next episode with one click'),
    ('instant-show-play.spec.ts', 'collection and playlist actions share a baseline when titles wrap'),
    ('playback-pause-repro.spec.ts', 'one play request does not immediately pause (direct)'),
    ('playback-pause-repro.spec.ts', 'one play request does not immediately pause (compatible)'),
    ('playback-startup.spec.ts', 'selecting a movie starts moving playback promptly'),
    ('playback-startup.spec.ts', 'blocked autoplay leaves one Play control that starts direct video from the beginning'),
    ('playback-startup.spec.ts', 'blocked autoplay leaves one Play control that starts direct video from saved progress'),
    ('playback-startup.spec.ts', 'blocked autoplay leaves one Play control that starts compatible video from the beginning'),
    ('playback-startup.spec.ts', 'blocked autoplay leaves one Play control that starts compatible video from saved progress'),
    ('playback-startup.spec.ts', 'blocked autoplay leaves one Play control that starts automatic video from the beginning'),
    ('playback-startup.spec.ts', 'blocked autoplay leaves one Play control that starts automatic video from saved progress'),
    ('playback-startup.spec.ts', 'restricted browser storage does not stop playback'),
    ('playback-startup.spec.ts', 'failed progress save does not stop playback flow'),
    ('playback-startup.spec.ts', 'selecting compatibility playback starts without a second play click'),
    ('playback-startup.spec.ts', 'trusted source opening sets the first media time'),
)


def selected_cases(project):
    if project not in PROJECTS:
        raise ValueError('fixed playback project required')
    return tuple(row for row in CASES
                 if project == 'chromium' or row[0] != 'test-instance-direct-retry.spec.ts')


def expected_skips(project):
    return {row: CDP_REASON for row in selected_cases(project)
            if project != 'chromium' and row[0] == 'playback-bandwidth.spec.ts'}


def admit_skip(test, result, reason):
    def annotations(value):
        if not isinstance(value, list) or len(value) != 1:
            raise ValueError('one exact capability annotation required')
        annotation = value[0]
        if (not isinstance(annotation, dict) or set(annotation) - {'type', 'description', 'location'}
                or annotation.get('type') != 'skip' or annotation.get('description') != reason):
            raise ValueError('registered capability reason required')
        if 'location' in annotation:
            location = annotation['location']
            if (not isinstance(location, dict) or set(location) != {'file', 'line', 'column'}
                    or not isinstance(location['file'], str) or not 1 <= len(location['file']) <= 4096
                    or any(type(location[key]) is not int or not 1 <= location[key] <= 1000000
                           for key in ('line', 'column'))):
                raise ValueError('bounded annotation location required')
    annotations(test.get('annotations'))
    if result.get('annotations'):
        annotations(result['annotations'])
    elif result.get('annotations', []) != []:
        raise ValueError('invalid result annotations')
