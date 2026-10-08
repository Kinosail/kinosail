"""The browser preflight owns strict navigation and closes its browser on failures.

The transport peer is deliberately not TLS/browser proof. Hosted actual Firefox
owns that proof; these controls guard lifecycle and rejection before Owner setup.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT/'apps/player/e2e/strict-firefox-probe.cjs'

class FirefoxProbe(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name); self.log=self.root/'calls.jsonl'
        shutil.copyfile(SOURCE,self.root/SOURCE.name)
        dependency=self.root/'node_modules/@playwright/test'; dependency.mkdir(parents=True)
        (dependency/'package.json').write_text('{"main":"index.cjs"}')
        (dependency/'index.cjs').write_text('''
const fs=require('node:fs');
const record=(name,value)=>fs.appendFileSync(process.env.CALLS,JSON.stringify([name,value])+'\\n');
exports.firefox={
 executablePath:()=>process.env.EXECUTABLE,
 launch:async options=>{
  record('launch',options);
  return {
   newContext:async options=>{
    record('context',options);
    return {newPage:async()=>({goto:async(url,options)=>{
     record('goto',[url,options]);
     if(process.env.FAILURE) throw new Error(process.env.FAILURE+' secret URL/password');
     return {status:()=>Number(process.env.STATUS),url:()=>process.env.RESPONSE_URL||url};
    }})};
   },
   close:async()=>{record('close');if(process.env.CLOSE_FAIL)throw new Error('secret cleanup error');}
  };
 }
};
''')
        self.env=os.environ|{'CALLS':str(self.log),'EXECUTABLE':'/owned/firefox','STATUS':'200',
            'PLAYWRIGHT_FIREFOX_POLICIES_JSON':'/owned/policies.json'}

    def run_probe(self, url='https://localhost:1234', **env):
        result=subprocess.run(['node',str(self.root/SOURCE.name),url,'/owned/firefox','/owned/policies.json'],
            env=self.env|env,capture_output=True,text=True,timeout=5)
        calls=[json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []
        return result,calls

    def test_strict_page_navigation_and_owned_browser_join(self):
        result,calls=self.run_probe()
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual([call[0] for call in calls],['launch','context','goto','close'])
        self.assertFalse(calls[1][1]['ignoreHTTPSErrors'])
        self.assertEqual(calls[2][1][0],'https://localhost:1234/healthz')
        self.assertEqual(json.loads(result.stdout),{'activation':'strict_browser_https','category':'passed','status':200,'cleanup':'closed'})

    def test_certificate_navigation_and_http_failures_are_closed_and_joined(self):
        for env,category in [({'FAILURE':'SEC_ERROR_UNKNOWN_ISSUER'},'certificate'),
                ({'FAILURE':'NS_ERROR_NET_RESET'},'network_reset'),({'STATUS':'503'},'http_status'),
                ({'RESPONSE_URL':'https://foreign.invalid/healthz'},'redirect')]:
            with self.subTest(category=category):
                self.log.unlink(missing_ok=True); result,calls=self.run_probe(**env)
                self.assertEqual(result.returncode,1); self.assertEqual(calls[-1][0],'close')
                receipt=json.loads(result.stdout); self.assertEqual(receipt['category'],category)
                if 'FAILURE' in env: self.assertEqual(receipt['errorCode'],env['FAILURE'])
                self.assertEqual(receipt['cleanup'],'closed'); self.assertNotIn('secret',result.stdout+result.stderr)

    def test_bad_inputs_and_foreign_launch_environment_have_no_browser_effect(self):
        for url,env in [('http://localhost:1234',{}),('https://outside.invalid:1234',{}),
                ('https://localhost:1234?unknown',{}),('https://localhost:1234/',{}),
                ('https://localhost:1234',{'EXECUTABLE':'/foreign/firefox'}),
                ('https://localhost:1234',{'PLAYWRIGHT_FIREFOX_POLICIES_JSON':'/foreign/policy'})]:
            with self.subTest(url=url,env=env):
                result,calls=self.run_probe(url,**env); self.assertEqual(result.returncode,1)
                self.assertEqual(calls,[])

    def test_cleanup_failure_cannot_be_reported_as_success(self):
        result,calls=self.run_probe(CLOSE_FAIL='1')
        self.assertEqual(result.returncode,1); self.assertEqual(json.loads(result.stdout)['cleanup'],'failed')
        self.assertEqual(calls[-1][0],'close')

    def test_missing_and_oversized_arguments_reject_before_url_parsing(self):
        # Replace the global URL constructor solely to witness whether it was called.
        bootstrap=self.root/'url-witness.cjs'
        bootstrap.write_text("global.URL=class {constructor(){require('fs').writeFileSync(process.env.CALLS,'URL parsed');throw Error('URL parsed');}}")
        for args in ([],['x'*2049,'/owned/firefox','/owned/policies.json'],
                ['https://localhost:1234','x'*4097,'/owned/policies.json']):
            with self.subTest(lengths=[len(value) for value in args]):
                self.log.unlink(missing_ok=True)
                result=subprocess.run(['node','--require',str(bootstrap),str(self.root/SOURCE.name),*args],
                    env=self.env,capture_output=True,text=True,timeout=5)
                self.assertEqual(result.returncode,1);self.assertFalse(self.log.exists())
