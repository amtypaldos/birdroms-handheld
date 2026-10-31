"""Local deterministic API fixture, with real HTTP requests and header checks."""
import http.server,json,urllib.parse,time,sys
ROM=dict(id=1,title='Game',platform='NES',extension='.nes',filename='Game.nes',size_bytes=65536,cover_url=None,tags=[])
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args): pass
 def send(self,status,data):
  body=data if isinstance(data,bytes) else json.dumps(data).encode()
  self.send_response(status);self.send_header('Content-Length',str(len(body)));self.end_headers()
  try:self.wfile.write(body)
  except (BrokenPipeError,ConnectionResetError):pass
 def do_POST(self):
  assert self.path=='/api/devices/pair/start'
  assert self.headers.get('Authorization') is None
  self.send(201,dict(code='ABC123',qr_url='https://example.invalid/pair?code=ABC123',expires_at='2026-10-31T00:00:00Z'))
 def do_GET(self):
  u=urllib.parse.urlparse(self.path);q=urllib.parse.parse_qs(u.query)
  if u.path.startswith('/api/devices/pair/poll'):
   assert self.headers.get('Authorization') is None
   code=q['code'][0];status={'pending':202,'expired':410,'limited':429,'approved':200}.get(code,404)
   self.send(status,{'device_token':'approved-secret'});return
  if u.path=='/external':
   assert self.headers.get('Authorization') is None
   self.send(200,b'img');return
  assert self.headers.get('Authorization')=='Device fixture-secret'
  if u.path=='/api/roms/platforms/':self.send(200,['NES'])
  elif u.path=='/api/playlists/':self.send(200,[dict(id=7,name='Favorites',rom_count=1)])
  elif u.path=='/api/playlists/7/':self.send(200,dict(roms=[ROM]))
  elif u.path=='/api/roms/':
   assert q['platform']==['NES & SNES'] and q['page']==['1'] and q['fields']==['device']
   self.send(200,dict(count=1,next=None,results=[ROM]))
  elif u.path=='/api/roms/1/download/':self.send(200,b'x'*65536)
  elif u.path=='/api/roms/2/download/':self.send(500,b'error must not reach part file')
  elif u.path=='/api/roms/3/download/':
   self.send_response(200);self.send_header('Content-Length','65536');self.end_headers()
   try:
    for _ in range(64):self.wfile.write(b'x'*1024);self.wfile.flush();time.sleep(.05)
   except (BrokenPipeError,ConnectionResetError):pass
  elif u.path=='/redirect':
   self.send_response(302);self.send_header('Location','http://localhost:'+str(self.server.server_port)+'/external');self.send_header('Content-Length','0');self.end_headers()
  elif u.path=='/oversize':self.send(200,b'x'*(4*1024*1024+1))
  else:self.send(404,{})
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
open(sys.argv[1],'w').write(str(server.server_port))
server.serve_forever()
