import { KinosailClient, normalizeServerURL } from './server-client';

jest.mock('./downloads', () => {
  throw new Error('Constructing an API client must not initialize native downloads.');
});

it('validates and constructs a server client without loading native transfers', () => {
  const url = normalizeServerURL(' https://kino.example/ ');
  expect(new KinosailClient(url, 'viewer').baseURL).toBe('https://kino.example');
});
