import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { routeItem } from '@/testing/route-fixtures';
import { AudioArtwork, AudioButton } from './audio-player-controls';

it('labels transport controls and dispatches playback actions', async () => {
  const onPress = jest.fn();
  const view = await render(
    <AudioButton label="Pause" icon="pause" primary onPress={onPress} />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Pause' }));
  expect(onPress).toHaveBeenCalledTimes(1);
});
it('does not dispatch an unavailable next track', async () => {
  const onPress = jest.fn();
  const view = await render(
    <AudioButton label="Next track" icon="next" disabled onPress={onPress} />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Next track' }));
  expect(onPress).not.toHaveBeenCalled();
});
it('shows a meaningful placeholder without artwork', async () => {
  const view = await render(<AudioArtwork item={routeItem} />);
  expect(view.getByLabelText('No album artwork')).toBeTruthy();
});
it('recovers from failed artwork', async () => {
  const view = await render(
    <AudioArtwork item={routeItem} uri="https://example.test/art" />,
  );
  await fireEvent(
    view.getByLabelText(`Album artwork for ${routeItem.title}`),
    'error',
    { nativeEvent: { error: 'unavailable' } },
  );
  expect(view.getByLabelText('No album artwork')).toBeTruthy();
});

it('keeps audiobook cover proportions and handles an unavailable cover', async () => {
  const item = { ...routeItem, kind: 'audiobook' };
  const view = await render(
    <AudioArtwork item={item} uri="https://example.test/cover" />,
  );
  const cover = view.getByLabelText(`Book cover for ${item.title}`);
  expect(cover.props.contentFit).toBe('contain');
  await fireEvent(cover, 'error', { nativeEvent: { error: 'unavailable' } });
  expect(view.getByLabelText('No book cover')).toBeTruthy();
});
