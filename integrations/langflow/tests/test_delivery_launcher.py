import importlib.util
import base64
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

SOURCE=Path(__file__).resolve().parents[1]/'delivery/runtime_launcher.py'

class LauncherTests(unittest.TestCase):
    def setUp(self):
        spec=importlib.util.spec_from_file_location('runtime_launcher',SOURCE)
        self.m=importlib.util.module_from_spec(spec);spec.loader.exec_module(self.m)

    def test_env_isolated_credentials_external_and_no_auto_login(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);payload=root/'installed/releases/v1/payload';payload.mkdir(parents=True)
            identity={'bootstrap':{'username':'test-bootstrap','password':'synthetic-password'},'secret_key':base64.urlsafe_b64encode(b'x'*32).decode()}
            with patch.dict(os.environ,{'OPENAI_API_KEY':'must-not-inherit','DB_PASSWORD':'must-not-inherit','LANGFLOW_AUTO_LOGIN':'true','LANGFLOW_ENABLE_SIGNUP':'true'}):
                env=self.m.environment(payload,root/'state',identity,root/'temp')
            self.assertNotIn('OPENAI_API_KEY',env);self.assertNotIn('DB_PASSWORD',env)
            self.assertEqual(env['LANGFLOW_AUTO_LOGIN'],'false');self.assertEqual(env['LANGFLOW_ENABLE_SIGNUP'],'false')
            self.assertEqual(env['LANGFLOW_COMPONENTS_PATH'],str(payload/'components'))
            self.assertTrue(env['LANGFLOW_DATABASE_URL'].endswith('/state/langflow.db'))
            self.assertFalse((payload/'langflow.db').exists())

    def test_invalid_identity_or_state_in_immutable_release_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            with self.assertRaises(ValueError):self.m.environment(root,root/'state',{},root/'temp')
            identity={'bootstrap':{'username':'test','password':'test'},'secret_key':'test'}
            with self.assertRaises(ValueError):self.m.environment(root,root/'state',identity,root/'temp')

    def test_invalid_fernet_bootstrap_key_rejected_before_start(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);payload=root/'release/payload'
            invalid={'bootstrap':{'username':'test','password':'test'},'secret_key':'a'*64}
            with self.assertRaises(ValueError):self.m.environment(payload,root/'state',invalid,root/'temp')

    def test_a_different_interpreter_cannot_validate_a_foreign_library(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):self.m.require_interpreter_library(Path(directory)/'foreign-site-packages')

    def test_state_must_not_write_into_any_installed_release(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            with self.assertRaises(ValueError):self.m.require_external_state(root,root/'releases/old-version/state')
            self.m.require_external_state(root,root/'state')

if __name__=='__main__':unittest.main()
