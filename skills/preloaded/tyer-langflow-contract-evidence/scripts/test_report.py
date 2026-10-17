import subprocess
import sys
import unittest
from pathlib import Path
import json

class ReportTest(unittest.TestCase):
    def invoke(self, value):
        return subprocess.run([sys.executable, str(Path(__file__).with_name('report.py'))], input=json.dumps(value), text=True, capture_output=True)
    def test_amount_and_boundary(self):
        result = self.invoke({'amount':'100000', 'prepayment_percent':'40'})
        self.assertEqual(result.returncode, 0, result.stderr)
        data = json.loads(result.stdout)
        self.assertEqual(data['prepayment_amount'], '40000.00')
        self.assertEqual(data['status'], '需要复核')
        self.assertEqual(json.loads(self.invoke({'amount':'0.10', 'prepayment_percent':'30'}).stdout)['status'], '未触发本条规则')
    def test_invalid_inputs(self):
        for amount, percent in [('NaN','40'), ('-1','40'), ('100','101'), ('100','-1')]:
            self.assertNotEqual(self.invoke({'amount':amount, 'prepayment_percent':percent}).returncode, 0)

if __name__ == '__main__':
    unittest.main()
