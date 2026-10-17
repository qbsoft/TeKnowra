from langchain_core.tools import StructuredTool
from lfx.custom import Component
from lfx.field_typing import Tool
from lfx.io import DropdownInput, Output
from pydantic import BaseModel,Field
from teknowra_support import design_inputs,refresh_directory,resource_id,call_host,tool_name

class TeKnowraKnowledge(Component):
    display_name='TeKnowra 知识检索'
    description='动态选择已绑定知识库；目录可见不代表最终员工获权，检索与引用仍由宿主检查。'
    icon='BookOpen'
    name='TeKnowraKnowledge'
    inputs=design_inputs()+[DropdownInput(name='knowledge_base_id',display_name='知识库',options=[],value='',refresh_button=True,real_time_refresh=True)]
    outputs=[Output(display_name='知识工具',name='tools',method='build_tools')]
    async def update_build_config(self,build_config,field_value,field_name=None):
        if field_name in ('directory_connection','knowledge_base_id'):
            result=refresh_directory(self,build_config,field_value,field_name,'knowledge_base_id','knowledge_bases')
            return result[0] if isinstance(result,tuple) else result
        return build_config
    def build_tools(self)->list[Tool]:
        class Query(BaseModel):query:str=Field(description='要检索的业务问题；回答引用返回的source_id')
        async def search(query:str)->str:
            return await call_host(self,'/knowledge',{'query':query,'knowledge_base_id':resource_id(self.knowledge_base_id)})
        kb_id=resource_id(self.knowledge_base_id)
        return [StructuredTool.from_function(name=tool_name('knowledge:'+kb_id,'knowledge_search'),description='检索当前员工有权访问的知识库 '+str(self.knowledge_base_id)+'，引用返回的source_id。',coroutine=search,args_schema=Query)]
