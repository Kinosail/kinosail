/** @jest-environment node */
const {
  configurePodfile,
  configureProject,
} = require('./with-xcode-build-cleanup');
const hook =
  'post_install do |installer|\n    react_native_post_install(\n      installer,\n      :ccache_enabled => ccache_enabled?(podfile_properties),\n    )\nend';
it('keeps the native post-install operation and adds one repeatable settings hook', () => {
  const result = configurePodfile(hook);
  expect(result).toContain('react_native_post_install(');
  expect(result).toContain('kinosail_xcode_build_settings(installer)');
  expect(configurePodfile(result)).toBe(result);
  expect(result).not.toContain('inhibit_all_warnings');
});
it.each([
  null,
  '',
  'unexpected hook',
  hook + hook,
  'x'.repeat(1024 * 1024 + 1),
  '# Kinosail Xcode build settings',
])('rejects missing, ambiguous or malformed generated Podfiles %#', (value) => {
  expect(() => configurePodfile(value)).toThrow();
});
const fixture = () => {
  const settings = {
    PRODUCT_BUNDLE_IDENTIFIER: 'com.kinosail.player',
    OTHER_LDFLAGS: ['"$(inherited)"', '"-ObjC"', '"-lc++"'],
    OTHER_CFLAGS: ['-Wall'],
  };
  const phase = {
    name: '"Bundle React Native code and images"',
    shellScript: JSON.stringify('original bundle command'),
  };
  return {
    settings,
    phase,
    project: {
      pbxXCBuildConfigurationSection: () => ({
        app: { buildSettings: settings },
        comment: 'comment',
      }),
      hash: {
        project: { objects: { PBXShellScriptBuildPhase: { bundle: phase } } },
      },
    },
  };
};
it('removes only the duplicate libc++ flag and preserves diagnostics and the original bundler', () => {
  const { settings, phase, project } = fixture();
  configureProject(project);
  expect(settings.OTHER_LDFLAGS).toEqual(['"$(inherited)"', '"-ObjC"']);
  expect(settings.OTHER_CFLAGS).toEqual(['-Wall']);
  expect(JSON.parse(phase.shellScript)).toContain('original bundle command');
  const before = phase.shellScript;
  configureProject(project);
  expect(phase.shellScript).toBe(before);
});
it.each([null, 'bad JSON', '{}', JSON.stringify('x'.repeat(1024 * 1024 + 1))])(
  'rejects invalid bundle scripts before altering link settings %#',
  (value) => {
    const { settings, phase, project } = fixture();
    phase.shellScript = value;
    const before = JSON.stringify(settings);
    expect(() => configureProject(project)).toThrow();
    expect(JSON.stringify(settings)).toBe(before);
  },
);
