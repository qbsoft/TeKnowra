"""Host-only recovery for one immutable app/Langflow Compose deployment."""
import argparse
import contextlib
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import threading
import time


class RecoveryError(RuntimeError):
    """Safe event code only: Docker output and deployment secrets stay private."""


def generation(item):
    return item['id'], item['started'], item['restarts']


class Reconciler:
    def __init__(self, docker, project, images, guard, *, clock=time.monotonic,
                 sleep=time.sleep, timeout=180, cancelled=lambda: False):
        self.docker, self.project, self.images, self.guard = docker, project, images, guard
        self.clock, self.sleep, self.timeout = clock, sleep, timeout
        self.cancelled = cancelled
        self.retry_at = 0
        self.failures = 0

    def pair(self):
        pair = {service: self.docker.inspect(service) for service in self.images}
        for service, item in pair.items():
            if item and (item['project'] != self.project or item['service'] != service
                         or item['image'] != self.images[service]):
                raise RecoveryError('container-identity-mismatch')
        return pair

    @staticmethod
    def running(pair):
        return all(item and item['running'] and not item['paused'] for item in pair.values())

    def same_network(self, pair):
        return (pair['langflow']['network'] == 'container:'+pair['app']['id']
                and self.docker.namespace('app', pair['app'])
                == self.docker.namespace('langflow', pair['langflow']))

    def tick(self):
        if self.cancelled(): return 'stopping'
        token = self.guard()
        pair = self.pair()
        if not self.running(pair): return 'inactive'
        if pair['app']['health'] != 'healthy': return 'waiting-app'
        app_generation = generation(pair['app'])
        if self.same_network(pair):
            return 'ready' if self.docker.healthy(pair['langflow']) else 'waiting-langflow'
        if self.clock() < self.retry_at: return 'cooldown'
        if self.guard() != token: raise RecoveryError('deployment-changed')
        current = self.pair()
        if not self.running(current): return 'inactive'
        if generation(current['app']) != app_generation: return 'changed-app'
        if current['app']['health'] != 'healthy': return 'waiting-app'
        if self.cancelled(): return 'stopping'
        try:
            self.docker.recreate()
            end = self.clock()+self.timeout
            while self.clock() < end:
                if self.cancelled(): return 'stopping'
                if self.guard() != token: raise RecoveryError('deployment-changed')
                current = self.pair()
                if not self.running(current):
                    # A deliberate stop during recovery must remain stopped.
                    return 'inactive'
                if generation(current['app']) != app_generation: return 'changed-app'
                if (current['app']['health'] == 'healthy' and self.same_network(current)
                        and self.docker.healthy(current['langflow'])):
                    final = self.pair()
                    if (not self.running(final) or generation(final['app']) != app_generation
                            or final['langflow']['id'] != current['langflow']['id']):
                        return 'changed-app'
                    self.failures = 0
                    self.retry_at = 0
                    return 'recovered'
                self.sleep(2)
            raise RecoveryError('recovery-timeout')
        except RecoveryError:
            self.failures += 1
            self.retry_at = self.clock()+min(300, 30*2**min(self.failures-1, 4))
            raise


class Docker:
    def __init__(self, executable, project, files):
        self.executable, self.project = executable, project
        self.compose = ['compose', '-p', project]
        for path in files: self.compose += ['-f', str(path)]

    def run(self, args, timeout=15):
        try:
            result = subprocess.run([self.executable, *args], capture_output=True,
                                    text=True, encoding='utf-8', timeout=timeout)
        except (OSError, subprocess.TimeoutExpired):
            raise RecoveryError('docker-unavailable-or-timeout') from None
        if result.returncode: raise RecoveryError('docker-command-failed')
        return result.stdout.strip()

    def inspect(self, service):
        ids = self.run([*self.compose, 'ps', '-a', '-q', service]).split()
        if not ids: return None
        if len(ids) != 1: raise RecoveryError('multiple-service-instances')
        template = ('{"id":{{json .Id}},"project":{{json (index .Config.Labels "com.docker.compose.project")}},'
                    '"service":{{json (index .Config.Labels "com.docker.compose.service")}},'
                    '"image":{{json .Config.Image}},"running":{{json .State.Running}},'
                    '"paused":{{json .State.Paused}},"started":{{json .State.StartedAt}},'
                    '"restarts":{{json .RestartCount}},"network":{{json .HostConfig.NetworkMode}},'
                    '"health":{{with (index .State "Health")}}{{json (index . "Status")}}{{else}}null{{end}}}')
        return json.loads(self.run(['inspect', ids[0], '--format', template]))

    def namespace(self, service, item):
        command = (['readlink', '/proc/self/ns/net'] if service == 'app' else
                   ['python', '-c', 'import os; print(os.readlink("/proc/self/ns/net"))'])
        value = self.run(['exec', item['id'], *command])
        if not re.fullmatch(r'net:\[\d+\]', value): raise RecoveryError('invalid-network-namespace')
        return value

    def healthy(self, item):
        try:
            return self.run(['exec', item['id'], 'python', '-c',
                             'import urllib.request; print(urllib.request.urlopen("http://127.0.0.1:7861/health_check",timeout=3).status)'], timeout=8) == '200'
        except RecoveryError: return False

    def recreate(self):
        self.run([*self.compose, 'up', '-d', '--no-deps', '--no-build', '--pull', 'never',
                  '--force-recreate', 'langflow'], timeout=120)


@contextlib.contextmanager
def exclusive(path):
    path = Path(path)
    if path.is_symlink(): raise RecoveryError('unsafe-lock-path')
    handle = path.open('a+b')
    try:
        if handle.tell() == 0: handle.write(b'0'); handle.flush()
        handle.seek(0)
        try:
            if os.name == 'nt':
                import msvcrt
                msvcrt.locking(handle.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError: raise RecoveryError('recovery-already-running') from None
        yield
    finally:
        handle.close()


def deployment(args):
    # Executed from a verified installed payload, never supplied by model/tool input.
    # Installed payloads have an exact manifest: even writable deployments must
    # not acquire an unlisted release.__pycache__ merely by starting recovery.
    sys.dont_write_bytecode = True
    import release
    root, compose = Path(args.root).resolve(), Path(args.compose).resolve()
    profile, locations = Path(args.profile).resolve(), Path(args.locations).resolve()
    overlays = [Path(p).resolve() for p in args.overlay]
    external = json.loads(locations.read_text(encoding='utf-8'))
    expected = release.container_configuration(root, profile, external)
    expected_profile = release.profile_fingerprint(profile)
    if json.loads(compose.read_text(encoding='utf-8')) != expected:
        raise RecoveryError('generated-compose-mismatch')
    # Overlays may add infrastructure dependencies/files, never replace the LF
    # image, network, command, component mounts or identity configuration.
    for path in overlays:
        extra = json.loads(path.read_text(encoding='utf-8')).get('services', {})
        if extra.get('langflow'): raise RecoveryError('langflow-overlay-forbidden')
        app = extra.get('app', {})
        if set(app)-{'volumes', 'depends_on'}: raise RecoveryError('app-overlay-forbidden')
        for volume in app.get('volumes', []):
            match = re.search(r':(/[^:]+)(?::(?:ro|rw))?$', volume) if isinstance(volume, str) else None
            if not match or match[1].startswith(('/opt/teknowra', '/opt/langflow-profile', '/run/', '/app', '/var/run/')) or match[1] == '/':
                raise RecoveryError('protected-mount-override')
    files = [root/'current.json', compose, locations, *overlays,
             Path(external['APP_ENV_FILE']), Path(external['LANGFLOW_ENV_FILE'])]
    hashes = {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for p in files}
    def guard():
        release.active(root)
        if any(hashlib.sha256(Path(p).read_bytes()).hexdigest() != digest for p, digest in hashes.items()):
            raise RecoveryError('deployment-changed-restart-monitor')
        if release.profile_fingerprint(profile) != expected_profile:
            raise RecoveryError('runtime-profile-changed-restart-monitor')
        return tuple(hashes.values())
    return {s: expected['services'][s]['image'] for s in ('app', 'langflow')}, guard, [compose, *overlays]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('root', 'profile', 'locations', 'compose', 'project'): parser.add_argument('--'+name, required=True)
    parser.add_argument('--docker', default='docker')
    parser.add_argument('--overlay', action='append', default=[])
    parser.add_argument('--once', action='store_true')
    parser.add_argument('--interval', type=float, default=5)
    args = parser.parse_args()
    if not re.fullmatch(r'[a-z0-9][a-z0-9_-]+', args.project) or args.interval < 1:
        parser.error('invalid project or interval')
    stopped = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM): signal.signal(sig, lambda *_: stopped.set())
    def emit(status): print(json.dumps({'event': status, 'project': args.project}), flush=True)
    try:
        with exclusive(Path(args.root)/'.recovery.lock'):
            images, guard, files = deployment(args)
            controller = Reconciler(Docker(args.docker, args.project, files), args.project, images, guard,
                                    sleep=lambda seconds: stopped.wait(seconds), cancelled=stopped.is_set)
            previous = None
            while not stopped.is_set():
                try: status = controller.tick()
                except RecoveryError as error:
                    status = str(error)
                    if args.once: emit(status); return 1
                if status != previous: emit(status); previous = status
                if args.once: return 0
                stopped.wait(args.interval)
    except (RecoveryError, ValueError, OSError):
        emit('recovery-configuration-or-lock-error'); return 1
    return 0


if __name__ == '__main__': raise SystemExit(main())
