import http.server,pathlib,json,sys,time,urllib.parse
root=pathlib.Path(sys.argv[1])
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  assert self.headers.get('Authorization')=='Device update-fixture-secret'
  url=urllib.parse.urlparse(self.path)
  assert urllib.parse.parse_qs(url.query)=={'platform':['switch']}
  slow=url.path.startswith('/slow/');large=url.path.startswith('/large/');error=url.path.startswith('/error/')
  if slow:time.sleep(2)
  if url.path.endswith('/api/handheld/'):
   data=(root/'metadata.json').read_bytes();status=200
  else:
   assert url.path.endswith('/api/handheld/download/')
   data=(root/'valid.zip').read_bytes();status=500 if error else 200
  self.send_response(status)
  self.send_header('Content-Length',str(65*1024*1024 if large else len(data)));self.end_headers()
  try:
   if large:
    for _ in range(65):self.wfile.write(b'x'*(1024*1024))
   else:self.wfile.write(data)
  except (BrokenPipeError,ConnectionResetError):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
(root/'http_port').write_text(str(server.server_port))
server.serve_forever()
