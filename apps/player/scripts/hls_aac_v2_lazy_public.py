"""Actual old-client Version1 lazy continuation controls on disposable caches."""
import hashlib
import json
from pathlib import Path
import shutil
from hls_aac_v2_compat_public import diagnostics, require_diagnostics, requests, responses, snapshot
from hls_aac_v2_public_http import ActualServer, diagnostic_producer_rows, idle
from hls_followon_frames import decode_frames
from hls_followon_public import bounded_bytes, check, prepare_once
from hls_timeline_http import source_state


def indexed(cache):
    paths = list(cache.glob('*/.copy-timeline'))
    check(len(paths) == 1, 'lazy_single_indexed_generation')
    directory = paths[0].parent
    timeline = json.loads(bounded_bytes(paths[0], 256 << 10, 'lazy_timeline_bound'))
    certificate = json.loads(bounded_bytes(directory / '.copy-clock', 4096, 'lazy_clock_bound'))
    check(certificate['version'] == 1 and timeline.get('AudioOrigin') is None
        and timeline.get('Presentation') is None and timeline.get('Clock') is not None
        and len(timeline['Keys']) == 10 and timeline['Policy'] ==
        bounded_bytes(directory / '.source', 16 << 10, 'lazy_binding_bound').decode(),
        'lazy_real_whole_version1_contract')
    return directory


def physical(cache):
    media = indexed(cache) / '360p'
    return [name for name in ['init.mp4', *['segment-' + str(n).zfill(5) + '.m4s' for n in range(10)]]
        if (media / name).is_file()]


def clone_arm(root, run, label, binary, source, seed, owners):
    directory = run / label
    directory.mkdir()
    shutil.copytree(seed, directory / 'cache')
    initial = snapshot(directory / 'cache')
    owner = ActualServer(root, directory, source, binary)
    owner.clone_snapshot = initial
    owners.append(owner)
    owner.start(authorize=True)
    return owner


def selected_path(owner):
    item = next(v for v in owner.api.call('/api/v1/library')['items'] if v['title'] == 'Fixture')
    plan = owner.api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
    return item, plan['compatible'].replace('/index.m3u8', '-o12000/index.m3u8')


def warm_probe(root, run, baseline, source, owners, witness):
    row = witness.setdefault('probeSetup', {})
    owner = clone_arm(root, run, 'probe-warmup', baseline, source, run / 'empty-cache', owners)
    row.update(stage='public-planning', candidatePreparePOST=False)
    item, _ = selected_path(owner)
    cache = owner.directory / 'cache'
    name = 'probes/' + hashlib.sha256(item['id'].encode()).hexdigest()[:32] + '.json'
    state = snapshot(cache)
    initial = owner.clone_snapshot
    row['cacheChanges'] = {'addedSourceProbe': int(name in state and name not in initial),
        'otherAdded': sum(v != name for v in set(state) - set(initial)),
        'removed': len(set(initial) - set(state)),
        'changed': sum(state[v] != initial[v] for v in set(state) & set(initial))}
    row.update(cacheEntries=len(state), onlySourceProbe=set(state) == {name},
        sourceCalls=len(owner.source_invocation_rows()))
    check(row['onlySourceProbe'] and row['sourceCalls'] == 0, 'lazy_probe_setup_cache_or_encoder')
    data = json.loads(bounded_bytes(cache / name, 512 << 10, 'lazy_probe_setup_bound'))
    stat = source.stat()
    row.update(schema=data.get('schema'), sourceVersionMatched=data.get('version') ==
        str(stat.st_size) + ':' + str(stat.st_mtime_ns))
    facts = data.get('result', {})
    video, audio = facts.get('Video', {}), facts.get('AudioFacts', [])
    row['fixedFixtureFactsMatched'] = (set(data) == {'schema', 'version', 'result'}
        and video.get('Codec') == 'h264' and video.get('Width') == 640
        and video.get('Height') == 360 and video.get('FrameRate') == 24
        and len(audio) == 1 and audio[0].get('Codec') == 'aac'
        and audio[0].get('SampleRate') == 48000 and audio[0].get('Channels') == 2
        and 31.9 <= facts.get('Duration', 0) <= 32.1)
    check(row['schema'] == 3 and row['sourceVersionMatched'] and row['fixedFixtureFactsMatched'],
        'lazy_probe_setup_source_binding')
    row['idle'] = idle(owner.api, owner.process, source)
    row['idleCacheUnchanged'] = snapshot(cache) == state
    owner.stop()
    row.update(joinedCacheUnchanged=snapshot(cache) == state,
        finalSourceCalls=len(owner.source_invocation_rows()), sourceUnchanged=source_state(source) == owner.before,
        sessions=owner.sessions,
        sourceAudit=diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids))
    check(row['idleCacheUnchanged'] and row['joinedCacheUnchanged'] and row['finalSourceCalls'] == 0
        and row['sourceUnchanged'], 'lazy_probe_setup_late_mutation')
    check(snapshot(run / 'empty-cache') == {}, 'lazy_probe_template_already_populated')
    shutil.copytree(cache, run / 'empty-cache', dirs_exist_ok=True)
    check(set(snapshot(run / 'empty-cache')) == {name}, 'lazy_probe_template_identity')
    row['stage'] = 'qualified'


def seed(root, run, baseline, source, owners, witness):
    warm_probe(root, run, baseline, source, owners, witness)
    witness['stage'] = 'owned-start'
    owner = clone_arm(root, run, 'seed', baseline, source, run / 'empty-cache', owners)
    item, selected = selected_path(owner)
    witness.update(startupCacheUnchanged=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
        startupSourceCalls=len(owner.source_invocation_rows()))
    check(witness['startupCacheUnchanged'] and witness['startupSourceCalls'] == 0,
        'lazy_seed_startup_mutation')
    witness['stage'] = 'preparation'
    prepared = {}
    prepare_once(owner.api, '/api/v1/items/' + item['id'] + '/playback-prepare',
        selected, owner.log_path, owner.process, source, prepared)
    witness['preparation'] = prepared
    check(prepared['preparationAttempt']['completionState'] == 'ready', 'lazy_seed_ready')
    idle(owner.api, owner.process, source)
    cache = owner.directory / 'cache'
    witness.update(preparationPhysicalAssets=sorted(path.name for path in cache.glob('*/360p/*')
        if path.name == 'init.mp4' or path.name.startswith('segment-')),
        preparationStartupMarkers=len(list(cache.glob('*/.startup'))),
        preparationSourceCalls=len(owner.source_invocation_rows()))
    directory = indexed(cache)
    names = physical(cache)
    witness.update(speculativePhysicalAssets=names, speculativeStartupMarker=directory.joinpath('.startup').is_file())
    check(names == ['init.mp4', *['segment-' + str(n).zfill(5) + '.m4s' for n in range(4)]]
        and directory.joinpath('.startup').is_file()
        and len(owner.source_invocation_rows()) == 1, 'lazy_seed_speculative_physical_control')
    owner.stop()
    speculative = run / 'speculative-cache'
    shutil.copytree(cache, speculative)
    witness['stage'] = 'adoption'
    owner.start(binary=baseline)
    controls = responses(owner, [*requests('master', 'GET', selected), *requests('rendition', 'GET', selected)])
    idle(owner.api, owner.process, source)
    witness.update(adoptionStatuses=[v[0] for v in controls], adoptionStartupMarker=directory.joinpath('.startup').exists(),
        adoptedPhysicalAssets=physical(cache), adoptionSourceCalls=len(owner.source_invocation_rows()))
    check(all(v[0] == 200 for v in controls) and not directory.joinpath('.startup').exists()
        and physical(cache) == names and len(owner.source_invocation_rows()) == 1, 'lazy_seed_adoption')
    owner.stop()
    adopted = run / 'adopted-cache'
    shutil.copytree(cache, adopted)
    witness['stage'] = 'baseline-hydration'
    owner.start(binary=baseline)
    full = responses(owner, requests('journey', 'GET', selected))
    witness['hydrationStatuses'] = [v[0] for v in full]
    check(all(v[0] == 200 for v in full), 'lazy_seed_complete_baseline_journey')
    idle(owner.api, owner.process, source)
    owner.stop()
    joined = owner.directory / 'seed-joined.mp4'
    joined.write_bytes(b''.join(v[1] for v in full[2:]))
    _, frames = decode_frames(joined)
    _, reference = decode_frames(source, offset=12)
    witness.update(decodedFrameCount=len(frames), referenceFrameCount=len(reference),
        exactSourceFrameSequence=[v[1] for v in frames] == [v[1] for v in reference])
    check(len(frames) == len(reference) == 480
        and [v[1] for v in frames] == [v[1] for v in reference], 'lazy_seed_exact_source_frames')
    complete = run / 'complete-cache'
    shutil.copytree(cache, complete)
    witness['completePhysicalAssets'] = physical(complete)
    check(len(physical(complete)) == 11, 'lazy_seed_all_physical_assets')
    witness.update(stage='qualified', decodedSourceFrames=480,
        finalSourceCalls=len(owner.source_invocation_rows()),
        sourceAudit=diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids),
        sessions=owner.sessions)
    return selected, speculative, adopted, complete


def candidate_changes(before, after, generation, startup):
    allowed_removed = {generation + '/.startup'} if startup else set()
    preserved = {name: row for name, row in before.items() if name not in allowed_removed}
    missing = {generation + '/360p/segment-' + str(n).zfill(5) + '.m4s' for n in range(10)} - set(before)
    return (all(after.get(name) == row for name, row in preserved.items())
        and set(before) - set(after) == allowed_removed
        and set(after) - set(before) == missing)


def journey(root, run, label, binary, source, cache, selected, owners, source_before, candidate, row):
    owner = clone_arm(root, run, label, binary, source, cache, owners)
    actual_selected = selected_path(owner)[1]
    row.update(cacheUnchangedAfterStartup=snapshot(owner.directory / 'cache') == owner.clone_snapshot,
        startupSourceCalls=len(owner.source_invocation_rows()), requestIdentityMatched=actual_selected == selected)
    check(row['cacheUnchangedAfterStartup'] and row['startupSourceCalls'] == 0
        and row['requestIdentityMatched'], 'lazy_clone_startup_or_request_identity_changed')
    cache = owner.directory / 'cache'
    directory = indexed(cache)
    generation = directory.name
    before = owner.clone_snapshot
    startup = directory.joinpath('.startup').exists()
    initial_assets = physical(cache)
    row.update(result='failed', candidatePreparePOST=False, initialAssets=initial_assets,
        initialStartupMarker=startup)
    # HEAD first is a public metadata control, then the real client GET journey.
    head = responses(owner, [*requests('master', 'HEAD', selected), *requests('rendition', 'HEAD', selected)])
    row.update(headStatuses=[v[0] for v in head], headCacheUnchanged=snapshot(cache) == before,
        headSourceCalls=len(owner.source_invocation_rows()))
    replies = responses(owner, requests('journey', 'GET', selected))
    row.update(statuses=[v[0] for v in replies],
        responseSHA256=[hashlib.sha256(v[1]).hexdigest() for v in replies])
    row['idle'] = idle(owner.api, owner.process, source)
    row['diagnostics'] = diagnostics(owner, [*head, *replies])
    idle_snapshot = snapshot(cache)
    owner.stop()
    row.update(sourceCalls=len(owner.source_invocation_rows()), sourceUnchanged=source_state(source) == source_before,
        finalAssets=physical(cache), finalStartupMarker=directory.joinpath('.startup').exists(),
        sourceAudit=diagnostic_producer_rows(owner.invocation_rows(), source, owner.owned_pids),
        sessions=owner.sessions)
    after = snapshot(cache)
    row['existingCachePreserved'] = candidate_changes(before, after, generation, startup)
    row['joinedCacheUnchangedFromIdle'] = idle_snapshot == after
    if all(v[0] == 200 for v in replies):
        joined = owner.directory / 'joined.mp4'
        joined.write_bytes(b''.join(v[1] for v in replies[2:]))
        _, frames = decode_frames(joined)
        _, reference = decode_frames(source, offset=12)
        row['decodedFrames'] = len(frames)
        row['exactDecodedSourceFrames'] = len(frames) == len(reference) == 480 and (
            [v[1] for v in frames] == [v[1] for v in reference])
    try:
        check(all(v[0] == 200 and not v[1] for v in head)
            and row['headCacheUnchanged'] and row['headSourceCalls'] == 0, 'lazy_head_regression')
        check(all(v[0] == 200 for v in replies) and row.get('exactDecodedSourceFrames'),
            'lazy_existing_client_status_or_frames_regression')
        check(row['sourceUnchanged'] and len(row['finalAssets']) == 11 and not row['finalStartupMarker']
            and 1 <= row['sourceCalls'] <= 10, 'lazy_source_or_completion_regression')
        if candidate:
            check(row['existingCachePreserved'] and row['joinedCacheUnchangedFromIdle'],
                'lazy_existing_generation_mutated')
        require_diagnostics(row['diagnostics'], 0)
        row['result'] = 'observed'
    except RuntimeError as error:
        row['failureClass'] = str(error)
    return row, replies
