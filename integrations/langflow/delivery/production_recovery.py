"""Reuse tested namespace recovery for the existing production Compose project.

No bundle or second release store is needed. Only trusted host configuration
can select Compose files; no values or Docker diagnostics enter event output.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import signal
import sys
import threading

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('teknowra_group_recovery', Path(__file__).with_name('recovery.py'))
recovery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recovery)
RecoveryError = recovery.RecoveryError


def configuration(path):
    path = Path(path).resolve()
    try:
        data = json.loads(path.read_text(encoding='utf-8-sig'))
        images = data['images']
        kind = data.get('kind', 'teknowra-langflow')
        profiles = {
            'teknowra-langflow': ('teknowra', 'liugl2025/teknowra-app:latest',
                                 r'(?:ghcr\.io/qbsoft|liugl2025)/teknowra-langflow@sha256:[0-9a-f]{64}'),
            'crm-wren': ('crm', 'ghcr.io/qbsoft/crm-backend:latest',
                         r'ghcr\.io/qbsoft/crm-wren@sha256:[0-9a-f]{64}')}
        project, app, sidecar = profiles[kind]
        if (data['project'] != project or set(images) != {'app', 'langflow'}
                or images['app'] != app or not re.fullmatch(sidecar, images['langflow'])):
            raise ValueError()
        compose = data['compose_files']
        guards = data['guard_files']
        if not isinstance(compose, list) or not compose or not isinstance(guards, list) or not guards:
            raise ValueError()
        files = {path, *(Path(p) for p in compose), *(Path(p) for p in guards)}
        if len(files) > 2048 or any(not p.is_absolute() or p.is_symlink() or not p.is_file() for p in files):
            raise ValueError()
        fingerprints = {p: hashlib.sha256(p.read_bytes()).digest() for p in files}
    except (KeyError, TypeError, ValueError, OSError):
        raise RecoveryError('invalid-production-recovery-config') from None

    def guard():
        try:
            if any(p.is_symlink() or hashlib.sha256(p.read_bytes()).digest() != expected
                   for p, expected in fingerprints.items()):
                raise RecoveryError('deployment-changed-restart-monitor')
        except OSError:
            raise RecoveryError('deployment-changed-restart-monitor') from None
        return tuple(fingerprints.values())
    return data, guard


class CRMWrenDocker(recovery.Docker):
    """Translate fixed CRM service names only after verifying their real labels."""
    services = {'app': 'crm-backend', 'langflow': 'crm-wren'}

    def inspect(self, service):
        actual = self.services[service]
        item = super().inspect(actual)
        if item is None:
            return None
        if item['project'] != 'crm' or item['service'] != actual:
            raise RecoveryError('container-identity-mismatch')
        return {**item, 'service': service}

    def healthy(self, item):
        code = ('import os,urllib.request; '
                'r=urllib.request.Request("http://127.0.0.1:8099/v1/catalog", '
                'headers={"X-Ask-Wren-Token":os.environ["ASK_WREN_TOKEN"]}); '
                'print(urllib.request.urlopen(r,timeout=3).status)')
        try:
            return self.run(['exec', item['id'], 'python', '-c', code], timeout=8) == '200'
        except RecoveryError:
            return False

    def recreate(self):
        self.run([*self.compose, 'up', '-d', '--no-deps', '--no-build', '--pull', 'never',
                  '--force-recreate', 'crm-wren'], timeout=120)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', required=True)
    parser.add_argument('--docker', default='docker')
    parser.add_argument('--once', action='store_true')
    args = parser.parse_args()
    stopped = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stopped.set())
    project = 'unconfigured'
    def emit(event):
        print(json.dumps({'event': event, 'project': project}), flush=True)
    try:
        with recovery.exclusive(Path(args.config).resolve().parent / '.production-recovery.lock'):
            data, guard = configuration(args.config)
            project = data['project']
            adapter = CRMWrenDocker if data.get('kind') == 'crm-wren' else recovery.Docker
            controller = recovery.Reconciler(
                adapter(args.docker, data['project'], data['compose_files']),
                data['project'], data['images'], guard,
                sleep=stopped.wait, cancelled=stopped.is_set)
            previous = None
            while not stopped.is_set():
                try:
                    status = controller.tick()
                except RecoveryError as error:
                    status = str(error)
                    if status == 'deployment-changed-restart-monitor':
                        raise
                if status != previous:
                    emit(status)
                    previous = status
                if args.once:
                    return 0 if status in ('ready', 'recovered') else 1
                stopped.wait(5)
    except (RecoveryError, ValueError, OSError):
        emit('recovery-configuration-or-lock-error')
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
