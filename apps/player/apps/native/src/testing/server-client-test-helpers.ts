import type { Fetcher } from '../core/server-client';
import type { InputValue } from '../core/contract';

export const response = (status: number, body: InputValue) =>
  new Response([204, 205, 304].includes(status) ? null : JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });

export const fetchMock = () =>
  jest.fn<ReturnType<Fetcher>, Parameters<Fetcher>>();
