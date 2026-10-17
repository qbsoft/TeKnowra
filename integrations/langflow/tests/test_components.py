import unittest
from teknowra_support import resource_id, tool_name, validate_arguments, refresh_fields, schema_model, wire_arguments

class ComponentContracts(unittest.TestCase):
    def test_encoded_tools_survive_text_unescaping(self):
        import base64,json
        from teknowra_support import decode_tool_descriptors
        tools=[{'service_id':'s','name':'lookup','description':'第一行\n引用 "customer_id" 和 \\ 路径','input_schema':{'type':'object'}}]
        wire='base64:'+base64.b64encode(json.dumps(tools,ensure_ascii=False).encode()).decode()
        self.assertEqual(decode_tool_descriptors(wire),tools)

    def test_names_keep_stable_resource_id(self):
        self.assertEqual(resource_id('已有模型 · model-one'), 'model-one')
        self.assertEqual(resource_id('model-one'), 'model-one')

    def test_same_tool_name_is_namespaced_by_service(self):
        self.assertNotEqual(tool_name('service-one', 'lookup'), tool_name('service-two', 'lookup'))

    def test_two_knowledge_components_route_to_distinct_databases(self):
        from teknowra.knowledge import TeKnowraKnowledge
        first=TeKnowraKnowledge(knowledge_base_id='kb-a').build_tools()[0]
        second=TeKnowraKnowledge(knowledge_base_id='kb-b').build_tools()[0]
        routes={tool.name:tool for tool in [first,second]}
        self.assertEqual(len(routes),2)

    def test_original_schema_rejects_numeric_id_and_invalid_enum(self):
        schema={'type':'object','properties':{'customer_id':{'type':'string'},'mode':{'type':'string','enum':['read']},'filters':{'type':'object','properties':{'tags':{'type':'array','items':{'type':'string'}}},'required':['tags']}},'required':['customer_id','mode','filters'],'additionalProperties':False}
        good={'customer_id':'2075179812203560962','mode':'read','filters':{'tags':['a']}}
        validate_arguments(schema,good)
        for key,value in [('customer_id',2075179812203560962),('mode','write'),('filters',{'tags':[1]})]:
            with self.assertRaises(ValueError):validate_arguments(schema,{**good,key:value})

    def test_refresh_clears_ticket_and_preserves_id_after_rename(self):
        config={'directory_connection':{'value':'short-ticket'},'model_id':{'value':'旧名称 · m'}}
        out=refresh_fields(config,{'agent_id':'a','models':[{'id':'m','name':'新名称'}]},'model_id','models')
        self.assertEqual(out['directory_connection']['value'],'')
        self.assertEqual(resource_id(out['model_id']['value']),'m')
        self.assertEqual(out['model_id']['options'],['新名称 · m'])

    def test_wire_arguments_preserve_nested_objects_and_explicit_null(self):
        schema={'type':'object','properties':{'nested':{'type':'object','properties':{'items':{'type':'array','items':{'type':'string'}}},'required':['items']},'nullable':{'type':['string','null']}},'required':['nested','nullable']}
        model=schema_model(schema)
        instance=model.model_validate({'nested':{'items':['a']},'nullable':None})
        args=wire_arguments({'nested':instance.nested,'nullable':None})
        self.assertEqual(args,{'nested':{'items':['a']},'nullable':None})
        validate_arguments(schema,args)

if __name__=='__main__':unittest.main()
