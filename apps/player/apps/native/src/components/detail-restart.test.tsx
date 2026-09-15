import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { DetailView } from './detail-view';
import { routeItem } from '@/testing/route-fixtures';
it('offers a separate restart action alongside resume', async () => {
  const onPlay = jest.fn(),
    onPlayFromBeginning = jest.fn();
  const view = await render(
    <DetailView
      item={routeItem}
      headers={{}}
      mediaURL={(path) => path}
      onBack={() => {}}
      onPlay={onPlay}
      onPlayFromBeginning={onPlayFromBeginning}
    />,
  );
  await fireEvent.press(
    view.getByRole('button', { name: 'More title actions' }),
  );
  await fireEvent.press(
    view.getByRole('button', { name: 'Play from beginning' }),
  );
  expect(onPlayFromBeginning).toHaveBeenCalledTimes(1);
  expect(onPlay).not.toHaveBeenCalled();
  await fireEvent.press(view.getByRole('button', { name: 'Close' }));
  await fireEvent.press(view.getByRole('button', { name: 'Resume' }));
  expect(onPlay).toHaveBeenCalledTimes(1);
});
it.each([
  { ...routeItem, progress: { ...routeItem.progress, seconds: 0 } },
  { ...routeItem, kind: 'book' as const },
])('omits restart when resume is unavailable', async (item) => {
  const view = await render(
    <DetailView
      item={item}
      headers={{}}
      mediaURL={(path) => path}
      onBack={() => {}}
      onPlay={() => {}}
      onPlayFromBeginning={() => {}}
    />,
  );
  const more = view.queryByRole('button', { name: 'More title actions' });
  if (more) await fireEvent.press(more);
  expect(
    view.queryByRole('button', { name: 'Play from beginning' }),
  ).toBeNull();
});
