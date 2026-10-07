"""Exact public-CA trust owned by one disposable macOS Actions proof job."""
import hashlib
import os
import ssl
import subprocess
import sys
from positive_reentry_processes import run_owned_command


def require_hosted_macos():
    if (sys.platform != 'darwin' or os.environ.get('CI') != 'true'
            or os.environ.get('GITHUB_ACTIONS') != 'true' or os.environ.get('RUNNER_OS') != 'macOS'):
        raise RuntimeError('TLS trust requires this disposable macOS Actions job; no local trust mutation')


class HostedFixtureTrust:
    keychain = '/Library/Keychains/System.keychain'

    def __init__(self, run, receipt):
        self.run, self.receipt = run, receipt
        self.certificate = run / 'fixture-public-ca.pem'
        self.fingerprint = None
        self.attempted = False

    def present(self):
        result = subprocess.run(['/usr/bin/security', 'find-certificate', '-a', '-Z', self.keychain],
                                capture_output=True, timeout=15, check=True)
        return self.fingerprint in result.stdout.decode().upper()

    def prepare(self, binary, environment, source):
        require_hosted_macos()
        candidate = self.run / 'tls-private-export.pem'
        with candidate.open('w') as output:
            run_owned_command([str(binary), 'tls-certificate'], environment, timeout=30, output=output)
        validator = source / 'scripts/ci/browser-fixture-tls.sh'
        with (self.run / 'tls-private.log').open('w') as output:
            run_owned_command(['bash', '-c', 'source "$1"; validate_browser_fixture_ca "$2"',
                               'fixture', str(validator), str(candidate)], environment, timeout=15, output=output)
        candidate.rename(self.certificate)
        der = subprocess.check_output(['openssl', 'x509', '-in', str(self.certificate), '-outform', 'DER'], timeout=15)
        self.fingerprint = hashlib.sha256(der).hexdigest().upper()
        if self.present():
            raise RuntimeError('Exact fixture CA already exists; preserve preexisting trust')
        self.receipt['fixturePublicCASHA256'] = hashlib.sha256(self.certificate.read_bytes()).hexdigest()
        self.receipt['fixturePublicCADER_SHA256'] = self.fingerprint.lower()
        self.receipt['fixtureCAValidatorSHA256'] = hashlib.sha256(validator.read_bytes()).hexdigest()
        self.attempted = True
        with (self.run / 'tls-private.log').open('a') as output:
            run_owned_command(['sudo', '-n', '/usr/bin/security', 'add-trusted-cert', '-d', '-r', 'trustRoot',
                               '-p', 'ssl', '-k', self.keychain, str(self.certificate)], environment, timeout=30, output=output)
        if not self.present():
            raise RuntimeError('Exact fixture CA installation was not confirmed')
        environment['NODE_EXTRA_CA_CERTS'] = str(self.certificate)
        self.receipt['browserCertificateBypasses'] = 'disabled'
        self.receipt['fixtureCAInstalled'] = True
        return ssl.create_default_context(cafile=str(self.certificate))

    def cleanup(self, environment):
        if not self.attempted:
            return True
        require_hosted_macos()
        with (self.run / 'tls-private.log').open('a') as output:
            run_owned_command(['sudo', '-n', '/usr/bin/security', 'remove-trusted-cert', '-d', str(self.certificate)],
                              environment, timeout=30, output=output)
            if self.present():
                run_owned_command(['sudo', '-n', '/usr/bin/security', 'delete-certificate', '-Z', self.fingerprint,
                                   self.keychain], environment, timeout=30, output=output)
        return not self.present()
