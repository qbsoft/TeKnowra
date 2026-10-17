"""Portable stopped-writer backup. No implicit stop/start/delete or production defaults."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import sqlite3
import stat
import subprocess
import sys
import zipfile

TABLES=('users','tenants','custom_agents','agent_shares','sessions','messages',
        'models','knowledge_bases','knowledges','chunks','tenant_skills','mcp_services')
SUPPORTED_SCHEMAS=(114,122)

class BackupError(Exception):pass

def require(value, code):
    if not value:raise BackupError(code)

def digest(path):
    h=hashlib.sha256()
    with Path(path).open('rb') as f:
        for block in iter(lambda:f.read(1024*1024),b''):h.update(block)
    return h.hexdigest()

def safe_name(name):
    p=PurePosixPath(name)
    require(bool(name) and not p.is_absolute() and '\\' not in name and ':' not in name
            and all(x not in ('','.','..') for x in name.split('/')),'unsafe-archive-path')
    return p

def link(path):
    s=path.lstat()
    return path.is_symlink() or bool(getattr(s,'st_file_attributes',0)&0x400)

def protect(path):
    if os.name=='nt':
        owner=os.environ.get('USERDOMAIN','')+'\\'+os.environ['USERNAME']
        r=subprocess.run(['icacls.exe',str(path),'/inheritance:r','/grant:r',owner+':(OI)(CI)F','*S-1-5-18:(OI)(CI)F'],capture_output=True)
        require(r.returncode==0,'private-directory-acl-failed')
    else:path.chmod(0o700)

def private_new(path):
    require(not path.exists() and not path.is_symlink(),'destination-exists')
    path.mkdir(parents=True,mode=0o700);protect(path)

def pack_state(roots, archive):
    require(not archive.exists(),'archive-exists')
    files={};directories=[]
    for label,root in roots.items():
        require(re.fullmatch(r'[a-z][a-z0-9_-]*',label) is not None,'unsafe-state-label')
        root=Path(root);require(root.is_dir() and not link(root),'invalid-state-root')
        directories.append(label)
        for p in sorted(root.rglob('*')):
            require(not link(p),'state-symlink-or-reparse-point')
            name=label+'/'+p.relative_to(root).as_posix();safe_name(name)
            if p.is_dir():directories.append(name)
            else:
                require(stat.S_ISREG(p.stat().st_mode),'state-special-file')
                files[name]={'sha256':digest(p),'bytes':p.stat().st_size,'source':p}
    total=sum(i['bytes'] for i in files.values())
    require(shutil.disk_usage(archive.parent).free>total+64*1024**2,'insufficient-state-space')
    with zipfile.ZipFile(archive,'x',compression=zipfile.ZIP_DEFLATED) as z:
        for name,item in files.items():z.write(item['source'],name)
        manifest={'format':1,'files':{n:{k:v for k,v in i.items() if k!='source'} for n,i in files.items()},'directories':directories}
        z.writestr('state.json',json.dumps(manifest,sort_keys=True))
    verify_state(archive)
    return manifest

def verify_state(archive):
    with zipfile.ZipFile(archive) as z:
        names=z.namelist();require(len(names)==len(set(names)),'duplicate-archive-member')
        for n in names:safe_name(n)
        m=json.loads(z.read('state.json'))
        require(m.get('format',1)==1 and isinstance(m.get('files'),dict) and isinstance(m.get('directories'),list),'invalid-state-manifest')
        require(set(names)==set(m['files'])|{'state.json'},'state-manifest-members-mismatch')
        require(len(m['directories'])==len(set(m['directories'])),'duplicate-state-directory')
        for d in m['directories']:safe_name(d)
        for name,item in m['files'].items():
            safe_name(name);require(name not in m['directories'],'file-directory-collision')
            require(all('/'.join(PurePosixPath(name).parts[:n]) not in m['files'] for n in range(1,len(PurePosixPath(name).parts))),'file-parent-collision')
            info=z.getinfo(name);require(not stat.S_ISLNK(info.external_attr>>16),'archive-symlink')
            require(info.file_size==item['bytes'],'state-size-mismatch')
            h=hashlib.sha256()
            with z.open(name) as f:
                for b in iter(lambda:f.read(1024*1024),b''):h.update(b)
            require(h.hexdigest()==item['sha256'],'state-hash-mismatch')
        for d in m['directories']:
            require(all('/'.join(PurePosixPath(d).parts[:n]) not in m['files'] for n in range(1,len(PurePosixPath(d).parts)+1)),'directory-file-collision')
    return m

def restore_state(archive,target):
    require(not target.exists() and not target.is_symlink(),'destination-exists')
    m=verify_state(archive) # Validate every byte and path before first destination write.
    parent=target.parent
    require(parent.exists(),'destination-parent-missing')
    require(shutil.disk_usage(parent).free>sum(i['bytes'] for i in m['files'].values())+64*1024**2,'insufficient-restore-space')
    private_new(target)
    with zipfile.ZipFile(archive) as z:
        for d in m['directories']:(target/d).mkdir(parents=True,exist_ok=True,mode=0o700)
        for name in m['files']:
            p=target/name;p.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
            with z.open(name) as src,p.open('xb') as out:shutil.copyfileobj(src,out)
            if os.name!='nt':p.chmod(0o600)
            require(digest(p)==m['files'][name]['sha256'],'restored-hash-mismatch')
    return m

def check_container(info,project,service,running):
    labels=info['Config'].get('Labels') or {};state=info['State']
    require(labels.get('com.docker.compose.project')==project and labels.get('com.docker.compose.service')==service,'container-scope-mismatch')
    require(state['Running']==running and not state.get('Paused',False),'container-not-in-required-state')

class Docker:
    def __init__(self,cfg):self.cfg=cfg
    def run(self,*args,output=None,input_file=None):
        with (Path(input_file).open('rb') if input_file else open(os.devnull,'rb')) as src:
            r=subprocess.run([self.cfg.get('docker','docker'),*args],stdin=src,stdout=output or subprocess.PIPE,stderr=subprocess.PIPE)
        require(r.returncode==0,'docker-operation-failed:'+args[0])
        return r.stdout or b''
    def container(self,service,running):
        ids=self.run('ps','-aq','--filter','label=com.docker.compose.project='+self.cfg['project'],'--filter','label=com.docker.compose.service='+service).decode().split()
        require(len(ids)==1,'expected-one-project-service-container')
        info=json.loads(self.run('inspect',ids[0]))[0]
        check_container(info,self.cfg['project'],service,running)
        return ids[0]

def config(path):
    cfg=json.loads(path.read_text(encoding='utf-8'))
    require(re.fullmatch(r'[a-z0-9][a-z0-9_-]+',cfg['project']) is not None,'invalid-project')
    require(bool(cfg['writers']) and {'app','langflow'}.issubset(cfg['writers']),'writer-list-incomplete')
    for n in (cfg['database'],cfg['db_user']):require(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',n) is not None,'unsafe-database-identifier')
    cfg['state_roots']={n:(path.parent/p).absolute() for n,p in cfg['state_roots'].items()}
    roots=list(cfg['state_roots'].values())
    require(bool(roots) and all(not link(p) for p in roots),'invalid-roots')
    require(not any(a.resolve().is_relative_to(b.resolve()) for i,a in enumerate(roots) for j,b in enumerate(roots) if i!=j),'overlapping-state-roots')
    require(type(cfg.get('schema')) is int and cfg['schema'] in SUPPORTED_SCHEMAS,'unsupported-declared-schema')
    return cfg

def sql(d,pg,database,query):
    return d.run('exec',pg,'psql','-U',d.cfg['db_user'],'-d',database,'-Atc',query).decode().strip()

def db_facts(d,pg,database,schema=None):
    expected=d.cfg.get('schema',114) if schema is None else schema
    require(type(expected) is int and expected in SUPPORTED_SCHEMAS,'unsupported-declared-schema')
    require(sql(d,pg,database,'SELECT version,dirty FROM schema_migrations')==str(expected)+'|f','schema-not-declared-clean')
    return {t:int(sql(d,pg,database,'SELECT count(*) FROM '+t)) for t in TABLES}

def verify_backup(root):
    m=json.loads((root/'backup.json').read_text(encoding='utf-8'))
    require(m.get('format')==1 and type(m.get('schema')) is int and m['schema'] in SUPPORTED_SCHEMAS,'unsupported-backup')
    require(set(m['artifacts'])=={'state.zip','platform.dump'},'backup-artifact-list-mismatch')
    for name,item in m['artifacts'].items():
        p=root/name;require(p.is_file() and not link(p),'backup-artifact-missing')
        require(p.stat().st_size==item['bytes'] and digest(p)==item['sha256'],'backup-artifact-hash-mismatch')
    verify_state(root/'state.zip')
    return m

def backup(cfg,out):
    d=Docker(cfg)
    writers={s:d.container(s,False) for s in cfg['writers']};pg=d.container(cfg.get('postgres_service','postgres'),True)
    for p in cfg['state_roots'].values():require(not out.resolve().is_relative_to(p.resolve()),'backup-inside-state')
    counts=db_facts(d,pg,cfg['database'])
    source_bytes=sum(p.stat().st_size for r in cfg['state_roots'].values() for p in r.rglob('*') if p.is_file())
    db_bytes=int(sql(d,pg,cfg['database'],'SELECT pg_database_size(current_database())'))
    require(shutil.disk_usage(out.parent).free>source_bytes+db_bytes+int(cfg.get('reserve_gib',8))*1024**3,'insufficient-backup-space')
    for relative in cfg.get('sqlite_files',[]):
        safe_name(relative);parts=PurePosixPath(relative).parts
        path=cfg['state_roots'][parts[0]].joinpath(*parts[1:])
        require(path.is_file() and not link(path),'sqlite-missing')
        db=sqlite3.connect(path.resolve().as_uri()+'?mode=ro',uri=True)
        try:require(db.execute('pragma integrity_check').fetchone()[0]=='ok','sqlite-integrity-failed')
        finally:db.close()
    private_new(out)
    with (out/'platform.dump').open('xb') as f:
        d.run('exec',pg,'pg_dump','-U',cfg['db_user'],'-d',cfg['database'],'-Fc','--no-owner','--no-acl',output=f)
    m=pack_state(cfg['state_roots'],out/'state.zip')
    require(db_facts(d,pg,cfg['database'])==counts,'database-changed-during-backup')
    for s,cid in writers.items():require(d.container(s,False)==cid,'writer-container-replaced')
    # Deployment-specific skill snapshots must be transferred as private assets.
    image_refs=sql(d,pg,cfg['database'],"SELECT DISTINCT config->'skill_image'->>'snapshot_id' FROM tenant_sandbox_configs WHERE deleted_at IS NULL AND config->'skill_image'->>'snapshot_id' IS NOT NULL").splitlines()
    images={ref:json.loads(d.run('image','inspect',ref))[0]['Id'] for ref in image_refs}
    record={'format':1,'project':cfg['project'],'schema':cfg['schema'],'counts':counts,'files':len(m['files']),'skill_images':images,'skill_images_exported':False,'writers_stopped':True,'artifacts':{n:{'bytes':(out/n).stat().st_size,'sha256':digest(out/n)} for n in ('state.zip','platform.dump')}}
    (out/'backup.json').write_text(json.dumps(record,indent=2)+'\n',encoding='utf-8')
    verify_backup(out)
    return {'backup_verified':True,'state_files':record['files'],'skill_images_require_separate_export':len(images)}

def restore_database(d,pg,database,dump,schema=114):
    require(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',database) is not None,'unsafe-database-identifier')
    require(type(schema) is int and schema in SUPPORTED_SCHEMAS,'unsupported-restore-schema')
    exists=sql(d,pg,'postgres',"SELECT 1 FROM pg_database WHERE datname='"+database+"'")
    require(exists=='','restore-database-already-exists')
    d.run('exec',pg,'createdb','-U',d.cfg['db_user'],'--template=template0',database)
    d.run('exec','-i',pg,'pg_restore','-U',d.cfg['db_user'],'-d',database,'--no-owner','--no-acl','--exit-on-error',input_file=dump)
    return db_facts(d,pg,database,schema)

def main():
    p=argparse.ArgumentParser(description=__doc__);sub=p.add_subparsers(dest='action',required=True)
    b=sub.add_parser('backup');b.add_argument('--config',type=Path,required=True);b.add_argument('--out',type=Path,required=True)
    v=sub.add_parser('verify');v.add_argument('--backup',type=Path,required=True)
    s=sub.add_parser('restore-state');s.add_argument('--backup',type=Path,required=True);s.add_argument('--target',type=Path,required=True)
    r=sub.add_parser('restore-db');r.add_argument('--backup',type=Path,required=True);r.add_argument('--config',type=Path,required=True);r.add_argument('--database',required=True)
    a=p.parse_args()
    try:
        if a.action=='backup':result=backup(config(a.config),a.out)
        else:
            m=verify_backup(a.backup)
            if a.action=='verify':result={'backup_verified':True,'state_files':m['files']}
            elif a.action=='restore-state':restore_state(a.backup/'state.zip',a.target);result={'state_restored':True,'state_files':m['files']}
            else:
                cfg=config(a.config)
                require(cfg['schema']==m['schema'],'restore-config-backup-schema-mismatch')
                d=Docker(cfg)
                for service in cfg['writers']:d.container(service,False)
                pg=d.container(cfg.get('postgres_service','postgres'),True)
                counts=restore_database(d,pg,a.database,a.backup/'platform.dump',m['schema'])
                require(counts==m['counts'],'restored-row-counts-mismatch');result={'database_restored':True,'schema':m['schema'],'row_counts_match':True}
        print(json.dumps(result))
    except BackupError as exc:
        print('Backup/restore refused: '+str(exc),file=sys.stderr);return 1
    except (OSError,ValueError,KeyError,TypeError,sqlite3.Error,zipfile.BadZipFile):
        # No source environment, secret, SQL dump, or Docker stderr in public output.
        print('Backup/restore refused or failed; preserve target and inspect configuration/state privately.',file=sys.stderr);return 1
    return 0

if __name__=='__main__':sys.exit(main())
