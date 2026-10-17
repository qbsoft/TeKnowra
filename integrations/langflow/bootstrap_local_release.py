"""Initialize two ordinary local Langflow identities; never print credentials.

Run once while the old explicit local auto-login is enabled. Afterwards launch
Langflow with AUTO_LOGIN=false and ENABLE_SIGNUP=false. Files remain in the
existing test environment's data directory, outside the Git checkout.
"""
import argparse,csv,io,json,re,secrets,subprocess
from pathlib import Path
import httpx

def safe(response):
    if response.status_code not in (200,201):
        raise RuntimeError('Local Langflow setup failed (HTTP '+str(response.status_code)+')')
    return response.json()

def clear_transient_fields(graph):
    """Copied drafts must not inherit temporary capabilities from an old test."""
    for node in graph.get('nodes', []):
        template=node.get('data',{}).get('node',{}).get('template',{})
        for name in ('directory_connection','task_capability','bridge_url','node_key','tools_json'):
            field=template.get(name)
            if isinstance(field,dict):
                field['value']=''
                field['load_from_db']=False
                field['is_global']=False
    return graph

def restrict(path):
    # Only the current Windows user and SYSTEM can read local service secrets.
    identity=subprocess.run(['whoami.exe','/user','/fo','csv','/nh'],capture_output=True,text=True,encoding='mbcs',timeout=15)
    if identity.returncode:raise RuntimeError('Could not identify credential owner')
    sid=list(csv.reader(io.StringIO(identity.stdout)))[0][1]
    if not re.fullmatch(r'S-\d+(?:-\d+)+',sid):raise RuntimeError('Invalid credential owner')
    result=subprocess.run(['icacls.exe',str(path),'/inheritance:r','/grant:r','*'+sid+':F','*S-1-5-18:F'],capture_output=True,timeout=30)
    if result.returncode:raise RuntimeError('Could not restrict local credential file')

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--data-dir',required=True);parser.add_argument('--bindings',required=True);args=parser.parse_args()
    data=Path(args.data_dir).resolve();data.mkdir(parents=True,exist_ok=True)
    state_path=data/'teknowra-release-identities.local.json'
    state=json.loads(state_path.read_text(encoding='utf-8')) if state_path.exists() else {}
    if not state_path.exists():state_path.write_text('{}',encoding='utf-8')
    restrict(state_path)
    with httpx.Client(base_url='http://127.0.0.1:7861',trust_env=False,timeout=45) as admin:
        if not state.get('editor') or not state.get('runner'):
            safe(admin.get('/api/v1/auto_login'))
        for kind,username in [('editor','teknowra-editor'),('runner','teknowra-runner')]:
            account=state.get(kind)
            if account is None:
                password=secrets.token_urlsafe(32)
                user=safe(admin.post('/api/v1/users/',json={'username':username,'password':password,'is_active':True,'is_superuser':False}))
                if user.get('is_superuser'):raise RuntimeError('Setup refused a privileged service identity')
                if not user.get('is_active'):safe(admin.patch('/api/v1/users/'+user['id'],json={'is_active':True}))
                account={'username':username,'password':password,'id':user['id']};state[kind]=account
                state_path.write_text(json.dumps(state),encoding='utf-8');restrict(state_path)
            with httpx.Client(base_url='http://127.0.0.1:7861',trust_env=False,timeout=45) as user_client:
                safe(user_client.post('/api/v1/login',data={'username':account['username'],'password':account['password']}))
                if not account.get('api_key'):
                    key=safe(user_client.post('/api/v1/api_key/',json={'name':'TeKnowra '+kind+' local service'}))
                    account['api_key']=key['api_key'];state_path.write_text(json.dumps(state),encoding='utf-8');restrict(state_path)
        if state['editor']['id']==state['runner']['id']:raise RuntimeError('Identities not separated')
        source=json.loads(Path(args.bindings).read_text(encoding='utf-8'))
        with httpx.Client(base_url='http://127.0.0.1:7861',headers={'X-API-Key':state['editor']['api_key']},trust_env=False,timeout=45) as editor:
            if not state.get('drafts'):
                safe(admin.get('/api/v1/auto_login'))
            drafts=state.setdefault('drafts',{})
            for binding in source['bindings']:
                agent=binding['agent_id']
                if agent not in drafts:
                    old=safe(admin.get('/api/v1/flows/'+binding['flow_id']))
                    created=safe(editor.post('/api/v1/flows/',json={'name':old['name']+' 编辑草稿','data':clear_transient_fields(old['data']),'is_component':False,'access_type':'PRIVATE'}))
                    if created['user_id']!=state['editor']['id']:raise RuntimeError('Draft owner mismatch')
                    drafts[agent]=created['id'];state_path.write_text(json.dumps(state),encoding='utf-8');restrict(state_path)
                binding['flow_id']=drafts[agent]
    credentials=data/'teknowra-release-service.local.json'
    if not credentials.exists():credentials.write_text('{}',encoding='utf-8')
    restrict(credentials)
    credentials.write_text(json.dumps({'editor_api_key':state['editor']['api_key'],'runtime_api_key':state['runner']['api_key'],'runtime_owner_id':state['runner']['id']}),encoding='utf-8');restrict(credentials)
    (data/'release-bindings.local.json').write_text(json.dumps(source,ensure_ascii=False,indent=2),encoding='utf-8')
    print('Ordinary editor and runner initialized; isolated drafts saved; local credentials restricted.')
    print('Draft count:',len(state['drafts']))

if __name__=='__main__':main()
