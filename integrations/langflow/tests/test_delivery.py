import importlib.util
import hashlib
import json
import tempfile
import unittest
import zipfile
from pathlib import Path

MODULE = Path(__file__).resolve().parents[1] / 'delivery' / 'release.py'

class DeliveryTests(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location('delivery_release', MODULE)
        self.m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.m)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'source'
        for name in self.m.SOURCE_FILES:
            path = self.source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('# synthetic source\n', encoding='utf-8')
        (self.source / '.env').write_text('MUST_NOT_LEAK=synthetic-secret')
        self.lock = {'format':1, 'python':'3.12.11', 'platform':'win32', 'packages':{'langflow-base':'1.12.5'}, 'sources':{}}
        self.target = self.root / 'installed'

    def build(self, version='v1'):
        output = self.root / (version+'.zip')
        self.m.build(self.source, self.lock, version, output)
        return output

    def test_deterministic_allowlist_never_copies_local_secrets(self):
        first = self.build()
        other = self.root / 'repeat.zip'
        self.m.build(self.source,self.lock,'v1',other)
        self.assertEqual(first.read_bytes(),other.read_bytes())
        with zipfile.ZipFile(first) as z:
            self.assertFalse(any('.env' in name for name in z.namelist()))
            self.assertNotIn(b'synthetic-secret', b''.join(z.read(name) for name in z.namelist()))

    def test_install_switch_and_rollback_whole_bundle(self):
        self.m.install(self.build(),self.target)
        self.m.activate(self.target,'v1',self.lock)
        (self.source/'teknowra_support.py').write_text('# actual v2 component change')
        self.m.install(self.build('v2'),self.target)
        self.m.activate(self.target,'v2',self.lock)
        self.assertEqual(self.m.active(self.target)['release_id'],'v2')
        self.m.activate(self.target,'v1',self.lock)
        self.assertEqual(self.m.active(self.target)['release_id'],'v1')
        self.assertEqual((self.target/'releases/v1/payload/teknowra_support.py').read_text(),'# synthetic source\n')

    def test_bad_environment_and_tampered_release_do_not_change_pointer(self):
        self.m.install(self.build(),self.target)
        self.m.activate(self.target,'v1',self.lock)
        self.m.install(self.build('v2'),self.target)
        incompatible={**self.lock,'packages':{'langflow-base':'999'}}
        with self.assertRaises(ValueError):self.m.activate(self.target,'v2',incompatible)
        (self.target/'releases/v2/payload/teknowra_support.py').write_text('tampered')
        with self.assertRaises(ValueError):self.m.activate(self.target,'v2',self.lock)
        self.assertEqual(self.m.active(self.target)['release_id'],'v1')

    def test_zip_slip_unknown_member_and_tampering_rejected_before_install(self):
        for name,data in [('../escape','x'),('payload/.env','x'),('payload/teknowra_support.py','modified')]:
            original=self.build()
            bad=self.root/'bad.zip'
            with zipfile.ZipFile(original) as src, zipfile.ZipFile(bad,'w') as dst:
                for entry in src.infolist():
                    dst.writestr(entry, data if entry.filename==name else src.read(entry))
                if name not in src.namelist():dst.writestr(name,data)
            with self.assertRaises(ValueError):self.m.install(bad,self.target)
            self.assertFalse((self.target/'releases/v1').exists())
            self.assertFalse((self.root/'escape').exists())

    def test_existing_release_cannot_be_overwritten_or_escape_install_root(self):
        self.m.install(self.build(),self.target)
        (self.source/'teknowra_support.py').write_text('different')
        with self.assertRaises(ValueError):self.m.install(self.build(),self.target)
        for version in ('../outside','', 'a/b'):
            with self.assertRaises(ValueError):self.m.build(self.source,self.lock,version,self.root/'bad.zip')

    def test_source_symlink_rejected(self):
        victim=self.source/'teknowra_support.py'
        victim.unlink()
        try:victim.symlink_to(self.source/'.env')
        except OSError:self.skipTest('symlink privilege unavailable')
        with self.assertRaises(ValueError):self.build()

    def test_unknown_installed_code_is_not_silently_accepted(self):
        self.m.install(self.build(),self.target)
        (self.target/'releases/v1/payload/injected.py').write_text('unapproved code')
        with self.assertRaises(ValueError):self.m.activate(self.target,'v1',self.lock)
        self.assertFalse((self.target/'current.json').exists())

    def test_installed_directory_symlink_cannot_escape_root(self):
        self.m.install(self.build(),self.target)
        outside=self.root/'outside';outside.mkdir()
        release=self.target/'releases/v1'
        release.rename(outside/'v1')
        try:release.symlink_to(outside/'v1',target_is_directory=True)
        except OSError:self.skipTest('symlink privilege unavailable')
        with self.assertRaises(ValueError):self.m.activate(self.target,'v1',self.lock)

    def test_container_kind_requires_pinned_images(self):
        with self.assertRaises(ValueError):
            self.m.build(self.source,self.lock,'container',self.root/'container.zip',images={'app':'latest','langflow':'latest'})

    def test_recovery_files_ship_together_and_prior_bundles_still_verify(self):
        recovery={'delivery/recovery.py', 'delivery/teknowra-langflow-recovery.service'}
        self.assertTrue(recovery.issubset(self.m.SOURCE_FILES))
        archive=self.build()
        with zipfile.ZipFile(archive) as source:
            values={n:source.read(n) for n in source.namelist()}
        legacy=json.loads(values['manifest.json'])
        for name in recovery:legacy['files'].pop('payload/'+name)
        self.m.check_manifest(legacy)
        partial=json.loads(json.dumps(legacy))
        partial['files']['payload/delivery/recovery.py']='1'*64
        with self.assertRaises(ValueError):self.m.check_manifest(partial)

    def test_container_config_uses_only_active_manifest_images_and_payload(self):
        self.lock['platform']='linux'
        profile=self.root/'profile';profile.mkdir()
        metadata=profile/'langflow_base-1.12.5.dist-info/METADATA';metadata.parent.mkdir()
        metadata.write_text('Name: langflow-base\nVersion: 1.12.5\n')
        self.lock['sources']={}
        for name in self.m.CRITICAL_SOURCES:
            file=profile/name;file.parent.mkdir(parents=True,exist_ok=True);file.write_text('pinned runtime source')
            self.lock['sources'][name]=hashlib.sha256(file.read_bytes()).hexdigest()
        (self.source/'delivery/compose.yaml').write_text((MODULE.parent/'compose.yaml').read_text())
        images={'app':'example.invalid/app@sha256:'+'1'*64,'langflow':'example.invalid/lf@sha256:'+'2'*64}
        bundle=self.root/'container.zip';self.m.build(self.source,self.lock,'container',bundle,images=images)
        self.m.install(bundle,self.target);self.m.activate(self.target,'container',self.lock)
        locations={key:str(self.root/key.lower()) for key in ('APP_ENV_FILE','LANGFLOW_ENV_FILE','BINDINGS_DIR','SERVICE_CREDENTIALS_DIR','APP_RELEASE_STATE_DIR','LANGFLOW_STATE_DIR')}
        config=self.m.container_configuration(self.target,profile,locations)
        self.assertEqual(config['services']['app']['image'],images['app'])
        self.assertEqual(config['services']['langflow']['image'],images['langflow'])
        self.assertEqual(config['services']['langflow']['network_mode'],'service:app')
        self.assertIn(str(self.target/'releases/container/payload'),config['services']['app']['volumes'][0])
        with self.assertRaises(ValueError):self.m.container_configuration(self.target,profile,{**locations,'LANGFLOW_ENV_FILE':locations['APP_ENV_FILE']})
        (profile/self.m.CRITICAL_SOURCES[0]).write_text('wrong profile')
        with self.assertRaises(ValueError):self.m.container_configuration(self.target,profile,locations)

if __name__=='__main__':unittest.main()
