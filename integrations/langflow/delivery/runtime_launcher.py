"""Launch one verified installed bundle; secrets and database stay outside releases."""
import argparse
import base64
import importlib.util
import json
import os
import subprocess
import sys
import sysconfig
from pathlib import Path
sys.dont_write_bytecode=True

def require_interpreter_library(library):
    library=Path(library).resolve()
    actual=Path(sysconfig.get_path('purelib')).resolve()
    if library!=actual:raise ValueError('library does not belong to the running Python interpreter')
    module=importlib.util.find_spec('langflow')
    if module is None or not module.origin or not Path(module.origin).resolve().is_relative_to(actual):
        raise ValueError('Langflow would import outside the verified interpreter library')

def require_external_state(install_root, state):
    if Path(state).resolve().is_relative_to(Path(install_root).resolve()/'releases'):
        raise ValueError('mutable state must stay outside every installed release')

def environment(payload, state, identity, temporary):
    payload=Path(payload).resolve();state=Path(state).resolve();temporary=Path(temporary).resolve()
    bootstrap=identity.get('bootstrap',{})
    if not bootstrap.get('username') or not bootstrap.get('password') or not identity.get('secret_key'):
        raise ValueError('external bootstrap identity is incomplete')
    try:
        raw_key=base64.b64decode(identity['secret_key'].encode(),altchars=b'-_',validate=True)
    except (ValueError,TypeError):
        raise ValueError('Langflow secret_key must be a URL-safe base64 encoded 32-byte Fernet key') from None
    if len(raw_key)!=32:raise ValueError('Langflow secret_key must encode exactly 32 bytes')
    immutable=payload.parent if payload.name=='payload' else payload
    if state.is_relative_to(immutable) or temporary.is_relative_to(immutable):
        raise ValueError('mutable state cannot be stored in an immutable release')
    env={key:value for key,value in os.environ.items() if key.upper() in (
        'PATH','SYSTEMROOT','WINDIR','COMSPEC','PATHEXT','USERPROFILE','HOME','LANG','LC_ALL','SSL_CERT_FILE','SSL_CERT_DIR')}
    env.update({'LANGFLOW_CONFIG_DIR':str(state),'LANGFLOW_DATABASE_URL':'sqlite:///'+str(state/'langflow.db').replace('\\','/'),
        'LANGFLOW_COMPONENTS_PATH':str(payload/'components'),'PYTHONPATH':str(payload),
        'LANGFLOW_AUTO_LOGIN':'false','LANGFLOW_ENABLE_SIGNUP':'false','LANGFLOW_STORE_ENVIRONMENT_VARIABLES':'false',
        'LANGFLOW_SUPERUSER':bootstrap['username'],'LANGFLOW_SUPERUSER_PASSWORD':bootstrap['password'],
        'LANGFLOW_SECRET_KEY':identity['secret_key'],'LANGFLOW_DO_NOT_TRACK':'true','DO_NOT_TRACK':'true',
        'PYTHONDONTWRITEBYTECODE':'1',
        'PYTHONNOUSERSITE':'1',
        'LANGFLOW_LOG_LEVEL':'ERROR','LANGFLOW_WORKERS':'1','TEMP':str(temporary),'TMP':str(temporary),
        'HF_HOME':str(temporary/'huggingface')})
    return env

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root',required=True);parser.add_argument('--library',required=True)
    parser.add_argument('--state',required=True);parser.add_argument('--identity-file',required=True)
    parser.add_argument('--port',type=int,default=17861)
    args=parser.parse_args()
    if not 1024<=args.port<=65535:raise ValueError('invalid service port')
    require_interpreter_library(args.library)
    require_external_state(args.root,args.state)
    require_external_state(args.root,args.identity_file)
    spec=importlib.util.spec_from_file_location('release',Path(__file__).with_name('release.py'))
    release=importlib.util.module_from_spec(spec);spec.loader.exec_module(release)
    pointer=release.active(args.root)
    directory=Path(args.root).resolve()/'releases'/pointer['release_id']
    lock=json.loads((directory/'runtime-lock.json').read_text())
    if lock!=release.runtime_lock(args.library):raise ValueError('runtime mismatch; launch refused')
    state=Path(args.state).resolve();temporary=state/'tmp'
    identity=json.loads(Path(args.identity_file).read_text(encoding='utf-8'))
    env=environment(directory/'payload',state,identity,temporary)
    state.mkdir(parents=True,exist_ok=True);temporary.mkdir(parents=True,exist_ok=True)
    print('Starting verified release '+pointer['release_id']+' on loopback port '+str(args.port),flush=True)
    return subprocess.call([sys.executable,'-m','langflow','run','--host','127.0.0.1','--port',str(args.port)],env=env,cwd=state)

if __name__=='__main__':raise SystemExit(main())
