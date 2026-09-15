import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { PlaybackSeekBar } from './playback-seek-bar';
it('exposes elapsed time and bounded screen-reader seek actions', async () => {
  const seek = jest.fn();
  const view = await render(
    <PlaybackSeekBar seconds={5} duration={12} onSeek={seek} />,
  );
  const slider = view.getByRole('adjustable', { name: 'Playback position' });
  expect(slider.props.accessibilityValue).toMatchObject({
    now: 5,
    text: '0:05 of 0:12',
  });
  await fireEvent(slider, 'accessibilityAction', {
    nativeEvent: { actionName: 'increment' },
  });
  expect(seek).toHaveBeenLastCalledWith(12);
  await fireEvent(slider, 'accessibilityAction', {
    nativeEvent: { actionName: 'decrement' },
  });
  expect(seek).toHaveBeenLastCalledWith(0);
  seek.mockClear();
  await fireEvent(slider, 'accessibilityAction', {
    nativeEvent: { actionName: 'unknown' },
  });
  expect(seek).not.toHaveBeenCalled();
});
it('previews a drag and seeks only on release, rejecting non-finite coordinates', async () => {
  const seek = jest.fn();
  const view = await render(
    <PlaybackSeekBar seconds={0} duration={100} onSeek={seek} />,
  );
  const slider = view.getByRole('adjustable');
  await fireEvent(slider, 'layout', {
    nativeEvent: { layout: { width: 200 } },
  });
  await fireEvent(slider, 'responderMove', { nativeEvent: { locationX: 100 } });
  expect(seek).not.toHaveBeenCalled();
  await fireEvent(slider, 'responderRelease', {
    nativeEvent: { locationX: 100 },
  });
  expect(seek).toHaveBeenCalledWith(50);
  seek.mockClear();
  await fireEvent(slider, 'responderRelease', {
    nativeEvent: { locationX: NaN },
  });
  expect(seek).not.toHaveBeenCalled();
});
it.each([0, -1, Infinity, NaN])(
  'does not offer a seek control without a usable duration %p',
  async (duration) => {
    const view = await render(
      <PlaybackSeekBar seconds={0} duration={duration} onSeek={jest.fn()} />,
    );
    expect(view.queryByRole('adjustable')).toBeNull();
  },
);

it('uses hours for long audiobook timelines', async () => {
  const view = await render(
    <PlaybackSeekBar seconds={3665} duration={36000} onSeek={jest.fn()} />,
  );
  expect(view.getByRole('adjustable').props.accessibilityValue.text).toBe(
    '1:01:05 of 10:00:00',
  );
  expect(view.getByText('−8:58:55')).toBeTruthy();
});
