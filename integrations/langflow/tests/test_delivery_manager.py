import importlib.util
from pathlib import Path
import tempfile
import json
import types
import unittest
from unittest.mock import patch

P=Path(__file__).resolve().parents[1]/'delivery/delivery_manager.py'
s=importlib.util.spec_from_file_location('manager',P);m=importlib.util.module_from_spec(s);s.loader.exec_module(m)


class Client:
    def __init__(self):self.calls=[];self.fail_pull=False;self.created_token=None
    def token(self,name):return self.created_token
    def identity(self,ref,optional=False):
        if ref=='bootstrap':return {'id':'sha256:'+'b'*64,'os':'linux','architecture':'amd64','digests':[]}
        return {'id':'sha256:'+'a'*64,'os':'linux','architecture':'amd64','digests':[ref]}
    def run(self,args,timeout=15):
        self.calls.append(args)
        if args[0]=='pull' and self.fail_pull:raise m.ManagerError('synthetic-pull-failed')
        if args[0]=='run':return 'owned-new-registry-id'
        return ''


class ManagerTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name)
        self.meta={'id':'offline-test','releases':['v1'],'bootstrap':{'tag':'bootstrap','config_id':'sha256:'+'b'*64},'images':{s:{'reference':'localhost:15000/teknowra/'+s+'@sha256:'+'c'*64,'config_id':'sha256:'+'a'*64,'platform':'linux/amd64'} for s in ('app','langflow','postgres','redis')}}

    def test_import_pulls_every_pinned_reference_and_removes_only_its_registry(self):
        c=Client()
        with patch.object(m.distribution,'verify_installed',return_value=self.meta),patch.object(m,'ensure_port_free'),patch.object(m,'registry_ready'):
            result=m.import_images(self.root,'docker','isolated-import',client=c)
        self.assertEqual(result['images_verified'],4)
        pulls=[a[1] for a in c.calls if a[0]=='pull']
        self.assertEqual(set(pulls),{v['reference'] for v in self.meta['images'].values()})
        self.assertEqual(c.calls[-1],['rm','-f','owned-new-registry-id'])

    def test_import_failure_cleans_only_its_new_container(self):
        c=Client();c.fail_pull=True
        with patch.object(m.distribution,'verify_installed',return_value=self.meta),patch.object(m,'ensure_port_free'),patch.object(m,'registry_ready'):
            with self.assertRaises(m.ManagerError):m.import_images(self.root,'docker','isolated-import',client=c)
        self.assertEqual(c.calls[-1],['rm','-f','owned-new-registry-id'])

    def test_wrong_bootstrap_tag_is_rejected_without_loading_or_starting(self):
        c=Client();self.meta['bootstrap']['config_id']='sha256:'+'f'*64
        with patch.object(m.distribution,'verify_installed',return_value=self.meta),patch.object(m,'ensure_port_free'):
            with self.assertRaises(m.ManagerError):m.import_images(self.root,'docker','isolated-import',client=c)
        self.assertEqual(c.calls,[])

    def test_partial_image_import_does_not_report_success(self):
        c=Client();original=c.identity
        def bad(ref,optional=False):
            value=original(ref,optional)
            if '/redis@' in ref:value['id']='sha256:'+'f'*64
            return value
        c.identity=bad
        with patch.object(m.distribution,'verify_installed',return_value=self.meta),patch.object(m,'ensure_port_free'),patch.object(m,'registry_ready'):
            with self.assertRaises(m.ManagerError):m.import_images(self.root,'docker','isolated-import',client=c)

    def test_switch_rejects_wrong_cached_image_before_stopping_services(self):
        c=Client();original=c.identity
        def bad(ref,optional=False):
            v=original(ref,optional);v['id']='sha256:'+'f'*64;return v
        c.identity=bad
        with patch.object(m.distribution,'verify_installed',return_value=self.meta):
            with self.assertRaises(m.ManagerError):
                m.switch(self.root,self.root/'components','v1',self.root/'locations.json',self.root/'overlay.json','isolated','docker',self.root/'generated.json',client=c)
        self.assertEqual(c.calls,[])
        self.assertFalse((self.root/'components/current.json').exists())

    def test_activation_timeout_after_pointer_write_restores_previous_state(self):
        c=Client();root=self.root/'components';root.mkdir()
        previous=b'{"release_id":"old"}';(root/'current.json').write_bytes(previous)
        config=self.root/'compose.json';config.write_bytes(b'previous-generated-config')
        locations=self.root/'locations.json';locations.write_text('{}')
        overlay=self.root/'overlay.json';overlay.write_text(json.dumps({'services':{s:{'image':self.meta['images'][s]['reference']} for s in ('postgres','redis')}}))
        staged=[]
        def timeout(args,timeout=15):
            if args[0]=='run':
                mount=next(a for a in args if a.startswith('type=bind,source=') and ',target=/install' in a)
                path=Path(mount.split('source=',1)[1].split(',target=',1)[0]);staged.append(path)
                (path/'current.json').write_text('{"release_id":"v1"}')
                raise m.ManagerError('activation-response-lost')
            return ''
        c.run=timeout;recovery=m.distribution.load('recovery')
        fake=types.SimpleNamespace(install=lambda *_:None)
        with patch.object(m.distribution,'verify_installed',return_value=self.meta),patch.object(m.distribution,'load',side_effect=lambda n:fake if n=='release' else recovery):
            with self.assertRaises(m.ManagerError):m.switch(self.root/'distribution',root,'v1',locations,overlay,'isolated','docker',config,client=c)
        self.assertEqual((root/'current.json').read_bytes(),previous)
        self.assertEqual(config.read_bytes(),b'previous-generated-config')
        self.assertTrue(staged)
        # The daemon can outlive the CLI timeout: a late write must still be
        # confined to staging, never the real host-owned activation pointer.
        (staged[0]/'current.json').write_text('{"release_id":"late"}')
        self.assertEqual((root/'current.json').read_bytes(),previous)

    def test_lost_registry_start_response_cleans_by_unique_owned_name(self):
        c=Client();original=c.run
        def lost(args,timeout=15):
            if args[0]=='run':
                c.calls.append(args)
                c.created_token=next(v.split('=',1)[1] for v in args if v.startswith('teknowra.operation=')) if any(v.startswith('teknowra.operation=') for v in args) else 'missing-token'
                raise m.ManagerError('response-lost')
            return original(args,timeout)
        c.run=lost
        with patch.object(m.distribution,'verify_installed',return_value=self.meta),patch.object(m,'ensure_port_free'):
            with self.assertRaises(m.ManagerError):m.import_images(self.root,'docker','isolated-import',client=c)
        self.assertTrue(any(a[0:2]==['rm','-f'] for a in c.calls))


if __name__=='__main__':unittest.main()
