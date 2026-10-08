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
