import {execFileSync} from 'node:child_process';
import {createServer, createConnection} from 'node:net';
import {networkInterfaces} from 'node:os';
import {createHash} from 'node:crypto';

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
function inspect(engine, args, deadline = Date.now() + 14000) {
  let raw;
  try { raw = execFileSync(engine, args, {encoding: 'utf8', timeout: Math.min(3000, Math.max(1, deadline - Date.now())), maxBuffer: 65536}); }
  catch { invalid(); }
  if (Date.now() >= deadline) invalid();
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > 65536) invalid();
  let value;
  try { value = JSON.parse(raw); } catch { invalid(); }
  // Templates emit canonical JSON: reject duplicate keys and ambiguous encodings.
  if (JSON.stringify(value) !== raw.trim()) invalid();
  return value;
}
const containerFormat = '{"owner":{{json (index .Config.Labels "org.kinosail.fixture-owner")}},"id":{{json .Id}},"name":{{json .Name}},"image":{{json .Image}},"running":{{json .State.Running}},"networks":[{{ $first := true }}{{range $name,$network := .NetworkSettings.Networks}}{{if not $first}},{{end}}{{ $first = false }}{"name":{{json $name}},"id":{{json $network.NetworkID}},"ip":{{json $network.IPAddress}}}{{end}}]}';
const networkFormat = '{"owner":{{json (index .Labels "org.kinosail.fixture-owner")}},"bridge":{{json (index .Options "com.docker.network.bridge.name")}},"gateway":{{json (index .IPAM.Config 0).Gateway}},"id":{{json .Id}},"name":{{json .Name}},"internal":{{json .Internal}},"driver":{{json .Driver}},"subnets":[{{range $i,$config := .IPAM.Config}}{{if $i}},{{end}}{{json $config.Subnet}}{{end}}],"members":[{{ $first := true }}{{range $id,$member := .Containers}}{{if not $first}},{{end}}{{ $first = false }}{"id":{{json $id}},"name":{{json $member.Name}},"address":{{json $member.IPv4Address}}}{{end}}]}';

export function localDocker(engine, deadline = Date.now() + 14000) {
  const env = process.env;
  if (process.platform !== 'linux' || engine !== 'docker'
      || env.DOCKER_HOST && env.DOCKER_HOST !== 'unix:///var/run/docker.sock'
      || env.DOCKER_CONTEXT && env.DOCKER_CONTEXT !== 'default'
      || env.DOCKER_TLS_VERIFY || env.DOCKER_CERT_PATH) invalid();
  let context;
  try { context = execFileSync(engine, ['context', 'show'], {encoding: 'utf8', timeout: Math.min(3000, Math.max(1, deadline - Date.now())), maxBuffer: 64}); }
  catch { invalid(); }
  if (context.trim() !== 'default' || Date.now() >= deadline) invalid();
  if (inspect(engine, ['context', 'inspect', 'default', '--format', '{{json .Endpoints.docker.Host}}'], deadline)
      !== 'unix:///var/run/docker.sock') invalid();
}

export function ownedResource(engine, kind, name, token, expected) {
  if (engine !== 'docker' || !['container', 'network', 'volume'].includes(kind)
      || typeof name !== 'string' || !/^kinosail-library-(?:(?:config|cache|backups)-)?[0-9]{1,12}-[0-9]{1,5}$/.test(name)
      || !/^[a-f0-9]{32}$/.test(token) || expected !== '-' && !/^[a-f0-9]{64}$/.test(expected)) invalid();
  const hasRole = /^kinosail-library-(?:config|cache|backups)-/.test(name);
  if ((kind === 'volume') !== hasRole) invalid();
  const label = kind === 'container' ? '.Config.Labels' : '.Labels';
  const identity = kind === 'volume' ? '.Name' : '.Id';
  const format = '{"id":{{json ' + identity + '}},"name":{{json .Name}},"owner":{{json (index ' + label + ' "org.kinosail.fixture-owner")}}'
      + (kind === 'volume' ? ',"created":{{json .CreatedAt}}}' : '}');
  const args = kind === 'container' ? ['inspect'] : [kind, 'inspect'];
  const value = inspect(engine, [...args, '--format', format, name]);
  keys(value, kind === 'volume' ? ['id', 'name', 'owner', 'created'] : ['id', 'name', 'owner']);
  if (value.owner !== token || ![name, '/' + name].includes(value.name)) invalid();
  let id = value.id;
  if (kind === 'volume') {
    if (id !== name || typeof value.created !== 'string' || value.created.length > 64
        || !/^[-0-9T:.+Z]+$/.test(value.created) || !Number.isFinite(Date.parse(value.created))) invalid();
    id = createHash('sha256').update(JSON.stringify([name, token, value.created])).digest('hex');
  }
  if (!/^[a-f0-9]{64}$/.test(id) || expected !== '-' && expected !== id) invalid();
  return id;
}

export function inspectTarget(engine, containerName, networkName, image, token, facts = {}, deadline = Date.now() + 14000) {
  if (process.platform !== 'linux' || !['docker', 'podman'].includes(engine)
      || typeof containerName !== 'string' || !/^kinosail-library-[0-9]{1,12}-[0-9]{1,5}$/.test(containerName)
      || networkName !== containerName || !/^sha256:[a-f0-9]{64}$/.test(image) || !/^[a-f0-9]{32}$/.test(token)) invalid();
  localDocker(engine, deadline);
  const container = inspect(engine, ['inspect', '--format', containerFormat, containerName], deadline);
  keys(container, ['owner', 'id', 'name', 'image', 'running', 'networks']);
  if (typeof container.running === 'boolean') facts.containerRunning = container.running;
  if (container.owner !== token || !/^[a-f0-9]{64}$/.test(container.id) || ![containerName, '/' + containerName].includes(container.name)
      || container.image !== image || container.running !== true
      || !Array.isArray(container.networks) || container.networks.length !== 1) invalid();
  const joined = container.networks[0];
  keys(joined, ['name', 'id', 'ip']);
  if (joined.name !== networkName || !/^[a-f0-9]{64}$/.test(joined.id)) invalid();
  privateAddress(joined.ip);
  const network = inspect(engine, ['network', 'inspect', '--format', networkFormat, networkName], deadline);
  keys(network, ['owner', 'bridge', 'gateway', 'id', 'name', 'internal', 'driver', 'subnets', 'members']);
  if (typeof network.internal === 'boolean') facts.networkInternal = network.internal;
  if (network.owner !== token || network.id !== joined.id || network.name !== networkName || network.internal !== true
      || network.driver !== 'bridge' || !Array.isArray(network.subnets) || network.subnets.length !== 1
      || !Array.isArray(network.members) || network.members.length !== 1) invalid();
  const prefix = subnet(network.subnets[0], joined.ip), member = network.members[0];
  keys(member, ['id', 'name', 'address']);
  if (member.id !== container.id || member.name !== containerName || member.address !== joined.ip + '/' + prefix) invalid();
  const bridge = 'br-' + network.id.slice(0, 12);
  if (network.bridge !== null && network.bridge !== '' && network.bridge !== bridge) invalid();
  privateAddress(network.gateway);
  if (network.gateway === joined.ip || subnet(network.subnets[0], network.gateway) !== prefix) invalid();
  const interfaces = networkInterfaces()[bridge];
  if (!Array.isArray(interfaces)) invalid();
  const ipv4 = interfaces.filter(value => value.family === 'IPv4');
  if (ipv4.length !== 1 || ipv4[0].internal !== false || ipv4[0].address !== network.gateway
      || ipv4[0].cidr !== network.gateway + '/' + prefix) invalid();
  facts.targetAdmitted = true;
  return {ip: joined.ip};
}

export function startRelay(target, deadline = Date.now() + 2000) {
  // Only the admitted internal address may reach the fixed application port.
  keys(target, ['ip']);
  privateAddress(target.ip);
  if (!Number.isFinite(deadline) || deadline <= Date.now() || deadline > Date.now() + 14000) invalid();
  return new Promise((resolve, reject) => {
    const sockets = new Set();
    const codes = ['ECONNRESET', 'ECONNREFUSED', 'ETIMEDOUT', 'EPIPE', 'ENETUNREACH', 'EHOSTUNREACH', 'unknown'];
    const transport = {schemaVersion: 1, connections: 0, connected: 0, capacityRejected: 0, connectDeadline: 0,
      clientClosed: 0, upstreamClosed: 0, clientBytes: 0, upstreamBytes: 0, overflow: false,
      errors: {client: Object.fromEntries(codes.map(code => [code, 0])), upstream: Object.fromEntries(codes.map(code => [code, 0]))}};
    const count = (object, key, amount = 1) => {
      const next = object[key] + amount;
      if (next > 2147483647) transport.overflow = true;
      object[key] = Math.min(next, 2147483647);
    };
    const error = (side, failure) => {
      let code;
      try { code = failure?.code; } catch {}
      count(transport.errors[side], codes.includes(code) ? code : 'unknown');
    };
    const snapshot = () => JSON.parse(JSON.stringify(transport));
    let settled = false, timer;
    const server = createServer({allowHalfOpen: true}, client => {
      if (sockets.size >= 128) { count(transport, 'capacityRejected'); client.destroy(); return; }
      count(transport, 'connections');
      const peer = createConnection({host: target.ip, port: 38127, allowHalfOpen: true});
      sockets.add(client); sockets.add(peer);
      const stop = () => { client.destroy(); peer.destroy(); sockets.delete(client); sockets.delete(peer); };
      client.on('error', failure => { error('client', failure); stop(); });
      peer.on('error', failure => { error('upstream', failure); stop(); });
      client.on('close', () => { count(transport, 'clientClosed'); sockets.delete(client); });
      peer.on('close', () => { count(transport, 'upstreamClosed'); sockets.delete(peer); });
      client.on('data', chunk => { if (Buffer.isBuffer(chunk)) count(transport, 'clientBytes', chunk.length); });
      peer.on('data', chunk => { if (Buffer.isBuffer(chunk)) count(transport, 'upstreamBytes', chunk.length); });
      peer.setTimeout(5000, () => { count(transport, 'connectDeadline'); stop(); });
      peer.once('connect', () => { count(transport, 'connected'); peer.setTimeout(0); });
      client.pipe(peer); peer.pipe(client);
    });
    const close = () => {
      for (const socket of sockets) socket.destroy();
      sockets.clear();
      return new Promise(done => { try { server.close(done); } catch { done(); } });
    };
    const rejectStartup = message => {
      if (settled) return;
      settled = true; clearTimeout(timer);
      server.removeListener('error', failed);
      void close().then(() => reject(new Error(message)));
    };
    const failed = () => rejectStartup('owned Library relay listen failed');
    timer = setTimeout(() => rejectStartup('owned Library relay startup deadline'), deadline - Date.now());
    server.once('error', failed);
    server.listen(0, '127.0.0.1', () => {
      if (settled) { void close(); return; }
      settled = true; clearTimeout(timer);
      server.removeListener('error', failed);
      server.on('error', () => { void close(); });
      resolve({port: server.address().port, close, snapshot});
    });
  });
}

if (process.argv[1]?.endsWith('/library-internal-relay.mjs')) {
  const facts = {schemaVersion: 1, containerRunning: null, networkInternal: null, targetAdmitted: false};
  try {
    if (process.argv[2] === 'topology') {
      if (process.argv.length !== 4) invalid();
      localDocker(process.argv[3]);
      process.stdout.write('local-rootful-docker\n');
    } else if (process.argv[2] === 'owned') {
      if (process.argv.length !== 8) invalid();
      process.stdout.write(ownedResource(...process.argv.slice(3)) + '\n');
    } else {
    if (process.argv.length !== 7) invalid();
    const deadline = Date.now() + 14000;
    const target = inspectTarget(...process.argv.slice(2), facts, deadline);
    const relay = await startRelay(target, deadline);
    let closing = false;
    const close = () => { if (!closing) { closing = true; void relay.close().then(() => { process.stderr.write(JSON.stringify({...facts, transport: relay.snapshot()}) + '\n', () => process.exit(0)); }); } };
    process.once('SIGTERM', close); process.once('SIGINT', close);
    process.stderr.write(JSON.stringify(facts) + '\n');
    process.stdout.write(JSON.stringify({schemaVersion: 1, port: relay.port}) + '\n');
    }
  } catch {
    process.stderr.write(JSON.stringify(facts) + '\n');
    process.exitCode = 2;
  }
}
