import { test as base } from '@playwright/test';
import { connect, createServer, type Socket } from 'node:net';

type Connection = { url: string; disconnect: () => Promise<void> };

// Close the real connection to the fixture. WebKit's simulated offline mode also
// blocks service-worker navigation, including a minimal non-Kinosail test page.
export const test = base.extend<{ connection: Connection }>({
  connection: async ({}, use) => {
    const target = new URL(process.env.KINOSAIL_E2E_URL ?? 'https://127.0.0.1:38127');
    const sockets = new Set<Socket>();
    const server = createServer((client) => {
      const upstream = connect(Number(target.port || (target.protocol === 'https:' ? 443 : 80)), target.hostname);
      for (const socket of [client, upstream]) {
        sockets.add(socket);
        socket.on('close', () => sockets.delete(socket));
        socket.on('error', () => { client.destroy(); upstream.destroy(); });
      }
      client.pipe(upstream).pipe(client);
    });
    await new Promise<void>((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('test connection did not bind');
    const disconnect = async () => {
      if (!server.listening) return;
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve) => server.close(() => resolve()));
    };
    try { await use({ url: `${target.protocol}//127.0.0.1:${address.port}`, disconnect }); }
    finally { await disconnect(); }
  },
  baseURL: async ({ connection }, use) => use(connection.url),
});
