"""Exact synthetic provider identities; no live provider or approval proof."""

PROJECTS = ('chromium', 'firefox', 'webkit')
CASES = (
    ('home-assistant.spec.ts', 'Home Assistant stays dark until the Owner enables it in Settings and the wizard'),
    ('jellyfin-setup.spec.ts', 'Jellyfin setup stays blocked until trusted HTTPS is saved'),
    ('supporter-checkout.spec.ts', 'Owner can switch Player checkout cadence without losing levels or accessibility'),
    ('supporter-sailcraft.spec.ts', 'Sailcraft honors remain readable in every supporter state'),
    ('supporter-sailcraft.spec.ts', 'Contribution frequency shows all ten agreed amounts'),
    ('supporter-sailcraft.spec.ts', 'Collected badges and hidden recognition work on desktop and mobile'),
    ('supporter-sailcraft.spec.ts', 'Display settings persist through the web form and API'),
    ('supporter-sailcraft.spec.ts', 'Honors support keyboard, light theme, reduced motion, and forced colors'),
    ('test-instance-supporter.spec.ts', 'Pending supporter recognition keeps navigation usable and then shows the collection'),
    ('test-instance-supporter.spec.ts', 'Supporter brand marks do not compress the server title'),
    ('test-instance-supporter.spec.ts', 'Supporter chooser combines badges and prices with optional activation'),
    ('test-instance-supporter.spec.ts', 'Every supporter level shows its artwork and title'),
    ('test-instance-supporter.spec.ts', 'Legacy supporter honors and masterwork remain visible beside current editions'),
    ('test-instance-supporter.spec.ts', 'Supporter badge rendering and share conversion remain responsive'),
    ('test-instance-supporter.spec.ts', 'Home keeps support reachable and respects hidden supporter recognition'),
)


def selected_cases(project):
    if not isinstance(project, str) or project not in PROJECTS:
        raise ValueError('fixed provider project required')
    return CASES

UI_FILES = ('supporter-populated-both.html', 'supporter-empty.html', 'supporter-living.html', 'supporter-patron.html', 'supporter-archived.html', 'supporter-long-name.html', 'supporter-living-certificate.html', 'supporter-patron-certificate.html', 'supporter-certificate.html')
UI_OUTPUTS = ('api-key-created.html', 'mcp-approval.html', 'media-share-items.html', 'mfa-required.html', 'mfa-setup.html', 'oidc-mfa.html', 'passkey-account.html', 'passkey-prompt.html', 'state-contracts.html', 'supporter-archived.html', 'supporter-certificate.html', 'supporter-empty.html', 'supporter-living-certificate.html', 'supporter-living.html', 'supporter-long-name.html', 'supporter-patron-certificate.html', 'supporter-patron.html', 'supporter-populated-both.html', 'viewing-import-preview.html', 'viewing-import-result.html')


def ui_fixtures(directory):
    """Admit fixed production-rendered local files before Owner/browser effects."""
    import hashlib
    import os
    from pathlib import Path
    import stat
    from library_profile_admission import read_proof
    if not isinstance(directory, (str, Path)):
        raise ValueError('owned UI fixture directory required')
    directory = Path(directory)
    if not directory.is_absolute() or len(str(directory)) > 4096 or '..' in directory.parts:
        raise ValueError('absolute UI fixture directory required')
    before = directory.stat(follow_symlinks=False)
    if not stat.S_ISDIR(before.st_mode):
        raise ValueError('regular UI fixture directory required')
    names = os.listdir(directory)
    if len(names) > 32 or not set(UI_FILES) <= set(names) or not set(names) <= set(UI_OUTPUTS):
        raise ValueError('closed UI renderer outputs required')
    total, hashes = 0, {}
    for name in names:
        raw = read_proof(directory / name)
        total += len(raw)
        if total > 4194304:
            raise ValueError('bounded rendered UI fixtures required')
        text = raw.decode('utf-8')
        if '<html' not in text and '<svg' not in text:
            raise ValueError('rendered HTML or SVG required')
        if name in UI_FILES:
            hashes[name] = hashlib.sha256(raw).hexdigest()
    after = directory.stat(follow_symlinks=False)
    if not stat.S_ISDIR(after.st_mode) or (before.st_dev, before.st_ino) != (after.st_dev, after.st_ino):
        raise ValueError('canonical UI fixture directory required')
    return hashes
