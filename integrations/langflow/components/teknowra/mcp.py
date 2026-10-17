import json
from langchain_core.tools import StructuredTool
from lfx.custom import Component
from lfx.field_typing import Tool
from lfx.io import DropdownInput,Output,StrInput
from teknowra_support import design_inputs,refresh_directory,directory,resource_id,tool_name,schema_model,validate_arguments,wire_arguments,call_host,decode_tool_descriptors

class TeKnowraMCP(Component):
    display_name='TeKnowra MCP 工具'
    description='从已发布目录选择服务和工具，参数schema自动生成；本人OAuth与审批由TeKnowra处理。'
    icon='Wrench'
    name='TeKnowraMCP'
    inputs=design_inputs()+[
        DropdownInput(name='service_id',display_name='MCP服务',options=[],value='',refresh_button=True,real_time_refresh=True),
        DropdownInput(name='tool',display_name='工具',options=[],value='',refresh_button=True,real_time_refresh=True,info='留空表示该服务全部已绑定工具；可在Actions选择专家要使用的工具'),
        StrInput(name='tools_json',display_name='已发布工具schema',value='',advanced=True),
    ]
    outputs=[Output(display_name='受控工具',name='tools',method='build_tools')]
    async def update_build_config(self,build_config,field_value,field_name=None):
        if field_name in ('directory_connection','service_id','tool'):
            result=refresh_directory(self,build_config,field_value,field_name,'service_id','services')
            if not isinstance(result,tuple):return result
            build_config,catalog=result
            service=resource_id(build_config['service_id']['value'])
            if field_name=='service_id':service=resource_id(field_value)
            build_config['tool']['options']=[t['name'] for s in catalog['services'] if s['id']==service for t in s['tools']]
            if build_config['tool'].get('value') not in build_config['tool']['options']:build_config['tool']['value']=''
            return build_config
        return build_config
    def build_tools(self)->list[Tool]:
        service=resource_id(self.service_id)
        if self.tools_json:
            descriptors=decode_tool_descriptors(self.tools_json)
        else:
            catalog=directory(self)
            descriptors=[{'service_id':s['id'],**t} for s in catalog['services'] for t in s['tools']]
        selected=getattr(self,'tool','') or ''
        descriptors=[d for d in descriptors if d['service_id']==service and (not selected or d['name']==selected)]
        if not descriptors:raise ValueError('未选择已发布且可用的工具，请刷新目录或检查绑定')
        tools=[]
        for descriptor in descriptors:
            schema=descriptor['input_schema'];model=schema_model(schema)
            def make_call(d):
                async def call(**kwargs):
                    args=wire_arguments(kwargs)
                    validate_arguments(d['input_schema'],args)
                    return await call_host(self,'/tool',{'service_id':d['service_id'],'tool':d['name'],'args':args})
                return call
            tools.append(StructuredTool.from_function(name=tool_name(service,descriptor['name']),description=descriptor.get('description','')+'（平台工具：'+descriptor['name']+'）',coroutine=make_call(descriptor),args_schema=model))
        return tools
