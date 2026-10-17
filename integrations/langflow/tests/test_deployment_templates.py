import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

class DeploymentTemplateTests(unittest.TestCase):
    def test_redis_password_resolved_inside_container_not_from_host(self):
        docker=os.environ.get('TEKNOWRA_DOCKER_CLI') or shutil.which('docker')
        if not docker:self.skipTest('Docker CLI needed for Compose configuration integration')
        source=Path(__file__).resolve().parents[1]/'delivery/templates/infrastructure.json'
        service=json.loads(source.read_text(encoding='utf-8'))['services']['redis']
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);env=root/'redis.env';env.write_text('REDIS_PASSWORD=synthetic-compose-test-secret\n')
            service['env_file']=str(env);service.pop('volumes')
            compose=root/'compose.json';compose.write_text(json.dumps({'services':{'redis':service}}))
            host={k:v for k,v in os.environ.items() if k not in ('REDIS_PASSWORD','COMPOSE_FILE','COMPOSE_PROJECT_NAME')}
            r=subprocess.run([docker,'compose','-p','template-config-test','-f',str(compose),'config','--format','json'],capture_output=True,text=True,env=host)
            self.assertEqual(r.returncode,0,'Compose must validate without host REDIS_PASSWORD')
            v=json.loads(r.stdout)['services']['redis']
            self.assertEqual(v['environment']['REDIS_PASSWORD'],'synthetic-compose-test-secret')
            self.assertIn('REDIS_PASSWORD',' '.join(v['command']),'Redis startup must retain container-time password expansion')

if __name__=='__main__':unittest.main()
