from langchain_core.tools import StructuredTool
from lfx.custom import Component
from lfx.field_typing import Tool
from lfx.io import DropdownInput, BoolInput, Output
from pydantic import BaseModel, Field, ConfigDict
from teknowra_support import design_inputs, refresh_directory, resource_id, call_host, tool_name


class TeKnowraSkills(Component):
    display_name = 'TeKnowra 技能'
    description = '按需读取平台已安装且当前Agent授权的技能与资源；脚本由TeKnowra现有执行环境运行。'
    icon = 'BookOpen'
    name = 'TeKnowraSkills'
    inputs = design_inputs() + [
        DropdownInput(name='skill_name', display_name='平台技能', options=[], value='', refresh_button=True, real_time_refresh=True),
        BoolInput(name='enable_scripts', display_name='提供技能脚本工具', value=False, info='还须发布绑定允许脚本；仅运行该技能scripts目录中的文件，不提供任意shell。'),
    ]
    outputs = [Output(display_name='技能工具', name='tools', method='build_tools')]

    async def update_build_config(self, build_config, field_value, field_name=None):
        if field_name in ('directory_connection', 'skill_name'):
            result = refresh_directory(self, build_config, field_value, field_name, 'skill_name', 'skills')
            return result[0] if isinstance(result, tuple) else result
        return build_config

    def build_tools(self) -> list[Tool]:
        name = resource_id(self.skill_name)
        class Read(BaseModel):
            model_config = ConfigDict(extra='forbid')
            file_path: str = Field(default='SKILL.md', description='默认读取技能正文；附加资源使用技能内相对路径，如REFERENCE.md。')
            offset: int = Field(default=1, ge=1, le=1000000, description='从第几行开始（1起算）；截断时用返回的next_offset续读。')
            limit: int = Field(default=2000, ge=1, le=2000, description='本次最多读取行数。')
            max_bytes: int = Field(default=65536, ge=1, le=65536, description='本次文本字节预算。')
        async def read(file_path: str = 'SKILL.md', offset: int = 1, limit: int = 2000, max_bytes: int = 65536) -> str:
            return await call_host(self, '/skill', {'skill_name': name, 'file_path': file_path, 'offset': offset, 'limit': limit, 'max_bytes': max_bytes})
        result = [StructuredTool.from_function(
            name=tool_name('skill:' + name, 'read_skill'),
            description='按需读取TeKnowra技能 ' + name + ' 的正文和资源。技能正文中提到read_skill/read_file时，使用此工具和file_path；技能不会授予额外知识或MCP权限。',
            coroutine=read, args_schema=Read,
        )]
        if self.enable_scripts:
            class Script(BaseModel):
                model_config = ConfigDict(extra='forbid')
                script_path: str = Field(description='已安装技能scripts目录内的相对入口，支持.py/.js/.sh。')
                args: list[str] = Field(default_factory=list, description='脚本参数，作为字面字符串传递，不接受shell命令。')
                stdin: str = Field(default='', description='传给技能脚本stdin的原始文本或JSON；不得传递平台凭据。')
            async def execute(script_path: str, args: list[str] = None, stdin: str = '') -> str:
                return await call_host(self, '/skill', {'skill_name': name, 'file_path': script_path, 'execute': True, 'args': args or [], 'stdin': stdin})
            result.append(StructuredTool.from_function(
                name=tool_name('skill:' + name, 'execute_skill_script'),
                description='通过TeKnowra现有受控环境执行技能 ' + name + ' 的脚本文件；先读取SKILL.md和入口说明，不能运行任意shell。',
                coroutine=execute, args_schema=Script,
            ))
        return result
