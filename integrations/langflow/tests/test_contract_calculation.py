"""Executable contract for the trusted stdin-only arithmetic skill."""
import json
from pathlib import Path
import subprocess
import sys
import unittest

SCRIPT = Path(__file__).resolve().parents[3] / 'skills/preloaded/tyer-contract-session-review/scripts/calculate.py'

class ContractCalculationTests(unittest.TestCase):
    def run_script(self, data):
        return subprocess.run([sys.executable, str(SCRIPT)], input=json.dumps(data), text=True, capture_output=True, encoding='utf-8')

    def payload(self):
        return dict(amount='123450.00', prepayment_percent=40, review_threshold_percent=35,
                    rule_reference='合成采购制度 TK-DEMO-20261016-V1 第3条')

    def test_rule_is_input_not_hardcoded_and_rounding_is_decimal(self):
        data=self.payload()
        result=self.run_script(data)
        self.assertEqual(result.returncode,0,result.stderr)
        value=json.loads(result.stdout)
        self.assertEqual(value['prepayment_amount'],'49380.00')
        self.assertTrue(value['threshold_exceeded'])
        self.assertEqual(value['rule_reference'],data['rule_reference'])
        data['review_threshold_percent']=45
        self.assertFalse(json.loads(self.run_script(data).stdout)['threshold_exceeded'])
        data['amount']='0.05';data['prepayment_percent']=10
        self.assertEqual(json.loads(self.run_script(data).stdout)['prepayment_amount'],'0.01')

    def test_rejects_path_fields_and_invalid_inputs(self):
        for field,value in [('path','/workspace/input/other.pdf'),('amount','NaN'),('amount','../old.txt'),('prepayment_percent',101),('review_threshold_percent',-1),('rule_reference','')]:
            data=self.payload();data[field]=value
            self.assertNotEqual(self.run_script(data).returncode,0,field)

if __name__=='__main__':unittest.main()
