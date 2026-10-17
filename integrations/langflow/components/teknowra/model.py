from langchain_openai import ChatOpenAI
from lfx.custom import Component
from lfx.field_typing import LanguageModel
from lfx.io import DropdownInput, FloatInput, IntInput, Output
from teknowra_support import design_inputs, refresh_directory, resource_id, runtime_url

class TeKnowraModel(Component):
    display_name='TeKnowra 模型'
    description='动态选择平台模型；运行时由TeKnowra验证当前员工权限，不复制供应商密钥。'
    icon='Brain'
    name='TeKnowraModel'
    inputs=design_inputs()+[
        DropdownInput(name='model_id',display_name='平台模型',options=[],value='',refresh_button=True,real_time_refresh=True),
        FloatInput(name='temperature',display_name='温度',value=0.15),
        IntInput(name='max_tokens',display_name='输出Token上限',value=4096),
    ]
    outputs=[Output(display_name='平台模型',name='model_output',method='build_model')]

    async def update_build_config(self,build_config,field_value,field_name=None):
        if field_name in ('directory_connection','model_id'):
            result=refresh_directory(self,build_config,field_value,field_name,'model_id','models')
            return result[0] if isinstance(result,tuple) else result
        return build_config

    def build_model(self)->LanguageModel:
        return ChatOpenAI(model=resource_id(self.model_id),api_key=self.task_capability,base_url=runtime_url(self.bridge_url)+'/v1',temperature=self.temperature,max_tokens=self.max_tokens,timeout=90,max_retries=0,streaming=False)
