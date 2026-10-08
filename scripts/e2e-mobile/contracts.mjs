export function validateRun(input) {
  if (!input || Object.keys(input).sort().join(',') !== 'device,platform,port,revision,run' || Object.values(input).some(value => typeof value !== 'string' || value.length > 2048)) throw new Error('invalid mobile run fields');
  const { platform, device, port, revision, run } = input;
  if (!['ios', 'android'].includes(platform) || !/^[a-f0-9]{40}$/.test(revision) || !/^\d{1,20}-\d{1,5}$/.test(run) || !/^\d{4,5}$/.test(port) || +port < 1024 || +port > 65535) throw new Error('invalid mobile run identity');
  if (platform === 'ios' ? !/^[A-Fa-f0-9]{8}(?:-[A-Fa-f0-9]{4}){3}-[A-Fa-f0-9]{12}$/.test(device) : !/^emulator-\d{4}$/.test(device) || +device.slice(9) % 2 !== 0) throw new Error('explicit owned simulator/emulator required');
  return input;
}
export function parseCode(values) {
  if (!Array.isArray(values) || values.length > 64 || values.some(value => typeof value !== 'string' || value.length > 2048)) throw new Error('invalid approval screen');
  const codes = values.map(value => /^(?:Approval code )?([0-9](?: ?[0-9]){5})$/.exec(value)?.[1]?.replaceAll(' ', '')).filter(Boolean);
  if (codes.length !== 1) throw new Error('approval screen must contain one code');
  return codes[0];
}
