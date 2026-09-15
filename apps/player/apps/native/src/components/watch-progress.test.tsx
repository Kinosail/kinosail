import { act, render, waitFor } from '@testing-library/react-native';
import React from 'react';
import { WatchProgress, parseWatchProgress } from './watch-progress';

const mediaURL = (path: string) => `https://kino.example${path}`;
const response = (value: unknown) =>
  new Response(JSON.stringify(value), {
    headers: { 'Content-Type': 'application/json' },
  });
afterEach(() => jest.restoreAllMocks());
it.each([
  null,
  [],
  {},
  { seconds: 10 },
  { seconds: 10, duration: 100, extra: 1 },
  { seconds: -1, duration: 100 },
  { seconds: '10', duration: 100 },
  { seconds: NaN, duration: 100 },
  { seconds: 101, duration: 100 },
  { seconds: 0, duration: 315360001 },
])('rejects invalid progress %#', (value) => {
  expect(parseWatchProgress(value)).toBeNull();
});
it('shows real progress and remaining minutes using the original accent', async () => {
  jest
    .spyOn(global, 'fetch')
    .mockResolvedValue(response({ seconds: 1200, duration: 3600 }));
  const view = await render(
    <WatchProgress id="movie" seconds={1200} mediaURL={mediaURL} />,
  );
  await waitFor(() =>
    expect(view.getByRole('progressbar').props.accessibilityValue).toEqual({
      min: 0,
      max: 100,
      now: 33,
      text: '40 min left',
    }),
  );
  expect(view.getByText('40 min left')).toBeTruthy();
});
it('shows elapsed time without inventing a percentage for unknown runtime', async () => {
  jest
    .spyOn(global, 'fetch')
    .mockResolvedValue(response({ seconds: 1200, duration: 0 }));
  const view = await render(
    <WatchProgress id="movie" seconds={1200} mediaURL={mediaURL} />,
  );
  await waitFor(() => expect(view.getByText('20 min watched')).toBeTruthy());
  expect(view.queryByRole('progressbar')).toBeNull();
});
it('rejects malformed IDs before requesting progress', async () => {
  const fetch = jest.spyOn(global, 'fetch');
  const view = await render(
    <WatchProgress id="../secret" mediaURL={mediaURL} />,
  );
  expect(fetch).not.toHaveBeenCalled();
  expect(view.queryByRole('progressbar')).toBeNull();
});
it('does not show the previous title after a slow response arrives', async () => {
  let finish!: (value: Response) => void;
  jest
    .spyOn(global, 'fetch')
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    )
    .mockResolvedValueOnce(response({ seconds: 60, duration: 600 }));
  const view = await render(<WatchProgress id="first" mediaURL={mediaURL} />);
  await view.rerender(<WatchProgress id="second" mediaURL={mediaURL} />);
  await waitFor(() => expect(view.getByText('9 min left')).toBeTruthy());
  await act(async () => {
    finish(response({ seconds: 1200, duration: 3600 }));
  });
  expect(view.queryByText('40 min left')).toBeNull();
  expect(view.getByText('9 min left')).toBeTruthy();
});

it.each([false, true])(
  'labels completed progress as watched (saved flag=%s)',
  async (watched) => {
    jest
      .spyOn(global, 'fetch')
      .mockResolvedValue(
        response({ seconds: watched ? 60 : 600, duration: 600 }),
      );
    const view = await render(
      <WatchProgress
        id="movie"
        seconds={600}
        watched={watched}
        mediaURL={mediaURL}
      />,
    );
    await waitFor(() => expect(view.getByText('Watched')).toBeTruthy());
    expect(view.queryByText('0 min left')).toBeNull();
  },
);
