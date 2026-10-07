"""Closed synthetic library-owner selection and exact first-attempt proof admission."""
import json

PROJECTS = ('chromium', 'firefox', 'webkit')
# Exact retained ba5 discovery identities, not executed-case evidence.
CASES = (
    ('collections-loading.spec.ts', 'Collections keep poster geometry while artwork loads'),
    ('curation-count.spec.ts', 'playlist detail counts match zero, one, and multiple titles'),
    ('curation-count.spec.ts', 'Collection detail counts match zero, one, and multiple titles'),
    ('mobile-quick-connect.spec.ts', 'Quick Connect is visible when More opens at 320px'),
    ('mobile-quick-connect.spec.ts', 'Quick Connect is visible when More opens at 390px'),
    ('navigation-repeat.spec.ts', 'repeating the active Movies link does not reload the document'),
    ('navigation-repeat.spec.ts', 'refresh keeps compact navigation and sign out reachable'),
    ('navigation-repeat.spec.ts', 'double-clicking a compact menu shortcut starts one navigation'),
    ('password-reveal.spec.ts', 'passwords can be shown and hidden without changing their value'),
    ('password-reveal.spec.ts', 'the password reveal control stays aligned across responsive settings'),
    ('shortcuts.spec.ts', 'Linear-style navigation shortcuts work on browse and settings pages'),
    ('shortcuts.spec.ts', 'closed command inputs and active editable fields handle go chords safely'),
    ('show-episode-ledger.spec.ts', 'season reel keeps episode choice cinematic, scannable, and responsive'),
    ('static-compression.spec.ts', 'compressed assets execute and reuse the warm cache at 390px'),
    ('static-compression.spec.ts', 'compressed assets execute and reuse the warm cache at 1440px'),
    ('test-instance-library.spec.ts', 'a stale login page redirects passkey sign-in to the canonical origin'),
    ('test-instance-library.spec.ts', 'public test instance exercises every media section and local TMDB metadata'),
    ('test-instance-library.spec.ts', 'media artwork keeps its intended ratio in the populated library'),
    ('test-instance-library.spec.ts', 'mobile media detail heroes stack artwork for shows, albums, and books'),
    ('test-instance-library.spec.ts', 'show episodes expose their 16:9 still artwork'),
    ('test-instance-library.spec.ts', 'beta UI surfaces stay reachable and expose only working controls'),
    ('test-instance-watched-departure.spec.ts', 'usable login form authenticates while an unrelated image remains pending'),
    ('test-instance-watched-departure.spec.ts', "Mark watched without JavaScript rejects the current page's first late progress write"),
    ('test-instance.spec.ts', 'preferred-language subtitle choices hide other tracks without changing files'),
    ('test-instance.spec.ts', 'unavailable Web Locks do not initialize offline storage on the downloads page'),
    ('test-instance.spec.ts', 'Viewer MFA enrollment gives accurate instructions'),
    ('test-instance.spec.ts', 'Connection choices stay optional and secure by default'),
    ('test-instance.spec.ts', 'Mobile More menu prioritizes personal tabs'),
    ('test-instance.spec.ts', 'Owner can edit Main navigation from desktop and compact layouts'),
    ('test-instance.spec.ts', 'Community edition keeps Support Kinosail reachable'),
    ('test-instance.spec.ts', 'supporter collection shows valid badges in the header'),
    ('test-instance.spec.ts', 'clearing a supporter collection removes badge styling'),
    ('test-instance.spec.ts', 'invalid supporter levels do not render a badge'),
    ('title-jump-first-paint.spec.ts', 'populated mobile library keeps the compact title jump while navigation loads'),
    ('ui-happy-paths.spec.ts', 'Owner can toggle the active title letter without a second scroll'),
    ('ui-happy-paths.spec.ts', 'Owner can use keyboard utilities and persist the chosen theme'),
    ('ui-happy-paths.spec.ts', 'Dark is the default and every theme choice persists'),
    ('ui-happy-paths.spec.ts', 'Owner can create, fill, empty, and delete a playlist and Collection'),
    ('ui-happy-paths.spec.ts', 'Owner can create and revoke an API key and manage a Viewer Profile'),
    ('ui-happy-paths.spec.ts', 'Owner can share Library Content, open the claim, and revoke it'),
    ('ui-happy-paths.spec.ts', 'Owner can authorize a waiting device with Quick Connect'),
    ('ui-happy-paths.spec.ts', 'Owner can paste a Quick Connect code into the digit fields'),
    ('web-qa-fixes.spec.ts', 'phone navigation keeps the TV Shows context on show details'),
    ('web-qa-fixes.spec.ts', 'phone brand link has a full touch target'),
    ('web-qa-fixes.spec.ts', 'watch page keeps device warnings out of initial playback'),
    ('dependency-thanks.spec.ts', 'Owner can read dependency thanks and notices on phone and desktop'),
)


def selection(arguments):
    if (not isinstance(arguments, (tuple, list)) or len(arguments) != 3
            or arguments[0] != 'library-owner' or arguments[1] not in PROJECTS
            or arguments[2] != 'fresh'):
        raise ValueError('fixed library profile/project/fresh state required')
    return tuple(arguments)


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError('ambiguous proof')
        value[key] = item
    return value


def invalid_number(_value):
    raise ValueError('nonfinite proof')


def admit(raw, profile, project, state, completed):
    selection((profile, project, state))
    if not isinstance(raw, bytes) or not 1 <= len(raw) <= 2097152 or type(completed) is not bool:
        raise ValueError('bounded proof required')
    try:
        value = json.loads(raw.decode('utf-8'), object_pairs_hook=unique_object, parse_constant=invalid_number)
        return admit_report(value, project, completed)
    except (KeyError, TypeError, AttributeError, UnicodeError, RecursionError) as error:
        raise ValueError('invalid fixed library proof') from error


def admit_report(value, project, completed):
    if not isinstance(value, dict) or value['errors'] != []:
        raise ValueError('invalid proof')
    config = value['config']
    if (type(config['workers']) is not int or config['workers'] != 1 or config['shard'] is not None
            or not isinstance(config['projects'], list) or len(config['projects']) != 1):
        raise ValueError('fixed runner required')
    configured = config['projects'][0]
    if (configured['name'] != project or type(configured['retries']) is not int or configured['retries'] != 0
            or type(configured['repeatEach']) is not int or configured['repeatEach'] != 1):
        raise ValueError('one first attempt required')
    stats = value['stats']
    for name, count in (('expected', 46 if completed else 0), ('skipped', 0), ('unexpected', 0), ('flaky', 0)):
        if type(stats[name]) is not int or stats[name] != count:
            raise ValueError('exact result counts required')
    if not isinstance(value['suites'], list) or len(value['suites']) > 64:
        raise ValueError('bounded suite tree required')
    pending = [(suite, (), 0, None) for suite in value['suites']]
    found, identifiers, visited = set(), set(), 0
    while pending:
        suite, ancestors, depth, root_file = pending.pop()
        visited += 1
        if not isinstance(suite, dict) or depth > 8 or visited > 64:
            raise ValueError('bounded suite tree required')
        title = suite['title']
        if not isinstance(title, str) or not 1 <= len(title) <= 240:
            raise ValueError('bounded title required')
        if depth == 0:
            if title not in {file for file, _ in CASES}:
                raise ValueError('registered file suite required')
            root_file = title
        context = ancestors if depth == 0 else (*ancestors, title)
        children, specs = suite.get('suites', []), suite.get('specs', [])
        if not isinstance(children, list) or not isinstance(specs, list) or len(children) > 64 or len(specs) > 46:
            raise ValueError('bounded cases required')
        pending.extend((child, context, depth + 1, root_file) for child in children)
        for spec in specs:
            identity = (spec['file'], ' › '.join((*context, spec['title'])), project)
            identifier = spec['id']
            if (spec['file'] != root_file or identity[:2] not in CASES or identity in found or not isinstance(identifier, str)
                    or not 1 <= len(identifier) <= 512 or identifier in identifiers or len(found) >= 46):
                raise ValueError('exact unique identities required')
            tests = spec['tests']
            if not isinstance(tests, list) or len(tests) != 1:
                raise ValueError('one project per identity required')
            test = tests[0]
            if test['projectName'] != project or test['expectedStatus'] != 'passed':
                raise ValueError('wrong project or expected status')
            results = test['results']
            if completed:
                if spec['ok'] is not True or test['status'] != 'expected' or not isinstance(results, list) or len(results) != 1:
                    raise ValueError('one passing result required')
                result = results[0]
                if result['status'] != 'passed' or type(result['retry']) is not int or result['retry'] != 0 or result['errors'] != []:
                    raise ValueError('first attempt pass required')
            elif results != []:
                raise ValueError('discovery is not execution')
            found.add(identity)
            identifiers.add(identifier)
    if found != {(file, title, project) for file, title in CASES}:
        raise ValueError('all46 fixed identities required')
    return found
