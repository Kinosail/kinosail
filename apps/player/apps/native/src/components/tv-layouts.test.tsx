import { fireEvent, render } from '@testing-library/react-native';
import React from 'react';
import { StyleSheet } from 'react-native';

import { parseItem } from '@/core/contract';

import { DetailView } from './detail-view';
import { homeViewStyles } from './home-view.styles';
import { SetupFlow } from './setup-flow';

jest.mock('react-native/Libraries/Utilities/Platform', () => {
  const platform = jest.requireActual<{
    default: typeof import('react-native').Platform;
  }>('react-native/Libraries/Utilities/Platform');
  Object.defineProperty(platform.default, 'isTV', {
    configurable: true,
    value: true,
  });
  return platform;
});

describe('native television layout contracts', () => {
  it('keeps home headings and spacing readable at television distance', () => {
    expect(homeViewStyles.hero).toMatchObject({
      minHeight: 330,
      paddingTop: 24,
    });
    expect(homeViewStyles.heroTitle).toMatchObject({
      fontSize: 72,
      lineHeight: 80,
    });
    expect(homeViewStyles.heroSummary).toMatchObject({
      fontSize: 28,
      lineHeight: 36,
    });
    expect(homeViewStyles.shelfTitle.fontSize).toBe(32);
  });

  it('keeps detail playback and back actions available without optional metadata', async () => {
    const item = parseItem({
      item: {
        id: 'episode',
        kind: 'video',
        title: 'Visitors',
        show: 'Visitors',
        season: 2,
        episode: 3,
      },
    });
    const onBack = jest.fn();
    const onPlay = jest.fn();
    const view = await render(
      <DetailView
        item={item}
        mediaURL={(path) => path}
        headers={{}}
        onBack={onBack}
        onPlay={onPlay}
      />,
    );
    expect(view.getByText('S2 E3')).toBeTruthy();
    expect(
      view.getByText('No description is available for this title.'),
    ).toBeTruthy();
    expect(
      StyleSheet.flatten(
        view.getByRole('header', { name: 'Visitors' }).props.style,
      ),
    ).toMatchObject({ fontSize: 64, lineHeight: 72 });
    expect(view.queryByLabelText('Poster for Visitors')).toBeNull();
    expect(view.queryByLabelText('Backdrop for Visitors')).toBeNull();
    await fireEvent.press(view.getByRole('button', { name: 'Back' }));
    await fireEvent.press(view.getByRole('button', { name: 'Play' }));
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onPlay).toHaveBeenCalledTimes(1);
  });

  it('identifies a television when requesting approval', async () => {
    const startQuickConnect = jest
      .fn()
      .mockResolvedValue({ code: '123456', secret: 'challenge' });
    const view = await render(
      <SetupFlow
        createClient={() => ({
          startQuickConnect,
          cancelQuickConnect: jest.fn().mockResolvedValue(undefined),
          pollQuickConnect: jest.fn().mockResolvedValue(null),
        })}
        onConnected={jest.fn()}
      />,
    );
    expect(
      StyleSheet.flatten(
        view.getByLabelText('Kinosail Server URL').props.style,
      ),
    ).toMatchObject({ fontSize: 28, minHeight: 80 });
    expect(
      StyleSheet.flatten(
        view.getByRole('header', { name: 'Find your server' }).props.style,
      ),
    ).toMatchObject({ fontSize: 40 });
    await fireEvent.changeText(
      view.getByLabelText('Kinosail Server URL'),
      'https://kino.example',
    );
    await fireEvent.press(view.getByRole('button', { name: 'Connect' }));
    expect(startQuickConnect).toHaveBeenCalledWith('Kinosail TV');
    expect(await view.findByText('123 456')).toBeTruthy();
    expect(
      StyleSheet.flatten(view.getByText('123 456').props.style),
    ).toMatchObject({ fontSize: 72 });
    expect(view.queryByText('The shortest path to play.')).toBeNull();
  });
});
