// This artifact uses trusted workflow metadata only. Never read rejected private output.
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
try {
  const env=process.env, platform=env.GITHUB_JOB;
  const outcomes={native:env.NATIVE_OUTCOME,cleanup:env.CLEANUP_OUTCOME,privacy:env.PRIVACY_OUTCOME};
  if(process.argv.length!==2 || env.GITHUB_ACTIONS!=='true' || !['tvos','androidtv'].includes(platform) || typeof env.GITHUB_SHA!=='string' || !/^[a-f0-9]{40}$/.test(env.GITHUB_SHA) || typeof env.GITHUB_RUN_ID!=='string' || !/^\d{1,20}$/.test(env.GITHUB_RUN_ID) || typeof env.GITHUB_RUN_ATTEMPT!=='string' || !/^\d{1,5}$/.test(env.GITHUB_RUN_ATTEMPT) || Object.values(outcomes).some(value=>!['success','failure','cancelled','skipped'].includes(value)) || outcomes.privacy==='success') throw new Error('invalid rejection metadata');
  const receipt={revision:env.GITHUB_SHA,run:`${env.GITHUB_RUN_ID}-${env.GITHUB_RUN_ATTEMPT}`,job:platform,
    command:`python3 hosted.py ${platform}`,data:'synthetic Example Movie (testsrc2/AAC); synthetic MFA Owner; actual dynamic pairing code',
    environment:'disposable hosted simulator/emulator',result:1,outcomes,
    privacy:{privateInputsRead:false,rawPublished:false},
    boundaries:['metadata only; device/build/runtime evidence withheld','no physical device','no phone/watchOS/Wear pairing','no casting/provider calls','no deployed server']};
  const body=JSON.stringify(receipt,null,2)+'\n', root='.e2e-rejection';
  // Exclusive creation preserves an existing foreign tree or link. No cleanup follows rejection.
  mkdirSync(root,{mode:0o700});
  writeFileSync(join(root,'receipt.json'),body,{mode:0o600,flag:'wx'});
  writeFileSync(join(root,'SHA256SUMS'),`${createHash('sha256').update(body).digest('hex')}  receipt.json\n`,{mode:0o600,flag:'wx'});
  console.log('TV native rejection metadata: .e2e-rejection');
} catch {
  console.error('Rejection metadata invalid or path occupied; artifact withheld');
  process.exitCode=1;
}
