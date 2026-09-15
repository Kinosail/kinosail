import { KinosailClient } from './server-client';
import { fetchMock, response } from '@/testing/server-client-test-helpers';

it.each([NaN, Infinity, -0.1, 1.1, null, '0.5'])(
  'rejects offset %p before any request',
  async (offset) => {
    const fetcher = fetchMock();
    const client = new KinosailClient(
      'https://kino.example',
      'viewer',
      fetcher,
    );
    await expect(
      client.saveReaderProgress('book', 1, offset as number),
    ).rejects.toThrow();
    expect(fetcher).not.toHaveBeenCalled();
  },
);
it('sends the chapter and its position in one authenticated request', async () => {
  const fetcher = fetchMock().mockResolvedValue(
    response(200, { page: 2, total: 3, offset: 0.625 }),
  );
  const client = new KinosailClient('https://kino.example', 'viewer', fetcher);
  await expect(client.saveReaderProgress('book', 2, 0.625)).resolves.toEqual({
    page: 2,
    total: 3,
    offset: 0.625,
  });
  expect(fetcher).toHaveBeenCalledWith(
    'https://kino.example/api/v1/books/book/reader/progress?includeOffset=true',
    expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ page: 2, offset: 0.625 }),
    }),
  );
});
