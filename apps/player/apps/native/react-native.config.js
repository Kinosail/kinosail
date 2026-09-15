module.exports = {
  dependencies:
    process.env.EXPO_TV === '1'
      ? {
          'react-native-google-cast': {
            platforms: { ios: null, android: null },
          },
        }
      : {},
};
