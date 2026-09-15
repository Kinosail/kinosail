import React from 'react';
import { AppState, type AppStateStatus } from 'react-native';
import { act, fireEvent, render } from '@testing-library/react-native';
import { TVApprovalPrompt } from './tv-approval-prompt';

let mockPath = '/';
const mockClient = {
  loadPendingTVs: jest.fn(),
  previewDevice: jest.fn(),
  loadViewer: jest.fn(),
  approveDevice: jest.fn(),
};
let mockSession: { client: typeof mockClient | null } = { client: mockClient };
jest.mock('@/core/session-context', () => ({ useSession: () => mockSession }));
jest.mock('expo-router', () => ({ usePathname: () => mockPath }));
jest.mock('react-native/Libraries/Utilities/Platform', () => {
  const value = jest.requireActual('react-native/Libraries/Utilities/Platform');
  Object.defineProperty(value.default, 'isTV', {
    configurable: true,
    value: false,
  });
  return value;
});
let changeState: (state: AppStateStatus) => void;
const pending = () => ({
  code: '123456',
  device: 'Kinosail TV',
  expiresAt: new Date(Date.now() + 300000).toISOString(),
});

beforeEach(() => {
  jest.useFakeTimers();
  mockPath = '/';
  mockSession = { client: mockClient };
  Object.defineProperty(AppState, 'currentState', {
    configurable: true,
    value: 'active',
  });
  jest
    .spyOn(AppState, 'addEventListener')
    .mockImplementation((_type, listener) => {
      changeState = listener;
      return { remove: jest.fn() };
    });
  const request = pending();
  mockClient.loadPendingTVs.mockReset().mockResolvedValue([request]);
  mockClient.previewDevice.mockReset().mockResolvedValue(request);
  mockClient.loadViewer
    .mockReset()
    .mockResolvedValue({ server: 'Server', viewer: { name: 'Mike' } });
  mockClient.approveDevice.mockReset().mockResolvedValue(undefined);
});
afterEach(() => {
  jest.restoreAllMocks();
  jest.useRealTimers();
});

it('shows an explicit foreground prompt and does not repeat a dismissed request', async () => {
  const view = await render(<TVApprovalPrompt />);
  expect(view.getByRole('button', { name: 'Approve as Mike' })).toBeTruthy();
  expect(mockClient.approveDevice).not.toHaveBeenCalled();
  await fireEvent.press(view.getByRole('button', { name: 'Not now' }));
  await act(async () => {
    jest.advanceTimersByTime(5000);
  });
  expect(view.queryByRole('button', { name: 'Approve as Mike' })).toBeNull();
  expect(mockClient.approveDevice).not.toHaveBeenCalled();
});

it('stops polling and closes the prompt in the background', async () => {
  const view = await render(<TVApprovalPrompt />);
  await act(async () => changeState('background'));
  const calls = mockClient.loadPendingTVs.mock.calls.length;
  await act(async () => {
    jest.advanceTimersByTime(30000);
  });
  expect(mockClient.loadPendingTVs).toHaveBeenCalledTimes(calls);
  expect(view.queryByRole('button', { name: 'Approve as Mike' })).toBeNull();
  await act(async () => changeState('active'));
  expect(mockClient.loadPendingTVs).toHaveBeenCalledTimes(calls + 1);
});

it.each(['/approve', '/watch/movie', '/read/book'])(
  'does not interrupt %s',
  async (path) => {
    mockPath = path;
    await render(<TVApprovalPrompt />);
    expect(mockClient.loadPendingTVs).not.toHaveBeenCalled();
  },
);

it('removes withdrawn requests and never polls after sign-out', async () => {
  const view = await render(<TVApprovalPrompt />);
  mockClient.loadPendingTVs.mockResolvedValue([]);
  await act(async () => {
    jest.advanceTimersByTime(5000);
  });
  expect(view.queryByRole('button', { name: 'Approve as Mike' })).toBeNull();
  mockSession = { client: null };
  await view.rerender(<TVApprovalPrompt />);
  const calls = mockClient.loadPendingTVs.mock.calls.length;
  await act(async () => {
    jest.advanceTimersByTime(15000);
  });
  expect(mockClient.loadPendingTVs).toHaveBeenCalledTimes(calls);
});
