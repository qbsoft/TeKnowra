import importlib.util
import tempfile
import json
import os
import shutil
import subprocess
import sys
import types
from unittest.mock import patch
import unittest
from pathlib import Path

MODULE = Path(__file__).resolve().parents[1] / 'delivery/recovery.py'
spec = importlib.util.spec_from_file_location('group_recovery', MODULE)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

IMAGES = {'app': 'local/app@sha256:'+'1'*64, 'langflow': 'local/lf@sha256:'+'2'*64}


class Docker:
    def __init__(self):
        self.items = {s: {'id': s+'-1', 'project': 'isolated', 'service': s,
                          'image': IMAGES[s], 'running': True, 'paused': False,
                          'health': 'healthy', 'started': 'generation-1', 'restarts': 0,
                          'network': 'container:app-1'} for s in IMAGES}
        self.namespaces = {'app': 'net:new', 'langflow': 'net:old'}
        self.repairs = 0
        self.fix = True
        self.on_repair = lambda: None

    def inspect(self, service): return dict(self.items[service])
    def namespace(self, service, item): return self.namespaces[service]
    def healthy(self, item): return True
    def recreate(self):
        self.repairs += 1
        self.items['langflow']['id'] = 'langflow-'+str(self.repairs+1)
        self.items['langflow']['network'] = 'container:'+self.items['app']['id']
        if self.fix: self.namespaces['langflow'] = self.namespaces['app']
        self.on_repair()


class RecoveryTests(unittest.TestCase):
    def setUp(self):
        self.d = Docker(); self.now = 0; self.version = 'v1'
        self.c = m.Reconciler(self.d, 'isolated', IMAGES, lambda: self.version,
                              clock=lambda: self.now, sleep=self.advance, timeout=3)

    def advance(self, seconds): self.now += seconds

    def test_recreates_only_langflow_and_verifies_actual_namespace(self):
        self.assertEqual(self.c.tick(), 'recovered')
        self.assertEqual(self.d.repairs, 1)
        self.assertEqual(self.d.items['app']['id'], 'app-1')
        self.assertEqual(self.c.tick(), 'ready')
        self.assertEqual(self.d.repairs, 1)

    def test_waits_for_backend_health_without_destroying_langflow(self):
        self.d.items['app']['health'] = 'starting'
        self.assertEqual(self.c.tick(), 'waiting-app')
        self.assertEqual(self.d.repairs, 0)

    def test_does_not_undo_intentional_stop_or_pause(self):
        for service in IMAGES:
            for field in ('running', 'paused'):
                with self.subTest(service=service, field=field):
                    old = self.d.items[service][field]
                    self.d.items[service][field] = field == 'paused'
                    self.assertEqual(self.c.tick(), 'inactive')
                    self.assertEqual(self.d.repairs, 0)
                    self.d.items[service][field] = old

    def test_wrong_project_or_image_is_rejected_before_mutation(self):
        for field, value in (('project', 'other-project'), ('image', 'wrong:latest')):
            old = self.d.items['langflow'][field]
            self.d.items['langflow'][field] = value
            with self.assertRaises(m.RecoveryError): self.c.tick()
            self.assertEqual(self.d.repairs, 0)
            self.d.items['langflow'][field] = old

    def test_new_backend_generation_during_repair_is_not_claimed_success(self):
        self.d.on_repair = lambda: self.d.items['app'].update(started='generation-2')
        self.assertEqual(self.c.tick(), 'changed-app')

    def test_release_change_during_repair_is_rejected(self):
        self.d.on_repair = lambda: setattr(self, 'version', 'v2')
        with self.assertRaises(m.RecoveryError): self.c.tick()

    def test_failed_repair_is_bounded_and_backed_off(self):
        self.d.fix = False
        with self.assertRaises(m.RecoveryError): self.c.tick()
        self.assertEqual(self.d.repairs, 1)
        self.assertEqual(self.c.tick(), 'cooldown')
        self.assertEqual(self.d.repairs, 1)

    def test_configuration_alone_cannot_claim_namespace_ready(self):
        self.d.fix = False
        with self.assertRaises(m.RecoveryError): self.c.tick()

    def test_slow_langflow_startup_is_not_recreated_repeatedly(self):
        self.d.namespaces['langflow'] = self.d.namespaces['app']
        self.d.healthy = lambda item: False
        self.assertEqual(self.c.tick(), 'waiting-langflow')
        self.assertEqual(self.d.repairs, 0)

    def test_monitor_can_stop_while_waiting_for_recovery(self):
        self.d.fix = False
        self.c.cancelled = lambda: True
        self.assertEqual(self.c.tick(), 'stopping')
        self.assertEqual(self.d.repairs, 0)

    def test_second_monitor_cannot_acquire_the_same_deployment(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)/'recovery.lock'
            with m.exclusive(path):
                with self.assertRaises(m.RecoveryError):
                    with m.exclusive(path): self.fail('duplicate monitor allowed')

    def test_environment_and_runtime_profile_change_invalidates_guard(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory); compose=root/'compose.json'; locations=root/'locations.json'
            expected={'services':{s:{'image':v} for s,v in IMAGES.items()}}
            compose.write_text(json.dumps(expected));(root/'current.json').write_text('{}')
            paths={name:str(root/name) for name in ('APP_ENV_FILE','LANGFLOW_ENV_FILE')}
            for path in paths.values():Path(path).write_text('SYNTHETIC=initial')
            locations.write_text(json.dumps(paths)); profile={'synthetic':'v1'}
            fake=types.SimpleNamespace(container_configuration=lambda *_:expected,active=lambda *_:None,
                                       profile_fingerprint=lambda *_:dict(profile))
            args=types.SimpleNamespace(root=str(root),profile=str(root),locations=str(locations),compose=str(compose),overlay=[])
            with patch.dict(sys.modules,{'release':fake}):
                _,guard,_=m.deployment(args);guard()
                for name in paths:
                    Path(paths[name]).write_text('SYNTHETIC=changed')
                    with self.assertRaises(m.RecoveryError):guard()
                    Path(paths[name]).write_text('SYNTHETIC=initial')
                profile['synthetic']='v2'
                with self.assertRaises(m.RecoveryError):guard()

    def test_script_does_not_create_unlisted_bytecode_in_writable_payload(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);script=root/'recovery.py'
            shutil.copyfile(MODULE,script)
            (root/'release.py').write_text('def container_configuration(*args): raise ValueError("synthetic")\n')
            env=os.environ.copy();env.pop('PYTHONDONTWRITEBYTECODE',None)
            args=[sys.executable,str(script),'--project','isolated']
            for name in ('root','profile','locations','compose'):args+=['--'+name,str(root)]
            result=subprocess.run(args,env=env,capture_output=True,timeout=15)
            self.assertEqual(result.returncode,1)
            self.assertFalse((root/'__pycache__').exists(),'strict installed manifest would reject generated bytecode')


if __name__ == '__main__': unittest.main()
