"""Metadata-only design connections; runtime credentials never enter this cache."""
import copy
import base64
import hashlib
import json
import re
import threading
import time
from urllib.parse import urlsplit

import httpx
from jsonschema import Draft202012Validator
from lfx.io import SecretStrInput, StrInput
from lfx.schema.json_schema import create_input_schema_from_json_schema
from pydantic import BaseModel

_connections = {}
_lock = threading.Lock()
DIRECTORY_URL = 'http://127.0.0.1:8080/api/v1/langflow/design/catalog'

def decode_tool_descriptors(value):
    # Langflow text inputs unescape JSON strings. Base64 preserves exact schema
    # and descriptions containing newlines, quotes and backslashes.
    if value.startswith('base64:'):
        value=base64.b64decode(value[7:],validate=True).decode('utf-8')
    result=json.loads(value)
    if not isinstance(result,list):raise ValueError('工具目录必须为列表')
    return result

def resource_id(value):
    return str(value or '').rsplit(' · ', 1)[-1]

def tool_name(service_id, name):
    prefix = hashlib.sha256((service_id+'/'+name).encode()).hexdigest()[:12]
    return 'tk_' + prefix + '_' + re.sub(r'[^a-zA-Z0-9_-]', '_', name)[:40]

def validate_arguments(schema, args):
    def check_refs(value):
        if isinstance(value, dict):
            if '$ref' in value and not value['$ref'].startswith('#/'):
                raise ValueError('本阶段不支持外部schema引用')
            for item in value.values():check_refs(item)
        elif isinstance(value, list):
            for item in value:check_refs(item)
    check_refs(schema)
    try:
        Draft202012Validator.check_schema(schema)
        Draft202012Validator(schema).validate(args)
    except Exception as exc:
        raise ValueError('工具参数不符合发布的schema') from exc

def schema_model(schema):
    validate_arguments_schema_only(schema)
    model = create_input_schema_from_json_schema(schema)
    # Keep the original enum/constraints visible to the model and Actions.
    model.model_json_schema = classmethod(lambda cls, *a, **kw: copy.deepcopy(schema))
    return model

def wire_arguments(value):
    if isinstance(value, BaseModel):
        return wire_arguments(value.model_dump(by_alias=True, exclude_unset=True))
    if isinstance(value, dict):return {key:wire_arguments(item) for key,item in value.items()}
    if isinstance(value, list):return [wire_arguments(item) for item in value]
    return value

def validate_arguments_schema_only(schema):
    if not isinstance(schema, dict) or schema.get('type') != 'object':
        raise ValueError('工具输入必须为JSON对象schema')
    Draft202012Validator.check_schema(schema)

def design_inputs():
    return [
        SecretStrInput(name='directory_connection', display_name='连接 TeKnowra 资源目录', value='', real_time_refresh=True, info='粘贴TeKnowra研发入口生成的15分钟只读目录连接，成功刷新后立即清空；不用于业务执行'),
        StrInput(name='directory_agent_id', display_name='目录所属智能体', value='', advanced=True),
        StrInput(name='bridge_url', display_name='运行能力地址', value='', advanced=True),
        SecretStrInput(name='task_capability', display_name='任务能力', value='', advanced=True),
        StrInput(name='node_key', display_name='运行节点', value='', advanced=True),
    ]

def refresh_fields(config, catalog, field, source):
    config['directory_connection']['value'] = ''
    rows = catalog.get(source, [])
    options = [row['name'] + ' · ' + row['id'] for row in rows]
    selected = resource_id(config[field].get('value'))
    config[field]['options'] = options
    matches = [option for option in options if resource_id(option) == selected]
    config[field]['value'] = matches[0] if matches else ''
    if 'directory_agent_id' in config:config['directory_agent_id']['value'] = catalog['agent_id']
    return config

def _load(url, ticket):
    if url != DIRECTORY_URL or not re.fullmatch(r'[0-9a-f]{64}', ticket or ''):
        raise ValueError('只接受TeKnowra本机研发入口生成的目录连接')
    try:
        response = httpx.get(url, headers={'X-TeKnowra-Design-Ticket':ticket}, timeout=8, follow_redirects=False, trust_env=False)
        response.raise_for_status()
        return response.json()['data']
    except Exception:
        raise ValueError('目录连接失效或权限已变化，请从TeKnowra重新连接') from None

def directory(component, connection=''):
    user = str(getattr(component, 'user_id', '') or '')
    if not user:raise ValueError('需要Langflow研发登录会话')
    if connection:
        try:
            data=json.loads(connection)
            url=data['directory_url'];ticket=data['directory_ticket']
        except Exception:
            raise ValueError('请粘贴完整的TeKnowra目录连接JSON') from None
        catalog=_load(url,ticket)
        with _lock:_connections[(user,catalog['agent_id'])]=(url,ticket,time.monotonic()+900)
        return catalog
    agent=str(getattr(component,'directory_agent_id','') or '')
    with _lock:
        for key,value in list(_connections.items()):
            if value[2]<=time.monotonic():del _connections[key]
        connection=_connections.get((user,agent))
    if not connection:raise ValueError('请先连接TeKnowra目录；连接不会写入组件保存数据')
    return _load(connection[0],connection[1])

def refresh_directory(component, config, field_value, field_name, field, source):
    if field_name == 'directory_connection' and not field_value:return config
    connection=field_value if field_name=='directory_connection' else ''
    try:
        catalog=directory(component,connection)
        return refresh_fields(config,catalog,field,source),catalog
    finally:
        if 'directory_connection' in config:config['directory_connection']['value']=''

def runtime_url(value):
    parsed=urlsplit(value)
    if parsed.scheme!='http' or parsed.hostname!='127.0.0.1' or not parsed.port or parsed.username or parsed.query or parsed.fragment:
        raise ValueError('请从TeKnowra正式任务启动流程')
    return value.rstrip('/')

async def call_host(component, path, payload):
    base=runtime_url(component.bridge_url)
    payload['node_key']=component.node_key
    async with httpx.AsyncClient(timeout=720, follow_redirects=False, trust_env=False) as client:
        response=await client.post(base+path,headers={'Authorization':'Bearer '+component.task_capability},json=payload)
        if response.status_code!=200:raise ValueError('TeKnowra能力调用失败；请检查任务权限、授权或工具配置')
        return json.dumps(response.json(),ensure_ascii=False)
