import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { PhotoView } from './photo-view';
import { routeItem } from '@/testing/route-fixtures';

it('displays authenticated original photos with contain sizing and retry', async () => {
  const back = jest.fn();
  const headers = { Authorization: 'Bearer viewer' };
  const view = await render(
    <PhotoView
      item={{ ...routeItem, kind: 'photo' }}
      uri="https://kino.example/media/photo"
      headers={headers}
      onBack={back}
    />,
  );
  const photo = view.getByLabelText(routeItem.title);
  expect(photo.props.source).toMatchObject([{
    uri: 'https://kino.example/media/photo',
    headers,
    cacheKey: expect.stringContaining('https://kino.example/media/photo'),
  }]);
  expect(photo.props.contentFit).toBe('contain');
  await fireEvent(photo, 'error', { nativeEvent: { error: 'unavailable' } });
  expect(view.getByText('Could not load this photo.')).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Try again' }));
  expect(view.getByText('Loading photo…')).toBeTruthy();
  await fireEvent(view.getByLabelText(routeItem.title), 'load', { nativeEvent: {} });
  expect(view.queryByText('Loading photo…')).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Back' }));
  expect(back).toHaveBeenCalledTimes(1);
});
