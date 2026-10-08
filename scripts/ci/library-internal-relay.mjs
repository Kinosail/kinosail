import {execFileSync} from 'node:child_process';
import {createServer, createConnection} from 'node:net';

const invalid = () => { throw new Error('invalid owned Library relay'); };
const keys = (value, expected) => {
  if (!value || typeof value !== 'object' || Array.isArray(value)
      || Object.keys(value).sort().join(',') !== expected.sort().join(',')) invalid();
};
function address(value) {
  if (typeof value !== 'string' || !/^(?:0|[1-9][0-9]{0,2})(?:\.(?:0|[1-9][0-9]{0,2})){3}$/.test(value)) invalid();
  const octets = value.split('.').map(Number);
  if (octets.some(part => part > 255)) invalid();
  return octets.reduce((result, part) => result * 256 + part, 0);
}
function privateAddress(value) {
  address(value);
  const octets = value.split('.').map(Number);
  if (!(octets[0] === 10 || octets[0] === 172 && octets[1] >= 16 && octets[1] <= 31
      || octets[0] === 192 && octets[1] === 168)) invalid();
}
function subnet(value, ip) {
  if (typeof value !== 'string' || !/^\S+\/(?:[89]|[12][0-9]|30)$/.test(value)) invalid();
  const [base, prefix] = value.split('/'), size = 2 ** (32 - Number(prefix));
  const start = address(base), target = address(ip);
  if (start % size !== 0 || target <= start || target >= start + size - 1) invalid();
  return prefix;
}
function inspect(engine, args) {
  let raw;
  try { raw = execFileSync(engine, args, {encoding: 'utf8', timeout: 3000, maxBuffer: 65536}); }
  catch { invalid(); }
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > 65536) invalid();
  let value;
  try { value = JSON.parse(raw); } catch { invalid(); }
  // Templates emit canonical JSON: reject duplicate keys and ambiguous encodings.
  if (JSON.stringify(value) !== raw.trim()) invalid();
  return value;
}
const containerFormat = '{"id":{{json .Id}},"name":{{json .Name}},"image":{{json .Image}},"running":{{json .State.Running}},"networks":[{{ $first := true }}{{range $name,$network := .NetworkSettings.Networks}}{{if not $first}},{{end}}{{ $first = false }}{"name":{{json $name}},"id":{{json $network.NetworkID}},"ip":{{json $network.IPAddress}}}{{end}}]}';
const networkFormat = '{"id":{{json .Id}},"name":{{json .Name}},"internal":{{json .Internal}},"driver":{{json .Driver}},"subnets":[{{range $i,$config := .IPAM.Config}}{{if $i}},{{end}}{{json $config.Subnet}}{{end}}],"members":[{{ $first := true }}{{range $id,$member := .Containers}}{{if not $first}},{{end}}{{ $first = false }}{"id":{{json $id}},"name":{{json $member.Name}},"address":{{json $member.IPv4Address}}}{{end}}]}';

export function inspectTarget(engine, containerName, networkName, image, facts = {}) {
  if (process.platform !== 'linux' || !['docker', 'podman'].includes(engine)
      || typeof containerName !== 'string' || !/^kinosail-library-[0-9]{1,12}-[0-9]{1,5}$/.test(containerName)
      || networkName !== containerName || !/^sha256:[a-f0-9]{64}$/.test(image)) invalid();
  const container = inspect(engine, ['inspect', '--format', containerFormat, containerName]);
  keys(container, ['id', 'name', 'image', 'running', 'networks']);
  if (typeof container.running === 'boolean') facts.containerRunning = container.running;
  if (!/^[a-f0-9]{64}$/.test(container.id) || ![containerName, '/' + containerName].includes(container.name)
      || container.image !== image || container.running !== true
      || !Array.isArray(container.networks) || container.networks.length !== 1) invalid();
  const joined = container.networks[0];
  keys(joined, ['name', 'id', 'ip']);
  if (joined.name !== networkName || !/^[a-f0-9]{64}$/.test(joined.id)) invalid();
  privateAddress(joined.ip);
  const network = inspect(engine, ['network', 'inspect', '--format', networkFormat, networkName]);
  keys(network, ['id', 'name', 'internal', 'driver', 'subnets', 'members']);
  if (typeof network.internal === 'boolean') facts.networkInternal = network.internal;
  if (network.id !== joined.id || network.name !== networkName || network.internal !== true
      || network.driver !== 'bridge' || !Array.isArray(network.subnets) || network.subnets.length !== 1
      || !Array.isArray(network.members) || network.members.length !== 1) invalid();
  const prefix = subnet(network.subnets[0], joined.ip), member = network.members[0];
  keys(member, ['id', 'name', 'address']);
  if (member.id !== container.id || member.name !== containerName || member.address !== joined.ip + '/' + prefix) invalid();
  facts.targetAdmitted = true;
  return {ip: joined.ip};
}

export function startRelay(target) {
  // Only the admitted internal address may reach the fixed application port.
  keys(target, ['ip']);
  privateAddress(target.ip);
  return new Promise((resolve, reject) => {
    const sockets = new Set();
    const server = createServer(client => {
      if (sockets.size >= 128) { client.destroy(); return; }
      const peer = createConnection({host: target.ip, port: 38127});
      sockets.add(client); sockets.add(peer);
      const stop = () => { client.destroy(); peer.destroy(); sockets.delete(client); sockets.delete(peer); };
      client.on('error', stop); peer.on('error', stop);
      client.on('close', stop); peer.on('close', stop);
      peer.setTimeout(5000, stop);
      peer.once('connect', () => peer.setTimeout(0));
      client.pipe(peer); peer.pipe(client);
    });
    const close = () => {
      for (const socket of sockets) socket.destroy();
      sockets.clear();
      return new Promise(done => { if (server.listening) server.close(done); else done(); });
    };
    const failed = () => {
      server.removeListener('error', failed);
      void close().then(() => reject(new Error('owned Library relay listen failed')));
    };
    server.once('error', failed);
    server.listen(0, '127.0.0.1', () => {
      server.removeListener('error', failed);
      server.on('error', () => { void close(); });
      resolve({port: server.address().port, close});
    });
  });
}

if (process.argv[1]?.endsWith('/library-internal-relay.mjs')) {
  const facts = {schemaVersion: 1, containerRunning: null, networkInternal: null, targetAdmitted: false};
  try {
    if (process.argv.length !== 6) invalid();
    const target = inspectTarget(...process.argv.slice(2), facts);
    const relay = await startRelay(target);
    let closing = false;
    const close = () => { if (!closing) { closing = true; void relay.close().then(() => process.exit(0)); } };
    process.once('SIGTERM', close); process.once('SIGINT', close);
    process.stderr.write(JSON.stringify(facts) + '\n');
    process.stdout.write(JSON.stringify({schemaVersion: 1, port: relay.port}) + '\n');
  } catch {
    process.stderr.write(JSON.stringify(facts) + '\n');
    process.exitCode = 2;
  }
}
