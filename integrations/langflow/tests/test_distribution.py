import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile

D=Path(__file__).resolve().parents[1]/'delivery'
def module(name):
    spec=importlib.util.spec_from_file_location(name,D/(name+'.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m
m=module('distribution');release=module('release')


class DistributionTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.r=Path(self.temp.name)
        self.registry=self.r/'registry';self.profile=self.r/'profile';self.profile.mkdir()
        metadata=self.profile/'langflow_base-1.dist-info/METADATA';metadata.parent.mkdir();metadata.write_text('Name: langflow-base\nVersion: 1\n')
        for n in release.CRITICAL_SOURCES:
            p=self.profile/n;p.parent.mkdir(parents=True,exist_ok=True);p.write_text('# synthetic\n')
        (self.profile/'.env').write_text('MUST_NOT_SHIP=synthetic')
        lock={'format':1,'platform':'linux','python':'3.14.7',**release.profile_fingerprint(self.profile)}
        self.images={}
        for role in ('app','langflow','postgres','redis'):
            cfg=self.blob(json.dumps({'architecture':'amd64','os':'linux','role':role}).encode())
            layer=self.blob(b'synthetic-shared-layer')
            manifest=self.blob(json.dumps({'schemaVersion':2,'config':{'digest':cfg},'layers':[{'digest':layer}]}).encode())
            self.images[role]={'reference':'localhost:15000/teknowra/'+role+'@'+manifest,'config_id':cfg,'platform':'linux/amd64'}
        self.bootstrap=self.r/'registry.tar';cfg=json.dumps({'architecture':'amd64','os':'linux','rootfs':{'type':'layers','diff_ids':[]}}).encode();self.bootstrap_id='sha256:'+hashlib.sha256(cfg).hexdigest();self.tag='teknowra-offline-bootstrap/registry:'+self.bootstrap_id.split(':')[1]
        with tarfile.open(self.bootstrap,'w') as t:
            for name,data in [('config.json',cfg),('manifest.json',json.dumps([{'Config':'config.json','RepoTags':[self.tag],'Layers':[]}]).encode())]:
                info=tarfile.TarInfo(name);info.size=len(data);t.addfile(info,io.BytesIO(data))
        source=self.r/'components'
        for n in release.SOURCE_FILES:
            p=source/n;p.parent.mkdir(parents=True,exist_ok=True);p.write_text('# synthetic\n')
        self.bundle=self.r/'v1.zip';release.build(source,lock,'v1',self.bundle,images={s:self.images[s]['reference'] for s in ('app','langflow')})
        self.output=self.r/'offline.zip'

    def blob(self,data):
        digest=hashlib.sha256(data).hexdigest();p=self.registry/'docker/registry/v2/blobs/sha256'/digest[:2]/digest/'data';p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data);return 'sha256:'+digest

    def build(self):
        return m.build(self.output,'offline-v1',self.registry,self.bootstrap,{'tag':self.tag,'config_id':self.bootstrap_id},self.images,[self.bundle],self.profile)

    def test_complete_transport_installs_without_host_state_or_credentials(self):
        self.build();value=m.verify(self.output);self.assertEqual(value['id'],'offline-v1')
        installed=m.install(self.output,self.r/'receiver')
        self.assertTrue((installed/'distribution.json').is_file())
        with zipfile.ZipFile(self.output) as z:
            self.assertFalse(any('.env' in n for n in z.namelist()))
            self.assertNotIn(b'MUST_NOT_SHIP',b''.join(z.read(n) for n in z.namelist()))
        self.assertEqual(m.install(self.output,self.r/'receiver'),installed)

    def test_missing_mirror_blob_fails_before_artifact_is_created(self):
        p=next(self.registry.rglob('data'));p.unlink()
        with self.assertRaises(ValueError):self.build()
        self.assertFalse(self.output.exists())

    def test_tampering_and_path_escape_never_install(self):
        self.build()
        with zipfile.ZipFile(self.output) as z:entries={n:z.read(n) for n in z.namelist()}
        for name,data in [('../escape',b'x'),('tools/distribution.py',b'changed'),('profile/.env',b'forbidden')]:
            values=dict(entries);values[name]=data;bad=self.r/'bad.zip'
            with zipfile.ZipFile(bad,'w') as z:
                for n,v in values.items():z.writestr(n,v)
            with self.assertRaises(ValueError):m.install(bad,self.r/'receiver')
            self.assertFalse((self.r/'receiver/offline-v1').exists())
        self.assertFalse((self.r/'escape').exists())

    def test_immutable_install_rejects_extra_or_changed_files(self):
        self.build();installed=m.install(self.output,self.r/'receiver')
        (installed/'profile/injected.py').write_text('unknown')
        with self.assertRaises(ValueError):m.install(self.output,self.r/'receiver')

    def test_bootstrap_archive_cannot_overwrite_unrelated_image_tags(self):
        self.tag='some-unrelated/image:latest'
        with self.assertRaises(ValueError):self.build()

    def test_bootstrap_compressed_inner_tar_is_rejected(self):
        data=self.bootstrap.read_bytes()
        with self.assertRaises(ValueError):
            import gzip
            m.bootstrap_check(gzip.compress(data),{'tag':self.tag,'config_id':self.bootstrap_id})

    def test_bootstrap_missing_layer_is_rejected(self):
        data=self.bootstrap.read_bytes()
        with tarfile.open(fileobj=io.BytesIO(data)) as original:
            values={x.name:original.extractfile(x).read() for x in original.getmembers()}
        manifest=json.loads(values['manifest.json']);manifest[0]['Layers']=['missing-layer.tar'];values['manifest.json']=json.dumps(manifest).encode()
        output=io.BytesIO()
        with tarfile.open(fileobj=output,mode='w') as t:
            for name,value in values.items():
                info=tarfile.TarInfo(name);info.size=len(value);t.addfile(info,io.BytesIO(value))
        with self.assertRaises(ValueError):m.bootstrap_check(output.getvalue(),{'tag':self.tag,'config_id':self.bootstrap_id})


if __name__=='__main__':unittest.main()
