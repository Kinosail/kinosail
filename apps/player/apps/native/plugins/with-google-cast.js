// Google Cast sender SDKs support mobile devices, not tvOS builds.
module.exports = (config) => {
  if (process.env.EXPO_TV === '1') return config;
  const withGoogleCast = require('react-native-google-cast/app.plugin').default;
  return withGoogleCast(config, {
    receiverAppId: 'CC1AD845',
    iosStartDiscoveryAfterFirstTapOnCastButton: true,
    iosSuspendSessionsWhenBackgrounded: true,
    expandedController: true,
  });
};
