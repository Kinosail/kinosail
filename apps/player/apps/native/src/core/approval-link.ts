import { normalizeServerURL } from './server-client';
import { approvalCode } from './device-approval';

export function approvalURL(server: string, code: string): string {
  return `${normalizeServerURL(server)}/connect?code=${approvalCode(code)}`;
}

// A scanned link can select a request, but never change the signed-in Server.
export function readApprovalLink(
  params: Record<string, string | string[] | undefined>,
  currentServer: string,
): string {
  if (Object.keys(params).some((key) => key !== 'server' && key !== 'code'))
    throw new Error('This sign-in link is invalid. Scan the TV again.');
  if (
    typeof params.server !== 'string' ||
    params.server.length > 2048 ||
    normalizeServerURL(params.server) !== normalizeServerURL(currentServer)
  )
    throw new Error(
      'This TV uses a different Server. Open the Player app connected to that Server, or approve in your browser.',
    );
  return approvalCode(params.code);
}
