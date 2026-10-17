"""Host-side import and lifecycle entry points for a verified offline distribution."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
import uuid

sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('offline_distribution',Path(__file__).with_name('distribution.py'))
distribution=importlib.util.module_from_spec(spec);spec.loader.exec_module(distribution)

class ManagerError(RuntimeError):pass

class Client:
    def __init__(self,docker):self.docker=docker
    def run(self,args,timeout=30):
        try:r=subprocess.run([self.docker,*args],capture_output=True,text=True,encoding='utf-8',timeout=timeout)
        except (OSError,subprocess.TimeoutExpired):raise ManagerError('docker-unavailable-or-timeout') from None
        if r.returncode:raise ManagerError('docker-command-failed')
        return r.stdout.strip()
    def identity(self,ref,optional=False):
        template='{"id":{{json .Id}},"os":{{json .Os}},"architecture":{{json .Architecture}},"digests":{{json .RepoDigests}}}'
        try:return json.loads(self.run(['image','inspect',ref,'--format',template]))
        except ManagerError:
            if optional:return None
            raise
    def token(self,name):
        args=[self.docker,'container','inspect',name,'--format','{{json (index .Config.Labels "teknowra.operation")}}']
        try:r=subprocess.run(args,capture_output=True,text=True,encoding='utf-8',timeout=15)
        except (OSError,subprocess.TimeoutExpired):raise ManagerError('container-inspection-failed') from None
        if r.returncode:
            if 'No such' in r.stderr:return None
            raise ManagerError('container-inspection-failed')
        return json.loads(r.stdout)

def remove_owned(client,name,token):
    actual=client.token(name)
    if actual is None:return
    if actual!=token:raise ManagerError('temporary-container-owner-mismatch')
    client.run(['rm','-f',name])

def project_name(value):
    if not re.fullmatch(r'[a-z0-9][a-z0-9_-]{1,62}',value):raise ManagerError('invalid-project')

def ensure_port_free():
    try:
        with socket.socket() as s:s.bind(('127.0.0.1',15000))
    except OSError:raise ManagerError('import-port-15000-in-use') from None

def registry_ready():
    opener=urllib.request.build_opener(urllib.request.ProxyHandler({}));end=time.monotonic()+30
    while time.monotonic()<end:
        try:
            with opener.open('http://127.0.0.1:15000/v2/',timeout=2) as r:
                if r.status==200:return
        except OSError:pass
        time.sleep(.5)
    raise ManagerError('import-registry-not-ready')

def image_check(client,item):
    v=client.identity(item['reference'])
    if v['id']!=item['config_id'] or v['os']!='linux' or v['architecture']!='amd64' or item['reference'] not in (v['digests'] or []):raise ManagerError('cached-image-identity-mismatch')

def import_images(installed,docker,project,client=None):
    project_name(project);installed=Path(installed).resolve();meta=distribution.verify_installed(installed)
    client=client or Client(docker);ensure_port_free();bootstrap=meta['bootstrap']
    present=client.identity(bootstrap['tag'],optional=True)
    if present and present['id']!=bootstrap['config_id']:raise ManagerError('bootstrap-tag-collision')
    if not present:
        client.run(['image','load','-i',str(installed/'bootstrap/registry.tar')],timeout=120)
        present=client.identity(bootstrap['tag'])
    if present['id']!=bootstrap['config_id']:raise ManagerError('bootstrap-load-mismatch')
    container=None;token=uuid.uuid4().hex;name=project+'-import-'+token[:8]
    try:
        container=client.run(['run','-d','--rm','--name',name,'--label','teknowra.operation='+token,
            '--label','teknowra.distribution='+meta['id'],'--memory','128m','--cpus','1',
            '-p','127.0.0.1:15000:5000','--mount','type=bind,source='+str(installed/'registry')+',target=/var/lib/registry,readonly',
            '--mount','type=bind,source='+str(installed/'tools/registry-config.json')+',target=/etc/docker/registry/config.yml,readonly',bootstrap['config_id']])
        registry_ready()
        for item in meta['images'].values():
            client.run(['pull',item['reference']],timeout=240);image_check(client,item)
        return {'distribution':meta['id'],'images_verified':len(meta['images']),'registry_removed':True}
    finally:
        if container:client.run(['rm','-f',container])
        else:remove_owned(client,name,token)

def atomic(path,data):
    path=Path(path);path.parent.mkdir(parents=True,exist_ok=True)
    if data is None:path.unlink(missing_ok=True);return
    fd,tmp=tempfile.mkstemp(prefix='.state-',dir=path.parent)
    try:
        with os.fdopen(fd,'wb') as f:f.write(data);f.flush();os.fsync(f.fileno())
        os.replace(tmp,path)
    finally:Path(tmp).unlink(missing_ok=True)

def switch(installed,root,version,locations,overlay,project,docker,config,client=None,startup_timeout=180):
    project_name(project);installed=Path(installed).resolve();meta=distribution.verify_installed(installed)
    if version not in meta['releases']:raise ManagerError('release-not-in-distribution')
    client=client or Client(docker)
    for item in meta['images'].values():image_check(client,item)
    release=distribution.load('release');recovery=distribution.load('recovery')
    root=Path(root).resolve();config=Path(config).resolve();overlay=Path(overlay).resolve();locations=Path(locations).resolve()
    if config.is_relative_to(root/'releases') or config.is_relative_to(installed):raise ManagerError('mutable-config-in-immutable-release')
    external=json.loads(locations.read_text(encoding='utf-8'))
    if any(Path(p).resolve().is_relative_to(installed) for p in external.values()):raise ManagerError('mutable-state-in-distribution')
    extra=json.loads(overlay.read_text(encoding='utf-8'))
    if set(extra.get('services',{}))-{'app','postgres','redis'}:raise ManagerError('unexpected-overlay-service')
    if set(extra.get('services',{}).get('app',{}))-{'volumes','depends_on'}:raise ManagerError('app-overlay-replacement-forbidden')
    for service in ('postgres','redis'):
        if extra['services'][service]['image']!=meta['images'][service]['reference']:raise ManagerError('infrastructure-image-not-pinned')
    root.mkdir(parents=True,exist_ok=True)
    try:
        with recovery.exclusive(root/'.recovery.lock'):
            for v in meta['releases']:release.install(installed/'bundles'/(v+'.zip'),root)
            pointer=root/'current.json';previous=pointer.read_bytes() if pointer.exists() else None
            previous_config=config.read_bytes() if config.exists() else None
            receipt=root/'transactions'/uuid.uuid4().hex;receipt.mkdir(parents=True)
            if previous is not None:(receipt/'current.before.json').write_bytes(previous)
            if previous_config is not None:(receipt/'compose.before.json').write_bytes(previous_config)
            compose=['compose','-p',project,'-f',str(config),'-f',str(overlay)]
            activated=False;services_touched=False
            try:
                activated=True
                # Validate in an isolated staging root. A timed-out Docker CLI
                # cannot leave a daemon container capable of late-writing the
                # real pointer. Only the host atomically publishes that file,
                # retaining ownership for ordinary Linux deployment users.
                staging=receipt/'activation';staging.mkdir();(staging/'releases').mkdir()
                operation=uuid.uuid4().hex;activation=project+'-activate-'+operation[:8]
                uid=str(os.getuid())+':'+str(os.getgid()) if hasattr(os,'getuid') else '1000:1000'
                try:
                    validated=client.run(['run','--rm','--name',activation,'--label','teknowra.operation='+operation,'--network','none','--memory','192m','--cpus','1','--user',uid,
                        '--mount','type=bind,source='+str(staging)+',target=/install',
                        '--mount','type=bind,source='+str(root/'releases')+',target=/install/releases,readonly',
                        '--mount','type=bind,source='+str(installed/'tools')+',target=/tools,readonly',
                        '--entrypoint','python',meta['images']['langflow']['reference'],'-B','/tools/release.py','activate',
                        '--root','/install','--version',version,'--library','/app/.venv/lib/python3.14/site-packages'],timeout=90)
                finally:remove_owned(client,activation,operation)
                pointer_value={'release_id':version,'manifest_sha256':release.digest_file(root/'releases'/version/'manifest.json')}
                if json.loads(validated)!=pointer_value:raise ManagerError('activation-result-mismatch')
                atomic(pointer,distribution.canonical(pointer_value))
                desired=release.container_configuration(root,installed/'profile',external)
                # Use the recovery preflight to reject overlay replacement of
                # component mounts before stopping any running service.
                atomic(config,distribution.canonical(desired))
                args=argparse.Namespace(root=str(root),profile=str(installed/'profile'),locations=str(locations),compose=str(config),overlay=[str(overlay)])
                sys.path.insert(0,str(installed/'tools'))
                try:recovery.deployment(args)
                finally:sys.path.pop(0)
                probe=recovery.Docker(docker,project,[config,overlay])
                allowed={s:{meta['images'][s]['reference']} for s in ('app','langflow')}
                if previous:
                    old=json.loads(previous)['release_id'];old_manifest=release.verify_installed(root/'releases'/old)
                    for s in allowed:allowed[s].add(old_manifest['images'][s])
                for s in allowed:
                    existing=probe.inspect(s)
                    if existing and (existing['project']!=project or existing['service']!=s or existing['image'] not in allowed[s]):raise ManagerError('existing-service-not-owned-by-release')
                client.run([*compose,'up','-d','--no-build','--pull','never','--wait','--wait-timeout',str(startup_timeout),'postgres','redis'],timeout=startup_timeout+15)
                services_touched=True
                client.run([*compose,'stop','langflow','app'],timeout=45)
                client.run([*compose,'up','-d','--no-deps','--no-build','--pull','never','--force-recreate','--wait','--wait-timeout',str(startup_timeout),'app'],timeout=startup_timeout+15)
                client.run([*compose,'up','-d','--no-deps','--no-build','--pull','never','--force-recreate','langflow'],timeout=60)
                end=time.monotonic()+startup_timeout
                while time.monotonic()<end:
                    a,l=probe.inspect('app'),probe.inspect('langflow')
                    if a and l and a['running'] and l['running'] and a['health']=='healthy' and l['network']=='container:'+a['id']:
                        if probe.namespace('app',a)==probe.namespace('langflow',l) and probe.healthy(l):break
                    time.sleep(2)
                else:raise ManagerError('group-not-ready')
                result={'status':'ready','version':version,'project':project,'database_downgraded':False}
                (receipt/'result.json').write_bytes(distribution.canonical(result));return result
            except Exception:
                if services_touched:
                    try:client.run([*compose,'stop','langflow','app'],timeout=45)
                    except ManagerError:pass
                if activated:atomic(pointer,previous)
                atomic(config,previous_config)
                (receipt/'result.json').write_bytes(distribution.canonical({'status':'failed','pointer_and_config_restored':True,'services_require_explicit_recovery':services_touched,'database_downgraded':False}))
                raise ManagerError('switch-failed-see-local-transaction') from None
    except recovery.RecoveryError:raise ManagerError('stop-recovery-monitor-before-maintenance') from None

def main():
    p=argparse.ArgumentParser(description=__doc__);sub=p.add_subparsers(dest='action',required=True)
    i=sub.add_parser('import-images');s=sub.add_parser('switch')
    for c in (i,s):
        c.add_argument('--installed',required=True);c.add_argument('--project',required=True);c.add_argument('--docker',default='docker')
    for name in ('root','version','locations','overlay','config'):s.add_argument('--'+name,required=True)
    s.add_argument('--startup-timeout',type=int,default=180);args=p.parse_args()
    try:
        if args.action=='import-images':v=import_images(args.installed,args.docker,args.project)
        else:v=switch(args.installed,args.root,args.version,args.locations,args.overlay,args.project,args.docker,args.config,startup_timeout=args.startup_timeout)
        print(json.dumps(v));return 0
    except (ManagerError,ValueError,OSError,KeyError):print('delivery-operation-failed',file=sys.stderr);return 1

if __name__=='__main__':raise SystemExit(main())
