from lfx.custom import Component
from lfx.io import DropdownInput, HandleInput, MessageInput, Output
from lfx.schema import Message


class ExpertResults(Component):
    display_name = 'TeKnowra 已核验专家汇总'
    description = '等待主协调结束，汇总专家真实结果；忽略协调模型编写的业务结论，失败时不发布。'
    icon = 'ListChecks'
    name = 'ExpertResults'
    inputs = [
        MessageInput(name='coordination', display_name='主Agent执行完成', required=True),
        HandleInput(name='expert_tools', display_name='原专家委派工具', input_types=['Tool'], is_list=True, required=True),
        DropdownInput(name='required_experts', display_name='必需完成的专家', options=['contract,credit', 'contract', 'credit'], value='contract,credit'),
    ]
    outputs = [Output(display_name='经核验的专家结果', name='report', method='build_report')]

    def build_report(self) -> Message:
        required = self.required_experts.split(',')
        tools = self.expert_tools if isinstance(self.expert_tools, list) else [self.expert_tools]
        results = {}
        request = None
        for tool in tools:
            run = getattr(tool, '_expert_run', None)
            role = getattr(tool, '_expert_role', None)
            if run is None or role not in ('contract', 'credit') or role in results:
                raise ValueError('必须连接原受控专家，不能替换、重复或伪造结果')
            if request is not None and request != run.request:
                raise ValueError('不同原始任务的专家结果不能混用')
            request = run.request
            results[role] = run.results.get(role)
        if any(not results.get(role) for role in required):
            raise ValueError('必需专家尚未通过全部检查，不发布组合结论')
        labels = {'contract': '合同专家', 'credit': '信用专家'}
        return Message(text='\n\n'.join('## ' + labels[r] + '\n\n' + results[r] for r in required) + '\n\n以上为只读专家结果，未付款、审批或回写。')
