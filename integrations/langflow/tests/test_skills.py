import asyncio,unittest
from unittest.mock import patch
from teknowra.skills import TeKnowraSkills

class SkillComponentContracts(unittest.TestCase):
    def test_bound_skill_name_and_entry_are_not_model_controlled(self):
        component=TeKnowraSkills(skill_name='技能 · tyer-demo',enable_scripts=True)
        calls=[]
        async def host(c,path,payload):calls.append((path,payload));return 'ok'
        with patch('teknowra.skills.call_host',host):
            tools=component.build_tools()
            self.assertEqual(len(tools),2)
            asyncio.run(tools[0].ainvoke({'file_path':'REFERENCE.md'}))
            asyncio.run(tools[1].ainvoke({'script_path':'scripts/report.py','args':['two words'],'stdin':'{"amount":100000}'}))
        self.assertEqual(calls[0][1]['skill_name'],'tyer-demo')
        self.assertEqual(calls[1][1]['skill_name'],'tyer-demo')
        self.assertEqual(calls[1][1]['file_path'],'scripts/report.py')
        self.assertTrue(calls[1][1]['execute'])
        self.assertNotIn('command',calls[1][1])
    def test_read_only_skill_never_exposes_script_tool(self):
        component=TeKnowraSkills(skill_name='tyer-demo',enable_scripts=False)
        self.assertEqual(len(component.build_tools()),1)
    def test_skill_resource_can_continue_after_truncation(self):
        component=TeKnowraSkills(skill_name='tyer-demo',enable_scripts=False)
        calls=[]
        async def host(c,path,payload):calls.append(payload);return '{"data":{"next_offset":2501}}'
        with patch('teknowra.skills.call_host',host):
            asyncio.run(component.build_tools()[0].ainvoke({'file_path':'REFERENCE.md','offset':2001,'limit':500,'max_bytes':4096}))
        self.assertEqual(calls[0]['offset'],2001)
        self.assertEqual(calls[0]['limit'],500)
        self.assertEqual(calls[0]['max_bytes'],4096)
