import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { requireNativeView } from 'expo';
import LocalVideo from './local-video.native';

jest.mock('@/core/protected-media', () => ({
  localCompatibilityAvailable: true,
}));
jest.mock('expo', () => {
  const ReactModule = jest.requireActual('react');
  const { View } = jest.requireActual('react-native');
  return {
    requireNativeView: jest.fn(() =>
      ReactModule.forwardRef((props: object, ref: unknown) =>
        ReactModule.createElement(View, {
          ...props,
          ref,
          testID: 'local-video',
        }),
      ),
    ),
  };
});
it('uses the Expo 57 view factory and forwards resume and native events', async () => {
  const progress = jest.fn(),
    tracks = jest.fn(),
    buffering = jest.fn();
  const view = await render(
    <LocalVideo
      source={{ uri: 'http://127.0.0.1:1234/capability', start: 42 }}
      paused={false}
      onProgress={progress}
      onTracks={tracks}
      onBuffering={buffering}
      onFirstFrame={jest.fn()}
      onPlaying={jest.fn()}
      onPaused={jest.fn()}
      onPictureInPictureReady={jest.fn()}
      onEnd={jest.fn()}
      onError={jest.fn()}
    />,
  );
  expect(requireNativeView).toHaveBeenCalledWith('ProtectedMedia');
  const native = view.getByTestId('local-video');
  expect(native.props.start).toBe(42);
  await fireEvent(native, 'progress', { nativeEvent: { currentTime: 43 } });
  expect(progress).toHaveBeenCalledWith({ currentTime: 43 });
  await fireEvent(native, 'buffering', { nativeEvent: { active: true } });
  expect(buffering).toHaveBeenCalledWith({ active: true });
});
