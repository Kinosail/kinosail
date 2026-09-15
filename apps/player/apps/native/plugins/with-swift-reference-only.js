const { withDangerousMod } = require('expo/config-plugins');

// Preserve the old Apple implementation as a migration/QA reference, but never
// let an older deployment watcher replace a device app with an Expo build.
module.exports = (config) =>
  withDangerousMod(config, [
    'ios',
    () => {
      throw new Error(
        'Apple builds moved to Kinosail.xcodeproj. Use scripts/build-apple.sh. The Expo source is a migration reference.',
      );
    },
  ]);
