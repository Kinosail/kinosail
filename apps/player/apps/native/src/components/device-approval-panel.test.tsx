import React from 'react';
import { act, fireEvent, render } from '@testing-library/react-native';
import { DeviceApprovalPanel } from './device-approval-panel';

const request = () => ({
  code: '123456',
  device: 'Kinosail TV',
  expiresAt: new Date(Date.now() + 300000).toISOString(),
});
const client = () => ({
  previewDevice: jest.fn().mockImplementation(() => Promise.resolve(request())),
  loadViewer: jest.fn().mockResolvedValue({
    server: 'Nox',
    viewer: { id: 'viewer', name: 'Mike', owner: true },
  }),
  approveDevice: jest.fn().mockResolvedValue(undefined),
});

it('shows the device, matching code, and profile before approval', async () => {
  const api = client(),
    close = jest.fn();
  const view = await render(
    <DeviceApprovalPanel client={api} code="123456" onClose={close} />,
  );
  expect(view.getByText('Kinosail TV')).toBeTruthy();
  expect(view.getByLabelText('Code 1 2 3 4 5 6')).toBeTruthy();
  expect(api.approveDevice).not.toHaveBeenCalled();
  await fireEvent.press(view.getByRole('button', { name: 'Approve as Mike' }));
  expect(api.approveDevice).toHaveBeenCalledTimes(1);
  expect(api.approveDevice).toHaveBeenCalledWith('123456');
  expect(view.getByText('TV approved')).toBeTruthy();
  await fireEvent.press(view.getByRole('button', { name: 'Done' }));
  expect(close).toHaveBeenCalledTimes(1);
});

it('dismisses without approving and blocks unavailable requests', async () => {
  const api = client(),
    close = jest.fn();
  const view = await render(
    <DeviceApprovalPanel client={api} code="123456" onClose={close} />,
  );
  await fireEvent.press(view.getByRole('button', { name: 'Not now' }));
  expect(api.approveDevice).not.toHaveBeenCalled();
  expect(close).toHaveBeenCalledTimes(1);
  api.previewDevice.mockRejectedValue(new Error('expired'));
  await view.rerender(
    <DeviceApprovalPanel client={api} code="654321" onClose={close} />,
  );
  expect(view.queryByRole('button', { name: 'Approve as Mike' })).toBeNull();
  expect(view.getByText(/request is unavailable/)).toBeTruthy();
});

it('does not send duplicate approvals while a request is in flight', async () => {
  const api = client();
  let finish!: () => void;
  api.approveDevice.mockImplementation(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
  );
  const view = await render(
    <DeviceApprovalPanel client={api} code="123456" onClose={jest.fn()} />,
  );
  const approve = view.getByRole('button', { name: 'Approve as Mike' });
  await fireEvent.press(approve);
  await fireEvent.press(approve);
  expect(api.approveDevice).toHaveBeenCalledTimes(1);
  await act(async () => finish());
});

it('requires a fresh preview after switching the signed-in client', async () => {
  const first = client(),
    second = client();
  second.previewDevice.mockRejectedValue(new Error('different server'));
  const view = await render(
    <DeviceApprovalPanel client={first} code="123456" onClose={jest.fn()} />,
  );
  await view.rerender(
    <DeviceApprovalPanel client={second} code="123456" onClose={jest.fn()} />,
  );
  expect(view.queryByRole('button', { name: 'Approve as Mike' })).toBeNull();
  expect(first.approveDevice).not.toHaveBeenCalled();
  expect(second.approveDevice).not.toHaveBeenCalled();
});
