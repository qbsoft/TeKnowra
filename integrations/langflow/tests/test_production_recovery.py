import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

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
