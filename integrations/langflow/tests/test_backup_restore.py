import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

MODULE=Path(__file__).resolve().parents[1]/'delivery/backup_restore.py'

class BackupRestoreTests(unittest.TestCase):
    def setUp(self):
        spec=importlib.util.spec_from_file_location('portable_backup',MODULE)
        self.m=importlib.util.module_from_spec(spec);spec.loader.exec_module(self.m)
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name)

    def test_state_roundtrip_and_refuse_overwrite(self):
        source=self.root/'state';source.mkdir();(source/'releases.json').write_text('{"v":1}')
        archive=self.root/'state.zip'
        self.m.pack_state({'platform':source},archive)
        out=self.root/'restored';self.m.restore_state(archive,out)
        self.assertEqual((out/'platform/releases.json').read_text(),'{"v":1}')
        with self.assertRaises(self.m.BackupError):self.m.restore_state(archive,out)

    def test_tampered_archive_refused_before_destination_created(self):
        z=self.root/'bad.zip'
        with zipfile.ZipFile(z,'w') as f:
            f.writestr('state.json',json.dumps({'files':{'platform/data':{'sha256':'0'*64,'bytes':4}},'directories':['platform']}))
            f.writestr('platform/data',b'evil')
        out=self.root/'restore'
        with self.assertRaises(self.m.BackupError):self.m.restore_state(z,out)
        self.assertFalse(out.exists())

    def test_path_traversal_rejected(self):
        z=self.root/'bad.zip'
        with zipfile.ZipFile(z,'w') as f:
            f.writestr('state.json',json.dumps({'files':{'../escaped':{'sha256':'0'*64,'bytes':1}},'directories':[]}))
            f.writestr('../escaped',b'x')
        with self.assertRaises(self.m.BackupError):self.m.restore_state(z,self.root/'out')
        self.assertFalse((self.root/'escaped').exists())

    def test_duplicate_zip_members_rejected(self):
        import warnings
        z=self.root/'duplicate.zip'
        with warnings.catch_warnings():
            warnings.simplefilter('ignore')
            with zipfile.ZipFile(z,'w') as f:
                f.writestr('state.json','{}');f.writestr('state.json','{}')
        with self.assertRaises(self.m.BackupError):self.m.restore_state(z,self.root/'out')

    def test_symlink_source_rejected(self):
        source=self.root/'state';source.mkdir();(source/'file').write_text('source')
        try:(source/'link').symlink_to(source/'file')
        except OSError:self.skipTest('symlink privilege unavailable')
        with self.assertRaises(self.m.BackupError):self.m.pack_state({'state':source},self.root/'state.zip')

    def test_scope_requires_project_label_and_stopped_writer(self):
        stopped={'Config':{'Labels':{'com.docker.compose.project':'owned','com.docker.compose.service':'app'}},'State':{'Running':False,'Paused':False}}
        self.m.check_container(stopped,'owned','app',running=False)
        with self.assertRaises(self.m.BackupError):self.m.check_container(stopped,'other','app',running=False)
        stopped['State']['Running']=True
        with self.assertRaises(self.m.BackupError):self.m.check_container(stopped,'owned','app',running=False)

    def test_existing_database_rejected_before_restore(self):
        class Fake:
            cfg={'db_user':'test'}
            calls=[]
            def run(self,*args,**kw):self.calls.append(args);return b'1\n'
        d=Fake()
        with self.assertRaises(self.m.BackupError):self.m.restore_database(d,'pg','test_restore',self.root/'dump',schema=114)
        self.assertFalse(any('createdb' in c for c in d.calls))

    def test_unsafe_state_label_rejected(self):
        with self.assertRaises(self.m.BackupError):self.m.pack_state({'../escape':self.root},self.root/'state.zip')

    def test_declared_schema122_is_accepted_but_unknown_versions_are_not(self):
        path=self.root/'config.json'
        (self.root/'state').mkdir()
        cfg={'project':'owned','writers':['app','langflow'],'database':'delivery','db_user':'delivery','state_roots':{'state':'state'},'schema':122}
        path.write_text(json.dumps(cfg))
        self.assertEqual(self.m.config(path)['schema'],122)
        for unsupported in (115,121,123,True):
            cfg['schema']=unsupported;path.write_text(json.dumps(cfg))
            with self.assertRaises(self.m.BackupError):self.m.config(path)

    def test_database_must_match_declared_schema_and_be_clean(self):
        class Fake:
            cfg={'schema':122,'db_user':'test'}
            version=b'122|f'
            def run(self,*args,**kw):return self.version if 'schema_migrations' in args[-1] else b'0'
        d=Fake()
        self.assertEqual(set(self.m.db_facts(d,'pg','delivery')),set(self.m.TABLES))
        for bad in (b'114|f',b'122|t',b'123|f'):
            d.version=bad
            with self.assertRaises(self.m.BackupError):self.m.db_facts(d,'pg','delivery')

    def test_new_database_restore_checks_requested_schema122(self):
        class Fake:
            cfg={'db_user':'test','schema':122}
            def __init__(self):self.calls=[]
            def run(self,*args,**kw):
                self.calls.append(args)
                if 'pg_database' in args[-1]:return b''
                if 'schema_migrations' in args[-1]:return b'122|f'
                return b'0'
        d=Fake()
        result=self.m.restore_database(d,'pg','new_restore',self.root/'dump',schema=122)
        self.assertEqual(set(result),set(self.m.TABLES))
        self.assertEqual(sum('createdb' in x for x in d.calls),1)
        self.assertEqual(sum('pg_restore' in x for x in d.calls),1)

    def test_restore_config_backup_schema_mismatch_refused_before_docker(self):
        import contextlib,io
        from unittest.mock import patch
        argv=['backup_restore.py','restore-db','--backup',str(self.root),'--config',str(self.root/'cfg.json'),'--database','new_restore']
        with patch('sys.argv',argv),patch.object(self.m,'verify_backup',return_value={'schema':114}),patch.object(self.m,'config',return_value={'schema':122}),patch.object(self.m,'Docker') as docker,contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(self.m.main(),1)
            docker.assert_not_called()

if __name__=='__main__':unittest.main()
