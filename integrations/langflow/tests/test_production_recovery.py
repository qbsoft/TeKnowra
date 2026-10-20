import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

MODULE = Path(__file__).resolve().parents[1] / 'delivery/production_recovery.py'
spec = importlib.util.spec_from_file_location('production_recovery', MODULE)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class ProductionRecoveryConfigurationTests(unittest.TestCase):
    def fixture(self, root):
        compose = root / 'compose.json'
        compose.write_text('{}')
        private = root / 'private.env'
        private.write_text('PRIVATE_VALUE=must-not-appear-in-errors')
        config = root / 'watch.json'
        data = {'project': 'teknowra', 'images': {
            'app': 'liugl2025/teknowra-app:latest',
            'langflow': 'ghcr.io/qbsoft/teknowra-langflow@sha256:' + '1' * 64},
            'compose_files': [str(compose)], 'guard_files': [str(private)]}
        config.write_text(json.dumps(data))
        return config, private, data

    def test_guard_detects_config_or_private_env_changes_without_secret_error(self):
        with tempfile.TemporaryDirectory() as directory:
            config, private, _ = self.fixture(Path(directory))
            data, guard = m.configuration(config)
            self.assertEqual(data['project'], 'teknowra')
            self.assertEqual(guard(), guard())
            private.write_text('PRIVATE_VALUE=changed')
            with self.assertRaisesRegex(m.RecoveryError, '^deployment-changed-restart-monitor$'):
                guard()

    def test_mutable_langflow_or_other_project_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            config, _, data = self.fixture(Path(directory))
            data['images']['langflow'] = 'ghcr.io/qbsoft/teknowra-langflow:latest'
            config.write_text(json.dumps(data))
            with self.assertRaises(m.RecoveryError):
                m.configuration(config)

    def test_actual_service_labels_are_checked_before_aliasing(self):
        adapter = m.CRMWrenDocker('docker', 'crm', ['compose.json'])
        item = {'project':'crm', 'service':'crm-backend', 'id':'backend'}
        with patch.object(m.recovery.Docker, 'inspect', return_value=item) as inspect:
            self.assertEqual(adapter.inspect('app')['service'], 'app')
            inspect.assert_called_once_with('crm-backend')
        for key,value in [('project','other'),('service','app')]:
            wrong = {**item, key:value}
            with patch.object(m.recovery.Docker, 'inspect', return_value=wrong):
                with self.assertRaises(m.RecoveryError):
                    adapter.inspect('app')

    def test_recreation_is_limited_to_wren_and_health_uses_existing_container_secret(self):
        adapter = m.CRMWrenDocker('docker', 'crm', ['compose.json'])
        with patch.object(adapter, 'run', return_value='200') as run:
            self.assertTrue(adapter.healthy({'id':'wren'}))
            self.assertIn('os.environ["ASK_WREN_TOKEN"]', run.call_args.args[0][-1])
        with patch.object(adapter, 'run') as run:
            adapter.recreate()
            command = run.call_args.args[0]
            self.assertEqual(command[-1], 'crm-wren')
            self.assertIn('--no-deps', command)

    def test_verified_owner_dockerhub_digest_is_allowed_but_other_registry_is_not(self):
        with tempfile.TemporaryDirectory() as directory:
            config, _, data = self.fixture(Path(directory))
            data['images']['langflow'] = 'liugl2025/teknowra-langflow@sha256:' + '2' * 64
            config.write_text(json.dumps(data))
            m.configuration(config)
            data['images']['langflow'] = 'another-owner/teknowra-langflow@sha256:' + '2' * 64
            config.write_text(json.dumps(data))
            with self.assertRaises(m.RecoveryError):
                m.configuration(config)
            data['project'] = 'other-project'
            config.write_text(json.dumps(data))
            with self.assertRaises(m.RecoveryError):
                m.configuration(config)

    def test_missing_guard_is_rejected_instead_of_monitoring_unprotected_config(self):
        with tempfile.TemporaryDirectory() as directory:
            config, private, _ = self.fixture(Path(directory))
            private.unlink()
            with self.assertRaises(m.RecoveryError):
                m.configuration(config)

    def test_crm_wren_profile_requires_exact_project_and_verified_images(self):
        with tempfile.TemporaryDirectory() as directory:
            config, _, data = self.fixture(Path(directory))
            data.update(kind='crm-wren', project='crm')
            data['images'] = {'app': 'ghcr.io/qbsoft/crm-backend:latest',
                              'langflow': 'ghcr.io/qbsoft/crm-wren@sha256:' + '3' * 64}
            config.write_text(json.dumps(data))
            actual, guard = m.configuration(config)
            self.assertEqual(actual['project'], 'crm')
            self.assertEqual(guard(), guard())
            data['project'] = 'another-project'
            config.write_text(json.dumps(data))
            with self.assertRaises(m.RecoveryError):
                m.configuration(config)
