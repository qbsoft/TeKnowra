"""Run the existing local installation with native service authentication."""
import argparse,json,os,secrets,subprocess
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--environment',required=True);args=parser.parse_args()
environment=Path(args.environment).resolve();source=Path(__file__).resolve().parent
from bootstrap_local_release import restrict
state_path=environment/'data/teknowra-release-identities.local.json'
state=json.loads(state_path.read_text(encoding='utf-8'))
if 'bootstrap' not in state:
    state['bootstrap']={'username':'langflow','password':secrets.token_urlsafe(40)}
    restrict(state_path);state_path.write_text(json.dumps(state),encoding='utf-8')
env=os.environ.copy();env.pop('OPENAI_API_KEY',None)
env.update({'LANGFLOW_STORE_ENVIRONMENT_VARIABLES':'false','LANGFLOW_CONFIG_DIR':str(environment/'data'),'LANGFLOW_DATABASE_URL':'sqlite:///'+str(environment/'data/langflow.db').replace('\\','/'),'LANGFLOW_COMPONENTS_PATH':str(source/'components'),'PYTHONPATH':str(source),'LANGFLOW_AUTO_LOGIN':'false','LANGFLOW_ENABLE_SIGNUP':'false','LANGFLOW_DO_NOT_TRACK':'true','DO_NOT_TRACK':'true','LANGFLOW_LOG_LEVEL':'ERROR','TEMP':str(environment/'temp'),'TMP':str(environment/'temp'),'HF_HOME':str(environment/'cache/huggingface')})
env['LANGFLOW_SUPERUSER']=state['bootstrap']['username']
env['LANGFLOW_SUPERUSER_PASSWORD']=state['bootstrap']['password']
raise SystemExit(subprocess.call([str(environment/'venv/Scripts/langflow.exe'),'run','--host','127.0.0.1','--port','7861'],env=env))
