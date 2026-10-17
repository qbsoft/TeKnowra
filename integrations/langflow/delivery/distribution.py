"""Offline content-addressed image transport; requires a trusted distribution channel."""
import argparse
from email.parser import BytesParser
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import sys
import tarfile
import tempfile
import zipfile

sys.dont_write_bytecode=True
TOOLS=('distribution.py','delivery_manager.py','release.py','recovery.py')
ROLES=('app','langflow','postgres','redis')
MAX_BYTES=8*1024**3
MAX_BOOTSTRAP=128*1024**2

def load(name):
    spec=importlib.util.spec_from_file_location('delivery_'+name,Path(__file__).with_name(name+'.py'))
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

def canonical(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
def sha(data):return hashlib.sha256(data).hexdigest()
def digest(value):
    if not isinstance(value,str) or not re.fullmatch(r'sha256:[0-9a-f]{64}',value):raise ValueError('invalid digest')
    return value.split(':')[1]

def safe(name):
    if not isinstance(name,str) or not re.fullmatch(r'[A-Za-z0-9_./+\-]+',name) or str(PurePosixPath(name))!=name or name.startswith('/') or '..' in PurePosixPath(name).parts:
        raise ValueError('unsafe member')
    for part in PurePosixPath(name).parts:
        if part.rstrip(' .')!=part or part.split('.')[0].upper() in {'CON','PRN','AUX','NUL',*[f'COM{i}' for i in range(1,10)],*[f'LPT{i}' for i in range(1,10)]}:raise ValueError('unsafe platform filename')
    return name

def blob_path(value):
    h=digest(value);return 'registry/docker/registry/v2/blobs/sha256/'+h[:2]+'/'+h+'/data'

def graph(images,read):
    if set(images)!=set(ROLES):raise ValueError('image set incomplete')
    result={}
    for role,item in images.items():
        prefix='localhost:15000/teknowra/'+role+'@'
        if not item['reference'].startswith(prefix) or item['platform']!='linux/amd64':raise ValueError('unsupported image reference/platform')
        root=item['reference'][len(prefix):];digest(root);expected=item['config_id'];digest(expected)
        blobs=set();manifests=set();configs=set()
        def small(value):
            data=read(blob_path(value))
            if len(data)>1024**2 or sha(data)!=digest(value):raise ValueError('invalid image metadata')
            return json.loads(data)
        def visit(value,depth=0):
            if value in manifests:return
            if depth>8:raise ValueError('image index nesting exceeded')
            manifests.add(value);blobs.add(value);obj=small(value)
            if 'manifests' in obj:
                for child in obj['manifests']:visit(child['digest'],depth+1)
            else:
                cfg=obj['config']['digest'];digest(cfg);configs.add(cfg);blobs.add(cfg)
                config=small(cfg)
                if cfg==expected and (config.get('architecture')!='amd64' or config.get('os')!='linux'):raise ValueError('runtime architecture mismatch')
                for layer in obj['layers']:digest(layer['digest']);blobs.add(layer['digest'])
        visit(root)
        if expected not in configs:raise ValueError('runtime config is not in image manifest')
        for b in blobs:
            result[blob_path(b)]=None
            result['registry/docker/registry/v2/repositories/teknowra/'+role+'/_layers/sha256/'+digest(b)+'/link']=b.encode()
        for b in manifests:
            result['registry/docker/registry/v2/repositories/teknowra/'+role+'/_manifests/revisions/sha256/'+digest(b)+'/link']=b.encode()
    return result

def bootstrap_check(data,info):
    identity=digest(info['config_id'])
    if info['tag']!='teknowra-offline-bootstrap/registry:'+identity or len(data)>MAX_BOOTSTRAP:raise ValueError('invalid bootstrap identity')
    try:
      # docker image save produces an uncompressed outer tar. Do not accept a
      # compressed inner archive whose expansion bypasses the ZIP size budget.
      with tarfile.open(fileobj=io.BytesIO(data),mode='r:') as t:
        members={};total=0
        for item in t:
            safe(item.name.rstrip('/'));total+=item.size
            if len(members)>=4096 or item.name.casefold() in members or total>MAX_BOOTSTRAP or not (item.isfile() or item.isdir()):raise ValueError('unsafe bootstrap member')
            members[item.name.casefold()]=item
        def read(name,limit):
            safe(name);item=members.get(name.casefold())
            if item is None or not item.isfile() or item.size>limit:raise ValueError('missing/oversize bootstrap file')
            raw=t.extractfile(item).read(limit+1)
            if len(raw)!=item.size:raise ValueError('truncated bootstrap file')
            return raw
        entries=json.loads(read('manifest.json',65536))
        if len(entries)!=1 or entries[0]['RepoTags']!=[info['tag']]:raise ValueError('bootstrap must contain exactly the dedicated image tag')
        cfg=entries[0]['Config'];raw=read(cfg,1024**2)
        if sha(raw)!=identity:raise ValueError('bootstrap config mismatch')
        obj=json.loads(raw)
        if obj.get('architecture')!='amd64' or obj.get('os')!='linux':raise ValueError('bootstrap platform mismatch')
        layers=entries[0]['Layers'];diffs=obj['rootfs']['diff_ids']
        if len(layers)!=len(diffs):raise ValueError('bootstrap layer count mismatch')
        for name,expected in zip(layers,diffs):
            if sha(read(name,MAX_BOOTSTRAP))!=digest(expected):raise ValueError('bootstrap layer mismatch')
        if 'index.json' in members:
            index=json.loads(read('index.json',65536));manifests=index['manifests']
            if len(manifests)!=1:raise ValueError('bootstrap contains multiple OCI images')
            label=manifests[0].get('annotations',{}).get('io.containerd.image.name')
            if label not in (info['tag'],'docker.io/'+info['tag']):raise ValueError('bootstrap OCI tag mismatch')
            def oci(value,depth=0):
                if depth>8:raise ValueError('bootstrap index nesting exceeded')
                h=digest(value);raw=read('blobs/sha256/'+h,MAX_BOOTSTRAP)
                if sha(raw)!=h:raise ValueError('bootstrap OCI digest mismatch')
                node=json.loads(raw)
                if 'manifests' in node:
                    for child in node['manifests']:oci(child['digest'],depth+1)
                else:
                    if node['config']['digest']!=info['config_id']:raise ValueError('bootstrap OCI config mismatch')
                    for layer in node['layers']:
                        h=digest(layer['digest'])
                        if sha(read('blobs/sha256/'+h,MAX_BOOTSTRAP))!=h:raise ValueError('bootstrap OCI layer missing/corrupt')
            oci(manifests[0]['digest'])
        if 'repositories' in members:
            repositories=json.loads(read('repositories',65536));repo,tag=info['tag'].rsplit(':',1)
            if any(k not in (repo,'docker.io/'+repo) or set(v)!={tag} for k,v in repositories.items()):raise ValueError('bootstrap legacy tag mismatch')
    except (tarfile.TarError,KeyError,TypeError,json.JSONDecodeError) as error:raise ValueError('invalid bootstrap archive') from error

def layout(meta,read):
    release=load('release');release.release_id(meta['id'])
    if meta.get('format')!=1:raise ValueError('unknown distribution format')
    files=meta['files']
    if any(not re.fullmatch('[0-9a-f]{64}',str(h)) for h in files.values()):raise ValueError('invalid file checksum')
    expected={'tools/'+n for n in TOOLS}|{'tools/registry-config.json','OFFLINE.md','bootstrap/registry.tar'}
    bootstrap_check(read('bootstrap/registry.tar'),meta['bootstrap'])
    if json.loads(read('tools/registry-config.json'))!={'version':0.1,'log':{'level':'warn'},'storage':{'filesystem':{'rootdirectory':'/var/lib/registry'},'maintenance':{'readonly':{'enabled':True},'uploadpurging':{'enabled':False}}},'http':{'addr':':5000'}}:raise ValueError('unsafe registry configuration')
    g=graph(meta['images'],read);expected.update(g)
    for name,value in g.items():
        if value is not None and read(name)!=value:raise ValueError('registry link mismatch')
    if not meta['releases'] or len(set(meta['releases']))!=len(meta['releases']):raise ValueError('invalid component versions')
    lock=None
    for version in meta['releases']:
        release.release_id(version);name='bundles/'+version+'.zip';expected.add(name)
        raw=read(name)
        if len(raw)>16*1024**2:raise ValueError('component bundle too large')
        with tempfile.TemporaryDirectory() as temp:
            p=Path(temp)/'component.zip';p.write_bytes(raw);manifest=release.inspect_archive(p)
        if manifest['release_id']!=version or manifest['images']!={s:meta['images'][s]['reference'] for s in ('app','langflow')}:raise ValueError('component/image mismatch')
        with zipfile.ZipFile(io.BytesIO(raw)) as z:current=json.loads(z.read('runtime-lock.json'))
        if current.get('platform')!='linux' or (lock is not None and current!=lock):raise ValueError('runtime profiles differ')
        lock=current
    packages={}
    for name in files:
        if re.fullmatch(r'profile/[A-Za-z0-9_.+\-]+\.dist-info/METADATA',name):
            metadata=BytesParser().parsebytes(read(name));key=re.sub(r'[-_.]+','-',metadata['Name']).lower()
            if key in packages:raise ValueError('duplicate profile distribution')
            packages[key]=metadata['Version'];expected.add(name)
    sources={n:files.get('profile/'+n) for n in release.CRITICAL_SOURCES}
    expected.update('profile/'+n for n in release.CRITICAL_SOURCES)
    if packages!=lock['packages'] or sources!=lock['sources']:raise ValueError('transport profile mismatch')
    if set(files)!=expected:raise ValueError('unexpected distribution members')
    return meta

def build(output,version,registry,bootstrap,bootstrap_info,images,bundles,profile):
    release=load('release');release.release_id(version);output=Path(output)
    if output.exists():raise ValueError('immutable output already exists')
    registry,profile=Path(registry),Path(profile)
    assets={'tools/'+n:Path(__file__).with_name(n) for n in TOOLS}
    assets['tools/registry-config.json']=Path(__file__).with_name('registry-config.json')
    assets.update({'OFFLINE.md':Path(__file__).with_name('OFFLINE.md'),'bootstrap/registry.tar':Path(bootstrap)})
    def read_blob(name):
        p=registry/Path(name).relative_to('registry')
        if not p.is_file() or p.is_symlink():raise ValueError('missing/unsafe source blob')
        if p.stat().st_size>1024**2:raise ValueError('oversize image metadata')
        return p.read_bytes()
    g=graph(images,read_blob)
    for name,value in g.items():assets[name]=value if value is not None else registry/Path(name).relative_to('registry')
    versions=[]
    for bundle in bundles:
        manifest=release.inspect_archive(bundle);versions.append(manifest['release_id']);assets['bundles/'+manifest['release_id']+'.zip']=Path(bundle)
    for p in profile.glob('*.dist-info/METADATA'):assets['profile/'+p.relative_to(profile).as_posix()]=p
    for name in release.CRITICAL_SOURCES:assets['profile/'+name]=profile/name
    output.parent.mkdir(parents=True,exist_ok=True);fd,temp=tempfile.mkstemp(prefix='.transport-',dir=output.parent);os.close(fd)
    try:
        hashes={};total=0
        with zipfile.ZipFile(temp,'w',compression=zipfile.ZIP_STORED,allowZip64=True) as z:
            for name,value in sorted(assets.items()):
                safe(name);h=hashlib.sha256()
                if isinstance(value,bytes):source=io.BytesIO(value)
                else:
                    if value.is_symlink() or not value.is_file():raise ValueError('missing/unsafe source file')
                    source=value.open('rb')
                with source,z.open(name,'w',force_zip64=True) as dest:
                    while chunk:=source.read(1024**2):
                        total+=len(chunk)
                        if total>MAX_BYTES:raise ValueError('distribution too large')
                        h.update(chunk);dest.write(chunk)
                hashes[name]=h.hexdigest()
                if name.endswith('/data') and '/blobs/sha256/' in name and hashes[name]!=name.split('/')[-2]:raise ValueError('source blob digest mismatch')
            meta={'format':1,'id':version,'images':images,'bootstrap':bootstrap_info,'releases':versions,'files':hashes}
            z.writestr('distribution.json',canonical(meta))
        verify(temp);os.replace(temp,output)
    finally:Path(temp).unlink(missing_ok=True)
    return meta

def verify(path):
    if Path(path).stat().st_size>MAX_BYTES:raise ValueError('archive too large')
    try:
        with zipfile.ZipFile(path) as z:
            infos=z.infolist();names=[safe(i.filename) for i in infos]
            if len(names)>10000 or len({n.casefold() for n in names})!=len(names) or sum(i.file_size for i in infos)>MAX_BYTES:raise ValueError('duplicate/excess archive members')
            if any(i.is_dir() or stat.S_IFMT(i.external_attr>>16) not in (0,stat.S_IFREG) for i in infos):raise ValueError('unsafe member type')
            if z.getinfo('distribution.json').file_size>4*1024**2:raise ValueError('manifest too large')
            meta=json.loads(z.read('distribution.json'))
            if set(names)!=set(meta['files'])|{'distribution.json'}:raise ValueError('undeclared archive member')
            for name,expected in meta['files'].items():
                with z.open(name) as f:actual=hashlib.file_digest(f,'sha256').hexdigest()
                if actual!=expected or ('/blobs/sha256/' in name and name.endswith('/data') and actual!=name.split('/')[-2]):raise ValueError('archive checksum mismatch')
            def small(name):
                limit=MAX_BOOTSTRAP if name=='bootstrap/registry.tar' else 16*1024**2
                if z.getinfo(name).file_size>limit:raise ValueError('metadata too large')
                return z.read(name)
            return layout(meta,small)
    except (KeyError,TypeError,json.JSONDecodeError,zipfile.BadZipFile,tarfile.TarError,OSError) as e:raise ValueError('invalid distribution') from e

def verify_installed(directory):
    directory=Path(directory)
    if directory.is_symlink() or getattr(directory,'is_junction',lambda:False)():raise ValueError('unsafe installation root')
    meta=json.loads((directory/'distribution.json').read_text());actual=set()
    for p in directory.rglob('*'):
        if p.is_symlink() or getattr(p,'is_junction',lambda:False)():raise ValueError('unsafe installed member')
        if p.is_file():actual.add(p.relative_to(directory).as_posix())
    if actual!=set(meta['files'])|{'distribution.json'}:raise ValueError('installed members mismatch')
    for n,h in meta['files'].items():
        safe(n)
        with (directory/n).open('rb') as f:
            if hashlib.file_digest(f,'sha256').hexdigest()!=h:raise ValueError('installed checksum mismatch')
    def small(name):
        limit=MAX_BOOTSTRAP if name=='bootstrap/registry.tar' else 16*1024**2
        if (directory/name).stat().st_size>limit:raise ValueError('installed metadata too large')
        return (directory/name).read_bytes()
    return layout(meta,small)

def install(archive,root):
    meta=verify(archive);root=Path(root)
    if any(p.is_symlink() or getattr(p,'is_junction',lambda:False)() for p in (root,*root.parents)):raise ValueError('unsafe install parent')
    root.mkdir(parents=True,exist_ok=True);root=root.resolve();target=root/meta['id']
    if target.exists():
        if verify_installed(target)!=meta:raise ValueError('immutable distribution already exists')
        return target
    stage=Path(tempfile.mkdtemp(prefix='.install-',dir=root))
    try:
        with zipfile.ZipFile(archive) as z:
            for name,expected in meta['files'].items():
                p=stage/safe(name);p.parent.mkdir(parents=True,exist_ok=True);h=hashlib.sha256()
                with z.open(name) as source,p.open('wb') as dest:
                    while chunk:=source.read(1024**2):h.update(chunk);dest.write(chunk)
                if h.hexdigest()!=expected:raise ValueError('archive changed during installation')
        (stage/'distribution.json').write_bytes(canonical(meta));verify_installed(stage);stage.rename(target)
    finally:
        if stage.exists():
            if not stage.resolve().is_relative_to(root):raise ValueError('unsafe cleanup path')
            shutil.rmtree(stage)
    return target

def main():
    p=argparse.ArgumentParser(description=__doc__);sub=p.add_subparsers(dest='action',required=True)
    v=sub.add_parser('verify');v.add_argument('--archive',required=True)
    i=sub.add_parser('install');i.add_argument('--archive',required=True);i.add_argument('--root',required=True)
    b=sub.add_parser('build')
    for name in ('output','version','registry','bootstrap','bootstrap-info','images','profile'):b.add_argument('--'+name,required=True)
    b.add_argument('--bundle',action='append',required=True)
    args=p.parse_args()
    try:
        if args.action=='verify':print('verified '+verify(args.archive)['id'])
        elif args.action=='install':print('installed '+str(install(args.archive,args.root)))
        else:print('built '+build(args.output,args.version,args.registry,args.bootstrap,json.loads(Path(args.bootstrap_info).read_text()),json.loads(Path(args.images).read_text()),args.bundle,args.profile)['id'])
    except (ValueError,OSError):print('distribution validation failed',file=sys.stderr);return 1
    return 0

if __name__=='__main__':raise SystemExit(main())
