import unittest

import teknowra_reviewed_experts_v2 as previous


class CanonicalDraftPolicyTests(unittest.IsolatedAsyncioTestCase):
    async def prompts(self, module):
        prompts = []
        async def model(prompt):
            prompts.append(prompt)
            return ({'summary': '人民币100000元，预付40%超过30%。', 'citations': ['s1']}
                    if len(prompts) == 1 else {'approved': True, 'issues': []})
        async def knowledge(_):
            return {'results': [{'source_id': 's1', 'content': '人民币100000元，预付40%；超过30%需提示风险。'}]}
        async def mcp(*_):
            self.fail('contract review must not call CRM')
        run = module.ExpertRun('核对合同与30%复核阈值', model, knowledge, mcp)
        await run.execute('contract')
        return prompts

    async def test_generator_uses_source_terms_without_changing_independent_audit(self):
        try:
            import teknowra_reviewed_experts_v3 as current
        except ImportError:
            current = previous
        old = await self.prompts(previous)
        new = await self.prompts(current)
        self.assertIn('直接使用资料中的政策原文术语', new[0])
        self.assertIn('不要额外讨论用户简称与政策术语是否等价', new[0])
        self.assertEqual(old[1], new[1], 'independent audit and acceptance rules must remain unchanged')


if __name__ == '__main__':
    unittest.main()
