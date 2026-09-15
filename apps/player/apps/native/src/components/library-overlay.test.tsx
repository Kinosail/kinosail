import React from 'react';
import { fireEvent, render } from '@testing-library/react-native';
import { StyleSheet } from 'react-native';
import { LibraryView } from './library-view';
import { routeItem } from '@/testing/route-fixtures';

it('moves the first result below a resized header and reserves space for floating controls', async () => {
  const view = await render(
    <LibraryView
      items={[]}
      total={0}
      query={{ view: 'books' }}
      busy={false}
      error=""
      hasMore={false}
      mediaURL={(path) => path}
      headers={{}}
      onChange={jest.fn()}
      onOpen={jest.fn()}
      onMore={jest.fn()}
      onPrevious={jest.fn()}
      onRetry={jest.fn()}
      onBack={jest.fn()}
    />,
  );
  const header = view.container
    .queryAll(() => true)
    .find(
      (node) =>
        typeof node.props.onLayout === 'function' &&
        StyleSheet.flatten(node.props.style)?.zIndex === 2,
    )!;
  await fireEvent(header, 'layout', {
    nativeEvent: { layout: { height: 200 } },
  });
  const content = StyleSheet.flatten(
    view.getByLabelText('Library results').props.contentContainerStyle,
  );
  expect(content.paddingTop).toBe(208);
  expect(content.paddingBottom).toBeGreaterThanOrEqual(160);
  expect(view.getByRole('header', { name: 'Ebooks' })).toBeTruthy();
  expect(view.getByText('No titles in this library yet.')).toBeTruthy();
});

it.each(['books', 'audiobooks'] as const)(
  'keeps the book type switch visible outside search for %s',
  async (selected) => {
    const onChange = jest.fn();
    const view = await render(
      <LibraryView
        items={[]}
        total={0}
        query={{ view: selected, offset: 60, q: 'old' }}
        busy={false}
        error=""
        hasMore={false}
        mediaURL={(path) => path}
        headers={{}}
        onChange={onChange}
        onOpen={jest.fn()}
        onMore={jest.fn()}
        onPrevious={jest.fn()}
        onRetry={jest.fn()}
        onBack={jest.fn()}
      />,
    );
    const target = selected === 'books' ? 'Audiobooks' : 'Ebooks';
    await fireEvent.press(view.getByRole('button', { name: target }));
    expect(onChange).toHaveBeenCalledWith({
      view: selected === 'books' ? 'audiobooks' : 'books',
      offset: 0,
      q: '',
    });
  },
);

it.each(['Close', 'Show 9 titles'])(
  'dismisses search with %s without changing filters',
  async (label) => {
    const onCloseSearch = jest.fn();
    const onChange = jest.fn();
    const view = await render(
      <LibraryView
        searchOpen
        onCloseSearch={onCloseSearch}
        items={[]}
        total={9}
        query={{ view: 'all', q: 'Arrival' }}
        busy={false}
        error=""
        hasMore={false}
        mediaURL={(path) => path}
        headers={{}}
        onChange={onChange}
        onOpen={jest.fn()}
        onMore={jest.fn()}
        onPrevious={jest.fn()}
        onRetry={jest.fn()}
        onBack={jest.fn()}
      />,
    );
    await fireEvent.press(view.getByRole('button', { name: label }));
    expect(onCloseSearch).toHaveBeenCalledTimes(1);
    expect(onChange).not.toHaveBeenCalled();
  },
);

const libraryProps = () => ({
  items: [],
  total: 0,
  query: { view: 'all' as const },
  busy: false,
  error: '',
  hasMore: false,
  mediaURL: (path: string) => path,
  headers: {},
  onChange: jest.fn(),
  onOpen: jest.fn(),
  onMore: jest.fn(),
  onPrevious: jest.fn(),
  onRetry: jest.fn(),
  onBack: jest.fn(),
});
it('distinguishes a failed library from zero results and offers offline recovery', async () => {
  const props = libraryProps(),
    onDownloads = jest.fn();
  const view = await render(
    <LibraryView
      {...props}
      error="Could not reach Kinosail Server."
      onDownloads={onDownloads}
    />,
  );
  expect(view.getByText('Library unavailable')).toBeTruthy();
  expect(view.queryByText('0 titles')).toBeNull();
  await fireEvent.press(view.getByRole('button', { name: 'Go to Downloads' }));
  expect(onDownloads).toHaveBeenCalledTimes(1);
});
it('selects sort directly and clears an over-filtered query', async () => {
  const props = libraryProps();
  const view = await render(
    <LibraryView
      {...props}
      searchOpen
      query={{ view: 'unwatched', q: 'missing', sort: 'title', offset: 60 }}
    />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Year' }));
  expect(props.onChange).toHaveBeenCalledWith({
    view: 'unwatched',
    q: 'missing',
    sort: 'year',
    offset: 0,
  });
  await view.rerender(
    <LibraryView
      {...props}
      query={{ view: 'unwatched', q: 'missing', sort: 'year', offset: 60 }}
    />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Clear filters' }));
  expect(props.onChange).toHaveBeenLastCalledWith({
    view: 'all',
    q: '',
    sort: 'year',
    offset: 0,
  });
});

it.each([
  ['movies', 2 / 3],
  ['audiobooks', 1],
  ['photos', 1],
] as const)(
  'uses the %s artwork shape only while results are missing',
  async (category, aspectRatio) => {
    const props = libraryProps();
    const view = await render(
      <LibraryView {...props} busy query={{ view: category }} />,
    );
    expect(view.getAllByRole('progressbar')).toHaveLength(1);
    expect(
      StyleSheet.flatten(
        view.getAllByTestId('browse-skeleton-artwork')[0].props.style,
      ).aspectRatio,
    ).toBe(aspectRatio);
    await view.rerender(<LibraryView {...props} query={{ view: category }} />);
    expect(view.queryByRole('progressbar')).toBeNull();
  },
);

it('uses track placeholders for songs and keeps existing results during a pending request', async () => {
  const props = libraryProps();
  const view = await render(
    <LibraryView {...props} busy query={{ view: 'music' }} />,
  );
  expect(view.getAllByTestId('browse-skeleton-row')).toHaveLength(6);
  expect(view.queryAllByTestId('browse-skeleton-artwork')).toHaveLength(0);
  await view.rerender(
    <LibraryView {...props} busy items={[routeItem]} total={1} />,
  );
  expect(view.queryByRole('progressbar')).toBeNull();
  expect(
    view.getByRole('button', { name: 'Arrival, 2016, In progress' }),
  ).toBeTruthy();
});
