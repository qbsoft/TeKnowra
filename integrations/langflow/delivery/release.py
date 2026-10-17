"""Allowlisted integration bundles; integrity checks do not replace a trusted build channel."""
import argparse
import hashlib
import importlib.metadata
import json
import os
import re
import shutil
import stat
import sys
import tempfile
import zipfile
from pathlib import Path, PurePosixPath

RECOVERY_FILES = ('delivery/recovery.py', 'delivery/teknowra-langflow-recovery.service')
SOURCE_FILES = (
    'teknowra_support.py', 'components/teknowra/model.py',
    'components/teknowra/knowledge.py', 'components/teknowra/mcp.py',
    'components/teknowra/skills.py', 'delivery/runtime_launcher.py', 'delivery/release.py',
    'delivery/compose.yaml', 'delivery/README.md',
) + RECOVERY_FILES
CRITICAL_SOURCES = ('lfx/components/models_and_agents/agent.py',
                    'lfx/components/input_output/chat.py', 'lfx/components/input_output/chat_output.py')
MAX_BYTES = 1024 * 1024 * 1024
MAX_MEMBER = 512 * 1024 * 1024

def canonical(value):
    return json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode('utf-8')

def digest_file(path):
    with Path(path).open('rb') as handle:
        return hashlib.file_digest(handle, 'sha256').hexdigest()

def release_id(value):
    if not isinstance(value,str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,63}',value):
        raise ValueError('invalid release id')
    return value

def runtime_lock(lib):
    profile=profile_fingerprint(lib)
    return {'format':1, 'python':sys.version.split()[0], 'platform':sys.platform, **profile}

def profile_fingerprint(lib):
    lib=Path(lib).resolve()
    packages={}
    for distribution in importlib.metadata.distributions(path=[str(lib)]):
        name=distribution.metadata.get('Name','')
        if not re.fullmatch(r'[A-Za-z0-9_.-]+', name):raise ValueError('invalid dependency name')
        normalized=re.sub(r'[-_.]+','-',name).lower()
        if normalized in packages:raise ValueError('duplicate runtime dependency')
        packages[normalized]=distribution.version
    if 'langflow-base' not in packages:raise ValueError('Langflow runtime missing')
    return {'packages':packages, 'sources':{name:digest_file(lib/name) for name in CRITICAL_SOURCES}}

def safe_file(root, name):
    root=Path(root).resolve()
    target=root/name
    current=target
    while current!=root:
        if current.is_symlink():raise ValueError('symlink forbidden')
        current=current.parent
    if not target.is_file() or not target.resolve().is_relative_to(root):raise ValueError('file outside release root')
    return target

def check_manifest(manifest):
    if manifest.get('format')!=1:raise ValueError('unsupported bundle format')
    release_id(manifest.get('release_id'))
    expected={'payload/'+name for name in SOURCE_FILES}|{'runtime-lock.json'}
    if manifest.get('platform_artifact'):
        expected.add('payload/platform/WeKnora.exe')
    legacy=expected-{'payload/'+name for name in RECOVERY_FILES}
    if set(manifest.get('files',{})) not in (expected,legacy):raise ValueError('unexpected bundle members')
    if any(not re.fullmatch('[0-9a-f]{64}',str(digest)) for digest in manifest['files'].values()):raise ValueError('invalid checksum')
    images=manifest.get('images',{})
    if manifest.get('kind')=='container':
        if set(images)!={'app','langflow'} or any(not re.fullmatch(r'[a-zA-Z0-9_./:-]+@sha256:[0-9a-f]{64}',v) for v in images.values()):
            raise ValueError('container images must use immutable digests')
    elif manifest.get('kind')!='native-validation' or images:raise ValueError('invalid delivery kind')

def build(source, lock, version, output, platform_artifact=None, images=None):
    version=release_id(version)
    source=Path(source).resolve()
    files={'payload/'+name:safe_file(source,name) for name in SOURCE_FILES}
    if platform_artifact:
        artifact=Path(platform_artifact)
        if artifact.is_symlink() or not artifact.is_file():raise ValueError('invalid platform artifact')
        files['payload/platform/WeKnora.exe']=artifact
    lock_bytes=canonical(lock)
    checksums={name:digest_file(path) for name,path in files.items()}
    checksums['runtime-lock.json']=hashlib.sha256(lock_bytes).hexdigest()
    manifest={'format':1,'release_id':version,'kind':'container' if images else 'native-validation',
              'images':images or {}, 'platform_artifact':bool(platform_artifact),'files':checksums}
    check_manifest(manifest)
    output=Path(output)
    output.parent.mkdir(parents=True,exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=output.parent,delete=False) as tmp:temp=Path(tmp.name)
    try:
        with zipfile.ZipFile(temp,'w',compression=zipfile.ZIP_STORED) as archive:
            for name in sorted(set(files)|{'manifest.json','runtime-lock.json'}):
                info=zipfile.ZipInfo(name,date_time=(2026,1,1,0,0,0));info.external_attr=(stat.S_IFREG|0o644)<<16
                with archive.open(info,'w') as out:
                    if name=='manifest.json':out.write(canonical(manifest))
                    elif name=='runtime-lock.json':out.write(lock_bytes)
                    else:
                        with files[name].open('rb') as handle:shutil.copyfileobj(handle,out,1024*1024)
        inspect_archive(temp)
        os.replace(temp,output)
    finally:temp.unlink(missing_ok=True)
    return manifest

def inspect_archive(path):
    if Path(path).stat().st_size>MAX_BYTES:raise ValueError('bundle too large')
    try:
        with zipfile.ZipFile(path) as archive:
            infos=archive.infolist()
            names=[entry.filename for entry in infos]
            if len(names)!=len(set(names)) or len(names)>32:raise ValueError('duplicate or excess members')
            if sum(entry.file_size for entry in infos)>MAX_BYTES:raise ValueError('expanded bundle too large')
            for entry in infos:
                name=entry.filename
                if '\\' in name or str(PurePosixPath(name))!=name or PurePosixPath(name).is_absolute() or '..' in PurePosixPath(name).parts:
                    raise ValueError('unsafe bundle path')
                mode=entry.external_attr>>16
                if entry.is_dir() or stat.S_ISLNK(mode) or (stat.S_IFMT(mode) not in (0,stat.S_IFREG)) or entry.file_size>MAX_MEMBER:
                    raise ValueError('unsafe bundle member')
            if 'manifest.json' not in names or archive.getinfo('manifest.json').file_size>65536:raise ValueError('manifest missing/too large')
            manifest=json.loads(archive.read('manifest.json'))
            check_manifest(manifest)
            if set(names)!=set(manifest['files'])|{'manifest.json'}:raise ValueError('undeclared member')
            for name,expected in manifest['files'].items():
                with archive.open(name) as handle:actual=hashlib.file_digest(handle,'sha256').hexdigest()
                if actual!=expected:raise ValueError('bundle checksum mismatch')
            return manifest
    except (zipfile.BadZipFile,KeyError,json.JSONDecodeError) as error:
        raise ValueError('invalid bundle') from error

def verify_installed(directory):
    directory=Path(directory)
    if directory.is_symlink() or directory.parent.is_symlink():raise ValueError('symlink release root forbidden')
    manifest_path=safe_file(directory,'manifest.json')
    manifest=json.loads(manifest_path.read_text(encoding='utf-8'))
    check_manifest(manifest)
    actual=set()
    for entry in directory.rglob('*'):
        if entry.is_symlink():raise ValueError('symlink in installed release')
        if entry.is_file():actual.add(entry.relative_to(directory).as_posix())
    if actual!=set(manifest['files'])|{'manifest.json'}:raise ValueError('unexpected installed member')
    for name,expected in manifest['files'].items():
        if digest_file(safe_file(directory,name))!=expected:raise ValueError('installed checksum mismatch')
    return manifest

def install(bundle, root):
    root=Path(root).resolve();releases=root/'releases'
    if releases.is_symlink():raise ValueError('release directory cannot be symlink')
    releases.mkdir(parents=True,exist_ok=True)
    stage=Path(tempfile.mkdtemp(prefix='.install-',dir=releases))
    try:
        # Validate and extract a private snapshot, never reopen a mutable source
        # archive after checking its paths (which would create a zip-slip race).
        snapshot=stage/'.incoming.zip'
        total=0
        with Path(bundle).open('rb') as source, snapshot.open('wb') as dest:
            while chunk:=source.read(1024*1024):
                total+=len(chunk)
                if total>MAX_BYTES:raise ValueError('bundle too large')
                dest.write(chunk)
        manifest=inspect_archive(snapshot)
        target=releases/manifest['release_id']
        if target.exists():
            if verify_installed(target)!=manifest:raise ValueError('immutable release already exists')
            return manifest
        with zipfile.ZipFile(snapshot) as archive:
            for info in archive.infolist():
                output=stage/info.filename;output.parent.mkdir(parents=True,exist_ok=True)
                with archive.open(info) as source, output.open('wb') as dest:shutil.copyfileobj(source,dest,1024*1024)
        snapshot.unlink()
        verify_installed(stage)
        stage.rename(target)
    finally:
        if stage.exists():shutil.rmtree(stage)
    return manifest

def activate(root, version, actual_runtime):
    root=Path(root).resolve();directory=root/'releases'/release_id(version)
    manifest=verify_installed(directory)
    expected=json.loads((directory/'runtime-lock.json').read_text(encoding='utf-8'))
    if expected!=actual_runtime:raise ValueError('runtime mismatch; restore the matching environment before rollback')
    pointer={'release_id':version,'manifest_sha256':digest_file(directory/'manifest.json')}
    fd,temp=tempfile.mkstemp(prefix='.current-',dir=root)
    try:
        with os.fdopen(fd,'wb') as handle:handle.write(canonical(pointer));handle.flush();os.fsync(handle.fileno())
        os.replace(temp,root/'current.json')
    finally:Path(temp).unlink(missing_ok=True)
    return pointer

def active(root):
    root=Path(root).resolve()
    pointer=json.loads(safe_file(root,'current.json').read_text(encoding='utf-8'))
    directory=root/'releases'/release_id(pointer['release_id'])
    verify_installed(directory)
    if digest_file(directory/'manifest.json')!=pointer['manifest_sha256']:raise ValueError('active manifest mismatch')
    return pointer

def container_configuration(root, profile, locations):
    root=Path(root).resolve()
    directory=root/'releases'/active(root)['release_id']
    manifest=verify_installed(directory)
    if manifest['kind']!='container':raise ValueError('native validation bundle cannot configure a container stack')
    lock=json.loads((directory/'runtime-lock.json').read_text())
    if lock['platform']!='linux' or profile_fingerprint(profile)!={key:lock[key] for key in ('packages','sources')}:
        raise ValueError('runtime profile does not match the container bundle')
    required={'APP_ENV_FILE','LANGFLOW_ENV_FILE','BINDINGS_DIR','SERVICE_CREDENTIALS_DIR','APP_RELEASE_STATE_DIR','LANGFLOW_STATE_DIR'}
    if set(locations)!=required:raise ValueError('container location set incomplete or contains overrides')
    values={key:str(Path(value).resolve()) for key,value in locations.items()}
    if values['APP_ENV_FILE']==values['LANGFLOW_ENV_FILE']:
        raise ValueError('platform and Langflow must not share an environment file')
    if any(Path(value).is_relative_to(root/'releases') for value in values.values()):
        raise ValueError('external configuration/state cannot overwrite releases')
    values.update({'TEKNOWRA_APP_IMAGE':manifest['images']['app'],'LANGFLOW_IMAGE':manifest['images']['langflow'],
        'COMPONENT_DIR':str(directory/'payload'),'PROFILE_DIR':str(Path(profile).resolve()),
        'APP_TEST_PORT':'18080','LANGFLOW_EDITOR_PORT':'17861'})
    template=json.loads((directory/'payload/delivery/compose.yaml').read_text())
    def substitute(item):
        if isinstance(item,dict):return {key:substitute(value) for key,value in item.items()}
        if isinstance(item,list):return [substitute(value) for value in item]
        if isinstance(item,str):return re.sub(r'\$\{([A-Z_]+)(?::[-?][^}]*)?\}',lambda match:values[match[1]],item)
        return item
    return substitute(template)

def main():
    parser=argparse.ArgumentParser(description=__doc__);sub=parser.add_subparsers(dest='action',required=True)
    capture=sub.add_parser('capture-runtime');capture.add_argument('--library',required=True);capture.add_argument('--output',required=True)
    build_cmd=sub.add_parser('build');build_cmd.add_argument('--source',required=True);build_cmd.add_argument('--lock',required=True);build_cmd.add_argument('--version',required=True);build_cmd.add_argument('--output',required=True);build_cmd.add_argument('--platform-artifact')
    build_cmd.add_argument('--app-image');build_cmd.add_argument('--langflow-image')
    install_cmd=sub.add_parser('install');install_cmd.add_argument('--bundle',required=True);install_cmd.add_argument('--root',required=True)
    activate_cmd=sub.add_parser('activate');activate_cmd.add_argument('--root',required=True);activate_cmd.add_argument('--version',required=True);activate_cmd.add_argument('--library',required=True)
    verify_cmd=sub.add_parser('verify');verify_cmd.add_argument('--bundle',required=True)
    container_cmd=sub.add_parser('container-config');container_cmd.add_argument('--root',required=True);container_cmd.add_argument('--profile',required=True);container_cmd.add_argument('--locations',required=True);container_cmd.add_argument('--output',required=True)
    args=parser.parse_args()
    if args.action=='capture-runtime':Path(args.output).write_bytes(canonical(runtime_lock(args.library)));print('runtime lock captured')
    elif args.action=='build':
        images={'app':args.app_image,'langflow':args.langflow_image} if args.app_image or args.langflow_image else None
        print(canonical(build(args.source,json.loads(Path(args.lock).read_text()),args.version,args.output,args.platform_artifact,images)).decode())
    elif args.action=='install':print('installed '+install(args.bundle,args.root)['release_id'])
    elif args.action=='activate':print(canonical(activate(args.root,args.version,runtime_lock(args.library))).decode())
    elif args.action=='container-config':
        generated=container_configuration(args.root,args.profile,json.loads(Path(args.locations).read_text()))
        Path(args.output).write_bytes(canonical(generated));print('verified container configuration generated; no services started')
    else:print('verified '+inspect_archive(args.bundle)['release_id'])

if __name__=='__main__':main()
