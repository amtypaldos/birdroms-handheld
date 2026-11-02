import pathlib,struct,hashlib,json,zipfile,sys,warnings
root=pathlib.Path(sys.argv[1]);root.mkdir(parents=True,exist_ok=True)
nro=bytearray(256);nro[0x10:0x14]=b'NRO0';struct.pack_into('<I',nro,0x18,len(nro));struct.pack_into('<IIIIII',nro,0x20,0,128,128,64,192,64)
old=bytearray(nro);old[0x90]=1
(root/'app.nro').write_bytes(old);(root/'config.json').write_text('{"device_token":"preserve-fixture-secret"}');(root/'Game.nes').write_bytes(b'ROM unchanged')
manifest=dict(platform='switch',version='2026.11.01.2',nro_sha256=hashlib.sha256(nro).hexdigest(),nro_size=len(nro))
def package(name,items):
 with warnings.catch_warnings():
  warnings.simplefilter('ignore')
  with zipfile.ZipFile(root/name,'w',zipfile.ZIP_STORED) as z:
   for path,data in items:z.writestr(path,data)
base=[('switch/birdroms/birdroms.nro',nro),('switch/birdroms/version.json',json.dumps(manifest))]
package('valid.zip',base+[('switch/birdroms/server.json',json.dumps(dict(server_url='https://example.invalid')))])
for name,items in [
 ('traversal.zip',base+[('../escape',b'bad')]),
 ('duplicate.zip',base+[base[0]]),
 ('config.zip',base+[('switch/birdroms/config.json','{}')]),
 ('seed-secret.zip',base+[('switch/birdroms/server.json','{"server_url":"https://example.invalid","device_token":"bad"}')]),
 ('missing.zip',base[:1]),
 ('manifest-extra.zip',[base[0],('switch/birdroms/version.json',json.dumps({**manifest,'device_token':'forbidden'}))]),
 ('manifest-duplicate.zip',[base[0],('switch/birdroms/version.json',json.dumps(manifest).replace('"platform": "switch"','"platform": "muos", "platform": "switch"'))]),
 ('manifest.zip',[base[0],('switch/birdroms/version.json',json.dumps({**manifest,'version':'2026.11.01.3'}))]),
 ('invalid-nro.zip',[('switch/birdroms/birdroms.nro',b'x'*256),base[1]])]:package(name,items)
fake_dir=zipfile.ZipInfo('switch/');fake_dir.create_system=3;fake_dir.external_attr=(0o100644<<16)
package('fake-dir.zip',base+[(fake_dir,b'')])
link=zipfile.ZipInfo('switch/birdroms/server.json');link.create_system=3;link.external_attr=(0o120777<<16)
package('symlink.zip',base+[(link,'config.json')])
raw=bytearray((root/'valid.zip').read_bytes());at=raw.find(b'PK\x01\x02');struct.pack_into('<I',raw,at+24,64*1024*1024+1);(root/'oversize.zip').write_bytes(raw)
# API sha256 identifies the stored ZIP, not this personalized download.
info=dict(available=True,platform="switch",version=manifest['version'],filename='birdroms-switch.zip',sha256=hashlib.sha256(b'stored-unpersonalized-package').hexdigest(),size=(root/'valid.zip').stat().st_size,nro_sha256=manifest['nro_sha256'],nro_size=manifest['nro_size'])
(root/'metadata.json').write_text(json.dumps(info));(root/'config.original').write_bytes((root/'config.json').read_bytes());(root/'old.nro').write_bytes(old)
