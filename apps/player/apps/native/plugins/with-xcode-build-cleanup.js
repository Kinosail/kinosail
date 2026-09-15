const { withPodfile, withXcodeProject } = require('expo/config-plugins');

const marker = '# Kinosail Xcode build settings';
const configurePodfile = (contents) => {
  if (typeof contents !== 'string' || contents.length > 1024 * 1024) {
    throw new Error('Invalid generated Podfile');
  }
  const hook = `    ${marker}\n    require_relative '../plugins/xcode-build-settings'\n    kinosail_xcode_build_settings(installer)`;
  if (contents.includes(marker)) {
    if (!contents.includes(hook))
      throw new Error('Incomplete Kinosail Podfile hook');
    return contents;
  }
  const anchor =
    '      :ccache_enabled => ccache_enabled?(podfile_properties),\n    )';
  if (contents.split(anchor).length !== 2) {
    throw new Error(
      'React Native post-install hook changed; review Xcode build settings',
    );
  }
  return contents.replace(anchor, `${anchor}\n${hook}`);
};
const configureProject = (project) => {
  const updates = [];
  for (const phase of Object.values(
    project.hash.project.objects.PBXShellScriptBuildPhase,
  )) {
    if (
      !phase ||
      typeof phase !== 'object' ||
      phase.name !== '"Bundle React Native code and images"'
    )
      continue;
    if (
      typeof phase.shellScript !== 'string' ||
      phase.shellScript.length > 1024 * 1024
    )
      throw new Error('Invalid native bundle phase');
    const script = JSON.parse(phase.shellScript);
    if (typeof script !== 'string')
      throw new Error('Invalid native bundle script');
    if (!script.includes(marker)) {
      updates.push([
        phase,
        JSON.stringify(
          `${marker}\nunset NO_COLOR\nexport FORCE_COLOR=0\nexport HERMES_GLOBALS_FILE="$PROJECT_DIR/../scripts/hermes-globals.js"\n${script}`,
        ),
      ]);
    }
  }
  const configurations = Object.values(
    project.pbxXCBuildConfigurationSection(),
  );
  for (const entry of configurations) {
    const settings = entry?.buildSettings;
    if (!settings?.PRODUCT_BUNDLE_IDENTIFIER) continue;
    // CocoaPods already supplies libc++; keep its inherited link settings.
    if (Array.isArray(settings.OTHER_LDFLAGS)) {
      settings.OTHER_LDFLAGS = settings.OTHER_LDFLAGS.filter(
        (flag) => !['-lc++', '"-lc++"'].includes(flag),
      );
    }
  }
  for (const [phase, script] of updates) phase.shellScript = script;
  return project;
};
module.exports = (config) => {
  config = withPodfile(config, (result) => {
    result.modResults.contents = configurePodfile(result.modResults.contents);
    return result;
  });
  return withXcodeProject(config, (result) => {
    configureProject(result.modResults);
    return result;
  });
};
module.exports.configurePodfile = configurePodfile;
module.exports.configureProject = configureProject;
